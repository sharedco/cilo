// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cloud_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sharedco/cilo/internal/cloud"
)

func TestClient_CreateEnvironment(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/environments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Check auth header
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return success response
		resp := cloud.CreateEnvironmentResponse{
			Environment: &cloud.Environment{
				ID:      "env-123",
				Name:    "test-env",
				Project: "test-project",
				Status:  "running",
			},
			Machine: &cloud.Machine{
				ID:       "machine-456",
				PublicIP: "203.0.113.1",
				Status:   "running",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := cloud.NewClient(server.URL, "test-api-key")

	// Test create environment
	ctx := context.Background()
	resp, err := client.CreateEnvironment(ctx, cloud.CreateEnvironmentRequest{
		Name:    "test-env",
		Project: "test-project",
		Format:  "compose",
	})

	if err != nil {
		t.Fatalf("CreateEnvironment failed: %v", err)
	}

	if resp.Environment.ID != "env-123" {
		t.Errorf("expected ID 'env-123', got '%s'", resp.Environment.ID)
	}

	if resp.Environment.Name != "test-env" {
		t.Errorf("expected name 'test-env', got '%s'", resp.Environment.Name)
	}

	if resp.Machine.PublicIP != "203.0.113.1" {
		t.Errorf("expected PublicIP '203.0.113.1', got '%s'", resp.Machine.PublicIP)
	}
}

func TestClient_CreateEnvironment_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "")

	ctx := context.Background()
	_, err := client.CreateEnvironment(ctx, cloud.CreateEnvironmentRequest{
		Name:    "test-env",
		Project: "test-project",
	})

	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestClient_GetEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/environments/env-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := cloud.Environment{
			ID:      "env-123",
			Name:    "test-env",
			Project: "test-project",
			Status:  "running",
			Subnet:  "10.224.1.0/24",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	env, err := client.GetEnvironment(ctx, "env-123")
	if err != nil {
		t.Fatalf("GetEnvironment failed: %v", err)
	}

	if env.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", env.Status)
	}

	if env.Subnet != "10.224.1.0/24" {
		t.Errorf("expected subnet '10.224.1.0/24', got '%s'", env.Subnet)
	}
}

func TestClient_GetEnvironmentByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Environments []*cloud.Environment `json:"environments"`
		}{
			Environments: []*cloud.Environment{
				{ID: "env-123", Name: "test-env-1", Status: "running"},
				{ID: "env-456", Name: "test-env-2", Status: "running"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	env, err := client.GetEnvironmentByName(ctx, "test-env-2")
	if err != nil {
		t.Fatalf("GetEnvironmentByName failed: %v", err)
	}

	if env.ID != "env-456" {
		t.Errorf("expected ID 'env-456', got '%s'", env.ID)
	}
}

func TestClient_GetEnvironmentByName_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Environments []*cloud.Environment `json:"environments"`
		}{
			Environments: []*cloud.Environment{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	_, err := client.GetEnvironmentByName(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent environment")
	}
}

func TestClient_ListEnvironments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/environments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := struct {
			Environments []*cloud.Environment `json:"environments"`
		}{
			Environments: []*cloud.Environment{
				{ID: "env-123", Name: "env-1", Status: "running"},
				{ID: "env-456", Name: "env-2", Status: "stopped"},
				{ID: "env-789", Name: "env-3", Status: "running"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	envs, err := client.ListEnvironments(ctx)
	if err != nil {
		t.Fatalf("ListEnvironments failed: %v", err)
	}

	if len(envs) != 3 {
		t.Errorf("expected 3 environments, got %d", len(envs))
	}

	if envs[1].Status != "stopped" {
		t.Errorf("expected second env status 'stopped', got '%s'", envs[1].Status)
	}
}

func TestClient_DestroyEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/environments/env-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		// Check Authorization header
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	err := client.DestroyEnvironment(ctx, "env-123")
	if err != nil {
		t.Fatalf("DestroyEnvironment failed: %v", err)
	}
}

func TestClient_SyncComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/environments/env-123/sync" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	err := client.SyncComplete(ctx, "env-123")
	if err != nil {
		t.Fatalf("SyncComplete failed: %v", err)
	}
}

func TestClient_WireGuardExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/wireguard/exchange" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Decode request
		var req cloud.WireGuardExchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.PublicKey == "" {
			t.Error("expected client public key in request")
		}

		resp := cloud.WireGuardExchangeResponse{
			AssignedIP:     "10.224.1.2",
			ServerPubKey:   "server-pubkey-123",
			ServerEndpoint: "203.0.113.1:51820",
			AllowedIPs:     "10.224.1.0/24",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	resp, err := client.WireGuardExchange(ctx, cloud.WireGuardExchangeRequest{
		EnvironmentID: "env-123",
		PublicKey:     "client-pubkey-456",
	})

	if err != nil {
		t.Fatalf("WireGuardExchange failed: %v", err)
	}

	if resp.AssignedIP != "10.224.1.2" {
		t.Errorf("expected AssignedIP '10.224.1.2', got '%s'", resp.AssignedIP)
	}

	if resp.ServerPubKey != "server-pubkey-123" {
		t.Errorf("expected ServerPubKey 'server-pubkey-123', got '%s'", resp.ServerPubKey)
	}
}

func TestClient_RemovePeer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/wireguard/peers/client-pubkey-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	err := client.RemovePeer(ctx, "client-pubkey-123")
	if err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	}))
	defer server.Close()

	client := cloud.NewClient(server.URL, "test-api-key")

	ctx := context.Background()
	_, err := client.CreateEnvironment(ctx, cloud.CreateEnvironmentRequest{
		Name:    "test-env",
		Project: "test-project",
	})

	if err == nil {
		t.Error("expected error for server error response")
	}
}
