// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cilod

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// ============================================================================
// Test Client Connect
// ============================================================================

func TestClientConnect(t *testing.T) {
	// Track requests
	var challengeRequest *AuthChallengeRequest
	var connectRequest *AuthConnectRequest
	challenge := "test-challenge-12345"
	token := "test-session-token-abc123"

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/challenge":
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			var req AuthChallengeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode challenge request: %v", err)
			}
			challengeRequest = &req

			resp := AuthChallengeResponse{
				Challenge: challenge,
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/auth/connect":
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			var req AuthConnectRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode connect request: %v", err)
			}
			connectRequest = &req

			// Verify the challenge was signed correctly
			if req.Challenge != challenge {
				t.Errorf("Expected challenge %s, got %s", challenge, req.Challenge)
			}

			resp := AuthConnectResponse{
				Token:     token,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			t.Errorf("Unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Generate test SSH key
	privateKeyPath := generateTestSSHKey(t)
	defer os.Remove(privateKeyPath)

	// Create client and connect
	client := NewClient(server.URL, "")
	receivedToken, err := client.Connect(privateKeyPath)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Verify token received
	if receivedToken != token {
		t.Errorf("Expected token %s, got %s", token, receivedToken)
	}

	// Verify challenge request was made with public key
	if challengeRequest == nil {
		t.Fatal("Challenge request was not made")
	}
	if challengeRequest.PublicKey == "" {
		t.Error("Challenge request missing public key")
	}

	// Verify connect request was made with signature
	if connectRequest == nil {
		t.Fatal("Connect request was not made")
	}
	if connectRequest.Signature == "" {
		t.Error("Connect request missing signature")
	}

	// Verify client token was set
	if client.token != token {
		t.Errorf("Client token not set, expected %s", token)
	}
}

func TestClientConnect_InvalidKey(t *testing.T) {
	client := NewClient("http://localhost:9999", "")
	_, err := client.Connect("/nonexistent/key")
	if err == nil {
		t.Error("Expected error for invalid key path")
	}
}

// ============================================================================
// Test Client List Environments
// ============================================================================

func TestClientListEnvironments(t *testing.T) {
	envs := []Environment{
		{
			Name:      "dev",
			Status:    "running",
			CreatedAt: time.Now(),
			Services:  []string{"api", "db"},
			Subnet:    "10.225.0.0/24",
		},
		{
			Name:      "staging",
			Status:    "stopped",
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Services:  []string{"api", "db", "cache"},
			Subnet:    "10.225.1.0/24",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/environments" {
			t.Errorf("Expected /environments, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Expected Bearer test-token, got %s", authHeader)
		}

		resp := ListEnvironmentsResponse{Environments: envs}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	result, err := client.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 environments, got %d", len(result))
	}

	if result[0].Name != "dev" {
		t.Errorf("Expected first env name 'dev', got %s", result[0].Name)
	}

	if result[1].Status != "stopped" {
		t.Errorf("Expected second env status 'stopped', got %s", result[1].Status)
	}
}

// ============================================================================
// Test Client Up Environment
// ============================================================================

func TestClientUpEnvironment(t *testing.T) {
	var receivedRequest EnvironmentUpRequest
	var receivedName string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/environments/") || !strings.HasSuffix(r.URL.Path, "/up") {
			t.Errorf("Expected /environments/:name/up, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Extract name from path
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			receivedName = parts[2]
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		resp := EnvironmentUpResponse{
			Name:   receivedName,
			Status: "running",
			Services: map[string]string{
				"api": "10.225.0.2",
				"db":  "10.225.0.3",
			},
			Subnet: "10.225.0.0/24",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	opts := UpOptions{
		WorkspacePath: "/tmp/workspace",
		Build:         true,
		Recreate:      false,
	}

	err := client.UpEnvironment("myenv", opts)
	if err != nil {
		t.Fatalf("UpEnvironment failed: %v", err)
	}

	if receivedName != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", receivedName)
	}

	if receivedRequest.WorkspacePath != "/tmp/workspace" {
		t.Errorf("Expected workspace path '/tmp/workspace', got %s", receivedRequest.WorkspacePath)
	}

	if !receivedRequest.Build {
		t.Error("Expected Build to be true")
	}
}

// ============================================================================
// Test Client Down Environment
// ============================================================================

func TestClientDownEnvironment(t *testing.T) {
	var receivedName string
	var receivedForce bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/environments/") || !strings.HasSuffix(r.URL.Path, "/down") {
			t.Errorf("Expected /environments/:name/down, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			receivedName = parts[2]
		}

		var req EnvironmentDownRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			receivedForce = req.Force
		}

		resp := EnvironmentDownResponse{
			Name:   receivedName,
			Status: "stopped",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.DownEnvironment("myenv")
	if err != nil {
		t.Fatalf("DownEnvironment failed: %v", err)
	}

	if receivedName != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", receivedName)
	}

	if receivedForce {
		t.Error("Expected Force to be false by default")
	}
}

// ============================================================================
// Test Client Destroy Environment
// ============================================================================

func TestClientDestroyEnvironment(t *testing.T) {
	var receivedName string
	var receivedForce bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/environments/") {
			t.Errorf("Expected /environments/:name, got %s", r.URL.Path)
		}
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			receivedName = parts[2]
		}

		// Check for force query param
		receivedForce = r.URL.Query().Get("force") == "true"

		resp := EnvironmentDestroyResponse{
			Name:   receivedName,
			Status: "destroyed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.DestroyEnvironment("myenv")
	if err != nil {
		t.Fatalf("DestroyEnvironment failed: %v", err)
	}

	if receivedName != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", receivedName)
	}

	if receivedForce {
		t.Error("Expected Force to be false by default")
	}
}

// ============================================================================
// Test Client Get Status
// ============================================================================

func TestClientGetStatus(t *testing.T) {
	expectedStatus := &EnvironmentStatus{
		Name:   "myenv",
		Status: "running",
		Services: []ServiceStatus{
			{
				Name:   "api",
				State:  "running",
				Status: "Up 2 hours",
				Health: "healthy",
				IP:     "10.225.0.2",
			},
		},
		Networks: []NetworkInfo{
			{
				Name:    "myenv_default",
				Subnet:  "10.225.0.0/24",
				Gateway: "10.225.0.1",
			},
		},
		LastActive: time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/environments/myenv/status"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected %s, got %s", expectedPath, r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedStatus)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	status, err := client.GetStatus("myenv")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Name != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", status.Name)
	}

	if status.Status != "running" {
		t.Errorf("Expected status 'running', got %s", status.Status)
	}

	if len(status.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(status.Services))
	}

	if status.Services[0].Name != "api" {
		t.Errorf("Expected service name 'api', got %s", status.Services[0].Name)
	}
}

// ============================================================================
// Test Client WireGuard Exchange
// ============================================================================

func TestClientWireGuardExchange(t *testing.T) {
	clientPubKey := "client-public-key-123"
	serverPubKey := "server-public-key-456"
	assignedIP := "10.225.0.5"
	endpoint := "192.168.1.100:51820"

	var receivedRequest WireGuardExchangeRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wireguard/exchange" {
			t.Errorf("Expected /wireguard/exchange, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		resp := WireGuardExchangeResponse{
			ServerPublicKey:   serverPubKey,
			ServerEndpoint:    endpoint,
			AssignedIP:        assignedIP,
			AllowedIPs:        []string{"10.225.0.0/24"},
			EnvironmentSubnet: "10.225.0.0/24",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	config, err := client.WireGuardExchange(clientPubKey)
	if err != nil {
		t.Fatalf("WireGuardExchange failed: %v", err)
	}

	if receivedRequest.PublicKey != clientPubKey {
		t.Errorf("Expected public key %s, got %s", clientPubKey, receivedRequest.PublicKey)
	}

	if config.ServerPublicKey != serverPubKey {
		t.Errorf("Expected server public key %s, got %s", serverPubKey, config.ServerPublicKey)
	}

	if config.AssignedIP != assignedIP {
		t.Errorf("Expected assigned IP %s, got %s", assignedIP, config.AssignedIP)
	}

	if config.ServerEndpoint != endpoint {
		t.Errorf("Expected endpoint %s, got %s", endpoint, config.ServerEndpoint)
	}

	if len(config.AllowedIPs) != 1 || config.AllowedIPs[0] != "10.225.0.0/24" {
		t.Errorf("Expected allowed IPs [10.225.0.0/24], got %v", config.AllowedIPs)
	}
}

// ============================================================================
// Test Client Error Handling
// ============================================================================

func TestClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorBody  string
		wantErr    string
	}{
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			errorBody:  `{"error": "invalid request"}`,
			wantErr:    "invalid request",
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			errorBody:  `{"error": "unauthorized"}`,
			wantErr:    "unauthorized",
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			errorBody:  `{"error": "environment not found"}`,
			wantErr:    "environment not found",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			errorBody:  `{"error": "internal error"}`,
			wantErr:    "internal error",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			errorBody:  `{"error": "service unavailable"}`,
			wantErr:    "service unavailable",
		},
		{
			name:       "Error without JSON body",
			statusCode: http.StatusBadRequest,
			errorBody:  "plain text error",
			wantErr:    "400 Bad Request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.errorBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			_, err := client.ListEnvironments()

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// ============================================================================
// Test Client Timeout
// ============================================================================

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than client timeout
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListEnvironmentsResponse{})
	}))
	defer server.Close()

	// Create client with very short timeout
	client := NewClient(server.URL, "test-token")
	client.SetTimeout(50 * time.Millisecond)

	_, err := client.ListEnvironments()
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// ============================================================================
// Test Client Retry Logic
// ============================================================================

func TestClientRetry(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Return transient error for first 2 attempts
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "temporarily unavailable"})
			return
		}
		// Success on 3rd attempt
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListEnvironmentsResponse{
			Environments: []Environment{
				{Name: "test", Status: "running"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	client.SetRetryPolicy(3, 10*time.Millisecond)

	envs, err := client.ListEnvironments()
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	if len(envs) != 1 {
		t.Errorf("Expected 1 environment, got %d", len(envs))
	}
}

func TestClientRetry_Exhausted(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "service down"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	client.SetRetryPolicy(2, 10*time.Millisecond)

	_, err := client.ListEnvironments()
	if err == nil {
		t.Fatal("Expected error after retries exhausted")
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}

	if !strings.Contains(err.Error(), "service down") {
		t.Errorf("Expected error containing 'service down', got: %v", err)
	}
}

// ============================================================================
// Test Stream Logs (stub - full impl in Task 11)
// ============================================================================

func TestClientStreamLogs(t *testing.T) {
	logData := []byte("log line 1\nlog line 2\nlog line 3\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/environments/myenv/logs"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected %s, got %s", expectedPath, r.URL.Path)
		}

		service := r.URL.Query().Get("service")
		if service != "api" {
			t.Errorf("Expected service 'api', got %s", service)
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write(logData)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	reader, err := client.StreamLogs("myenv", "api")
	if err != nil {
		t.Fatalf("StreamLogs failed: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}

	if !bytes.Equal(data, logData) {
		t.Errorf("Expected %q, got %q", logData, data)
	}
}

// ============================================================================
// Test Exec (stub - full WebSocket impl in Task 11)
// ============================================================================

func TestClientExec(t *testing.T) {
	var receivedRequest EnvironmentExecRequest
	var receivedName string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/environments/myenv/exec"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected %s, got %s", expectedPath, r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			receivedName = parts[2]
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Return 200 OK - full WebSocket upgrade in Task 11
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.Exec("myenv", "api", []string{"ls", "-la"})

	// For now, stub returns nil error (full impl in Task 11)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if receivedName != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", receivedName)
	}

	if receivedRequest.Service != "api" {
		t.Errorf("Expected service 'api', got %s", receivedRequest.Service)
	}

	if len(receivedRequest.Command) != 2 || receivedRequest.Command[0] != "ls" {
		t.Errorf("Expected command ['ls', '-la'], got %v", receivedRequest.Command)
	}
}

// ============================================================================
// Test Sync Workspace (stub - rsync over SSH)
// ============================================================================

func TestClientSyncWorkspace(t *testing.T) {
	var receivedName string
	var receivedSyncType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/sync/") {
			t.Errorf("Expected /sync/:name, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			receivedName = parts[2]
		}

		var req WorkspaceSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		receivedSyncType = req.SyncType

		resp := WorkspaceSyncResponse{
			EnvironmentName: receivedName,
			FilesReceived:   10,
			FilesUpdated:    5,
			FilesDeleted:    0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.SyncWorkspace("myenv", "/tmp/workspace")

	// For now, stub returns nil error (full rsync impl later)
	if err != nil {
		t.Fatalf("SyncWorkspace failed: %v", err)
	}

	if receivedName != "myenv" {
		t.Errorf("Expected name 'myenv', got %s", receivedName)
	}

	if receivedSyncType != "full" {
		t.Errorf("Expected sync type 'full', got %s", receivedSyncType)
	}
}

// ============================================================================
// Test Client Constructor
// ============================================================================

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080", "my-token")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL http://localhost:8080, got %s", client.baseURL)
	}

	if client.token != "my-token" {
		t.Errorf("Expected token my-token, got %s", client.token)
	}

	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}

	// Check default timeout
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClient_WithIP(t *testing.T) {
	client := NewClient("192.168.1.100:8080", "token")

	if client.baseURL != "http://192.168.1.100:8080" {
		t.Errorf("Expected baseURL with http:// prefix, got %s", client.baseURL)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func generateTestSSHKey(t *testing.T) string {
	t.Helper()

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key")

	// Write private key in PEM format
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}

	// Generate and write public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to generate public key: %v", err)
	}

	publicKeyPath := keyPath + ".pub"
	publicKeyData := ssh.MarshalAuthorizedKey(publicKey)
	if err := os.WriteFile(publicKeyPath, publicKeyData, 0644); err != nil {
		t.Fatalf("Failed to write public key: %v", err)
	}

	return keyPath
}
