// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sharedco/cilo/internal/agent/config"
)

// Server implements the cilod API
type Server struct {
	router      chi.Router
	config      *config.Config
	http        *http.Server
	envManager  *EnvironmentManager
	wgManager   *WireGuardManager
	proxy       *EnvProxy
	authHandler *AuthHandler
	peerStore   *JSONPeerStore
}

// NewServer creates a new agent server with all dependencies initialized
func NewServer(cfg *config.Config) (*Server, error) {
	wgMgr, err := NewWireGuardManager(cfg)
	if err != nil {
		if cfg.WGPrivateKey == "" {
			return nil, fmt.Errorf("WireGuard private key not configured (set CILO_WG_PRIVATE_KEY)")
		}
		return nil, fmt.Errorf("failed to initialize WireGuard: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wgMgr.EnsureInterface(ctx); err != nil {
		return nil, fmt.Errorf("failed to create WireGuard interface: %w", err)
	}

	var proxy *EnvProxy
	wgIP := strings.Split(cfg.WGAddress, "/")[0]
	proxyAddr := wgIP + ":80"
	proxy, err = NewEnvProxy(proxyAddr)
	if err != nil {
		log.Printf("Warning: failed to start reverse proxy on %s: %v", proxyAddr, err)
		proxy = nil
	}

	peerStore, err := NewJSONPeerStore("/var/cilo/peers.json")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize peer store: %w", err)
	}

	verifier := NewDefaultSSHVerifier()
	authHandler := NewAuthHandler(verifier, "/var/cilo/peers.json")

	s := &Server{
		router:      chi.NewRouter(),
		config:      cfg,
		envManager:  NewEnvironmentManager(cfg.WorkspaceDir, proxy),
		wgManager:   wgMgr,
		proxy:       proxy,
		authHandler: authHandler,
		peerStore:   peerStore,
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.http = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      s.router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	return s, nil
}

// setupMiddleware configures the middleware stack for the router
func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(120 * time.Second))
}

// setupRoutes configures all HTTP routes for the agent
func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	s.router.Post("/auth/connect", s.HandleAuthConnect)
	s.router.With(s.authHandler.AuthMiddleware).Delete("/auth/disconnect", s.HandleAuthDisconnect)

	s.router.Group(func(r chi.Router) {
		r.Use(s.authHandler.AuthMiddleware)

		r.Get("/environments", s.HandleListEnvironments)
		r.Route("/environments/{name}", func(env chi.Router) {
			env.Post("/up", s.HandleEnvironmentUp)
			env.Post("/down", s.HandleEnvironmentDown)
			env.Delete("/", s.HandleEnvironmentDestroy)
			env.Get("/status", s.HandleEnvironmentStatus)
			env.Get("/logs", s.HandleEnvironmentLogs)
			env.Post("/exec", s.HandleEnvironmentExec)
		})

		r.Route("/wireguard", func(wg chi.Router) {
			wg.Post("/exchange", s.HandleWireGuardExchange)
			wg.Delete("/peers/{key}", s.HandleWireGuardRemovePeer)
			wg.Get("/status", s.HandleWireGuardStatus)
		})

		r.Post("/sync/{name}", s.HandleWorkspaceSync)
	})

	s.router.Route("/environment", func(r chi.Router) {
		r.Post("/up", s.handleUp)
		r.Post("/down", s.handleDown)
		r.Get("/status", s.handleStatus)
		r.Get("/logs/{service}", s.handleLogs)
	})

	s.router.Route("/wireguard", func(r chi.Router) {
		r.Post("/add-peer", s.handleAddPeer)
		r.Delete("/remove-peer/{key}", s.handleRemovePeer)
		r.Get("/status", s.handleWGStatus)
	})
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.proxy != nil {
		s.proxy.Close()
	}
	return s.http.Shutdown(ctx)
}

// Router returns the underlying router (useful for testing)
func (s *Server) Router() chi.Router {
	return s.router
}
