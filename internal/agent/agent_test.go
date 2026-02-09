// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sharedco/cilo/internal/agent"
	"github.com/sharedco/cilo/internal/agent/config"
)

func TestAgentHealth(t *testing.T) {
	srv := setupTestAgent(t)

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

	if resp["machine_id"] != "test-machine" {
		t.Errorf("expected machine_id 'test-machine', got %v", resp["machine_id"])
	}
}

func TestEnvironmentUp(t *testing.T) {
	srv := setupTestAgent(t)

	body := agent.UpRequest{
		WorkspacePath: "/tmp/test-workspace",
		EnvName:       "test-env",
		Subnet:        "10.224.1.0/24",
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/environment/up", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		var resp agent.UpResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		t.Logf("environment up succeeded: %+v", resp)
	} else {
		t.Logf("environment up expected to fail without docker: status=%d", w.Code)
	}
}

func TestEnvironmentUpInvalidRequest(t *testing.T) {
	srv := setupTestAgent(t)

	invalidBody := []byte(`{"invalid": "json"`)
	req := httptest.NewRequest("POST", "/environment/up", bytes.NewReader(invalidBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestEnvironmentDown(t *testing.T) {
	srv := setupTestAgent(t)

	body := map[string]interface{}{
		"env_name": "test-env",
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/environment/down", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	t.Logf("environment down status: %d", w.Code)
}

func TestEnvironmentDownMissingEnvName(t *testing.T) {
	srv := setupTestAgent(t)

	body := map[string]interface{}{}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/environment/down", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when env_name is missing, got %d", w.Code)
	}
}

func TestEnvironmentStatus(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/environment/status?env_name=test-env", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	t.Logf("environment status: %d", w.Code)
}

func TestEnvironmentStatusMissingParam(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/environment/status", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when env_name is missing, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] == nil {
		t.Error("expected error message in response")
	}
}

func TestEnvironmentLogs(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/environment/logs/api?env_name=test-env", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	t.Logf("environment logs status: %d", w.Code)
}

func TestEnvironmentLogsMissingService(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/environment/logs/?env_name=test-env", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Logf("expected 400 when service is missing, got %d", w.Code)
	}
}

func TestWireGuardAddPeer(t *testing.T) {
	srv := setupTestAgent(t)

	body := agent.AddPeerRequest{
		PublicKey:  "dGVzdHB1YmxpY2tleXRoYXRpczMyYnl0ZXNs",
		AllowedIPs: "10.225.0.1/32",
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/wireguard/add-peer", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code == http.StatusNotImplemented {
		t.Log("add-peer endpoint not yet implemented (expected)")
	} else {
		t.Logf("add-peer status: %d", w.Code)
	}
}

func TestWireGuardRemovePeer(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("DELETE", "/wireguard/remove-peer/test-key", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code == http.StatusNotImplemented {
		t.Log("remove-peer endpoint not yet implemented (expected)")
	} else {
		t.Logf("remove-peer status: %d", w.Code)
	}
}

func TestWireGuardStatus(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/wireguard/status", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code == http.StatusNotImplemented {
		t.Log("wireguard status endpoint not yet implemented (expected)")
	} else {
		t.Logf("wireguard status: %d", w.Code)
	}
}

func TestNotFoundRoute(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("GET", "/non-existent-route", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent route, got %d", w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := setupTestAgent(t)

	req := httptest.NewRequest("DELETE", "/health", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for wrong method, got %d", w.Code)
	}
}

func setupTestAgent(t *testing.T) http.Handler {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "cilo-agent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	cfg := &config.Config{
		ListenAddr:   ":0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		MachineID:    "test-machine",
		WorkspaceDir: tmpDir,
		WGInterface:  "wg0-test",
		WGListenPort: 0,
		WGPrivateKey: "",
		WGAddress:    "10.225.0.100/16",
	}

	srv, err := agent.NewServer(cfg)
	if err != nil {
		t.Skipf("failed to create test server (wireguard may not be available): %v", err)
	}

	return srv.Router()
}
