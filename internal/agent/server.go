// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sharedco/cilo/internal/agent/config"
)

type Server struct {
	router     chi.Router
	config     *config.Config
	http       *http.Server
	envManager *EnvironmentManager
	wgManager  *WireGuardManager
}

func NewServer(cfg *config.Config) (*Server, error) {
	wgMgr, err := NewWireGuardManager(cfg)
	if err != nil && cfg.WGPrivateKey != "" {
		return nil, err
	}

	s := &Server{
		router:     chi.NewRouter(),
		config:     cfg,
		envManager: NewEnvironmentManager(cfg.WorkspaceDir),
		wgManager:  wgMgr,
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

// setupMiddleware configures the middleware stack for the router.
func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(120 * time.Second))
}

// setupRoutes configures all HTTP routes for the agent.
func (s *Server) setupRoutes() {
	// Health check
	s.router.Get("/health", s.handleHealth)

	// Environment management
	s.router.Route("/environment", func(r chi.Router) {
		r.Post("/up", s.handleUp)
		r.Post("/down", s.handleDown)
		r.Get("/status", s.handleStatus)
		r.Get("/logs/{service}", s.handleLogs)
	})

	// WireGuard peer management
	s.router.Route("/wireguard", func(r chi.Router) {
		r.Post("/add-peer", s.handleAddPeer)
		r.Delete("/remove-peer/{key}", s.handleRemovePeer)
		r.Get("/status", s.handleWGStatus)
	})
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Router returns the underlying router (useful for testing)
func (s *Server) Router() chi.Router {
	return s.router
}
