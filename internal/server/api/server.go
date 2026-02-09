// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sharedco/cilo/internal/server/api/handlers"
	"github.com/sharedco/cilo/internal/server/auth"
	"github.com/sharedco/cilo/internal/server/config"
	"github.com/sharedco/cilo/internal/server/store"
	"github.com/sharedco/cilo/internal/server/wireguard"
)

// Server represents the HTTP API server
type Server struct {
	router      *chi.Mux
	store       *store.Store
	authStore   *auth.Store
	authHandler *handlers.AuthHandler
	wgHandler   *handlers.WireGuardHandler
	config      *config.Config
	httpServer  *http.Server
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config, st *store.Store) (*Server, error) {
	authStore := auth.NewStore(st.Pool())
	authHandler := handlers.NewAuthHandler(authStore)

	// Initialize WireGuard components
	wgStore := wireguard.NewStore(st.Pool())
	wgExchange := wireguard.NewExchange(wgStore)
	wgHandler := handlers.NewWireGuardHandler(wgExchange, st)

	s := &Server{
		router:      chi.NewRouter(),
		store:       st,
		authStore:   authStore,
		authHandler: authHandler,
		wgHandler:   wgHandler,
		config:      cfg,
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      s.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return s, nil
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Public routes
	s.router.Get("/health", s.handleHealth)
	s.router.Get("/status", s.handleStatus)

	// Metrics endpoint (if enabled)
	if s.config.Features.MetricsEnabled {
		s.router.Handle("/metrics", promhttp.Handler())
	}

	// API v1 routes
	s.router.Route("/v1", func(r chi.Router) {
		// Auth validation (protected, but returns user info)
		r.With(s.authMiddleware).Get("/auth/validate", s.handleValidateAuth)

		// Auth routes (protected)
		r.Route("/auth", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/keys", s.handleCreateAPIKey)
			r.Get("/keys", s.handleListAPIKeys)
			r.Delete("/keys/{keyID}", s.handleRevokeAPIKey)
		})

		// Environment routes (protected)
		r.Route("/environments", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/", s.handleCreateEnvironment)
			r.Get("/", s.handleListEnvironments)
			r.Get("/{envID}", s.handleGetEnvironment)
			r.Delete("/{envID}", s.handleDestroyEnvironment)
			r.Post("/{envID}/sync", s.handleSyncEnvironment)
		})

		// WireGuard routes (protected)
		r.Route("/wireguard", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/exchange", s.handleWireGuardExchange)
			r.Delete("/peers/{key}", s.handleRemovePeer)
			r.Get("/status/{environment_id}", s.handleWireGuardStatus)
		})

		// Machine routes (protected)
		r.Route("/machines", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/", s.handleRegisterMachine)
			r.Get("/", s.handleListMachines)
			r.Delete("/{machineID}", s.handleRemoveMachine)
		})
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Router returns the underlying router (useful for testing)
func (s *Server) Router() *chi.Mux {
	return s.router
}
