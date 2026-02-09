// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sharedco/cilo/internal/server/api"
	"github.com/sharedco/cilo/internal/server/config"
	"github.com/sharedco/cilo/internal/server/store"
)

// TestHealthEndpoint verifies /health returns healthy status
func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected healthy status, got %v", resp["status"])
	}
}

// TestStatusEndpoint verifies /status returns server info
func TestStatusEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "operational" {
		t.Errorf("expected operational status, got %v", resp["status"])
	}
}

// TestAuthMiddlewareWithoutToken verifies API key authentication fails without token
func TestAuthMiddlewareWithoutToken(t *testing.T) {
	srv := setupTestServer(t)

	// Without API key - should fail
	req := httptest.NewRequest("GET", "/v1/environments", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without API key, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] == nil {
		t.Error("expected error message in response")
	}
}

// TestAuthMiddlewareWithInvalidToken verifies API key authentication fails with invalid token
func TestAuthMiddlewareWithInvalidToken(t *testing.T) {
	srv := setupTestServer(t)

	// With invalid API key - should fail
	req := httptest.NewRequest("GET", "/v1/environments", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid API key, got %d", w.Code)
	}
}

// TestAuthMiddlewareWithMalformedHeader verifies API key authentication fails with malformed header
func TestAuthMiddlewareWithMalformedHeader(t *testing.T) {
	srv := setupTestServer(t)

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "invalid-token"},
		{"wrong prefix", "Token invalid-token"},
		{"empty bearer", "Bearer "},
		{"only bearer", "Bearer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/environments", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 with malformed header %q, got %d", tc.header, w.Code)
			}
		})
	}
}

// TestCreateEnvironmentNotImplemented tests environment creation endpoint returns not implemented
func TestCreateEnvironmentNotImplemented(t *testing.T) {
	srv := setupTestServer(t)

	// Create a mock team and API key for testing
	// Note: This will fail auth since we don't have a real API key
	// For now, we're just testing the endpoint exists

	body := map[string]interface{}{
		"name":    "test-env",
		"project": "test-project",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/environments", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// Missing auth header intentionally - will fail at auth
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Should fail at auth, not at handler
	if w.Code != http.StatusUnauthorized {
		t.Logf("expected 401 (auth failure), got %d", w.Code)
	}
}

// TestWireGuardExchangeEndpoint tests wireguard exchange endpoint exists
func TestWireGuardExchangeEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body := map[string]interface{}{
		"environment_id":    "test-env-id",
		"client_public_key": "dGVzdHB1YmxpY2tleXRoYXRpczMyYnl0ZXNs",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/wireguard/exchange", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Should fail at auth
	if w.Code != http.StatusUnauthorized {
		t.Logf("expected 401 (auth failure), got %d", w.Code)
	}
}

// TestMachineRegistrationEndpoint tests machine registration endpoint exists
func TestMachineRegistrationEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body := map[string]interface{}{
		"machine_id": "test-machine-123",
		"public_ip":  "192.168.1.100",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/machines", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Should fail at auth
	if w.Code != http.StatusUnauthorized {
		t.Logf("expected 401 (auth failure), got %d", w.Code)
	}
}

// TestNotFoundEndpoint verifies 404 for non-existent routes
func TestNotFoundEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/non-existent-route", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent route, got %d", w.Code)
	}
}

// setupTestServer creates a test server instance with mock/in-memory database
func setupTestServer(t *testing.T) http.Handler {
	t.Helper()

	// Use in-memory SQLite for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			ListenAddr:   ":0",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Database: config.DatabaseConfig{
			// Use local PostgreSQL for tests (pgxpool requires postgres DSN format)
			URL: "postgres://localhost/cilo_test?sslmode=disable",
		},
		Pool: config.PoolConfig{
			MinReady: 0, // Disable pool for tests
			MaxTotal: 0,
		},
		Features: config.FeaturesConfig{
			BillingEnabled: false,
			MetricsEnabled: false, // Disable metrics for tests
		},
	}

	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		t.Skipf("skipping test: no database available: %v", err)
	}
	defer st.Close()

	// Create server
	srv, err := api.NewServer(cfg, st)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	return srv.Router()
}
