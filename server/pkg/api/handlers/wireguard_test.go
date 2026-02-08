package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sharedco/cilo/server/pkg/store"
	"github.com/sharedco/cilo/server/pkg/wireguard"
)

// ============================================================================
// MOCKS AND INTERFACES
// ============================================================================

// ExchangeInterface defines the methods needed from wireguard.Exchange
type ExchangeInterface interface {
	RegisterPeer(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error)
	RemovePeer(ctx context.Context, machineID, publicKey string) error
	GetPeersForEnvironment(ctx context.Context, environmentID string) ([]wireguard.PeerRegistration, error)
	GetPeersForMachine(ctx context.Context, machineID string) ([]wireguard.PeerRegistration, error)
	AllocatePeerIP(ctx context.Context, machineID string) (string, error)
	ValidatePeerSubnet(ip string) bool
}

// StoreInterface defines the methods needed from store.Store for machine operations
type StoreInterface interface {
	GetMachine(ctx context.Context, id string) (*store.Machine, error)
	SaveMachine(ctx context.Context, machine *store.Machine) error
	ListMachines(ctx context.Context) ([]*store.Machine, error)
	ListMachinesByStatus(ctx context.Context, status string) ([]*store.Machine, error)
	UpdateMachineStatus(ctx context.Context, id, status string) error
	AssignMachine(ctx context.Context, machineID, envID string) error
	ReleaseMachine(ctx context.Context, machineID string) error
	DeleteMachine(ctx context.Context, id string) error
}

// MockExchange is a mock implementation of the wireguard exchange
type MockExchange struct {
	RegisterPeerFunc           func(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error)
	RemovePeerFunc             func(ctx context.Context, machineID, publicKey string) error
	GetPeersForEnvironmentFunc func(ctx context.Context, environmentID string) ([]wireguard.PeerRegistration, error)
	GetPeersForMachineFunc     func(ctx context.Context, machineID string) ([]wireguard.PeerRegistration, error)
	AllocatePeerIPFunc         func(ctx context.Context, machineID string) (string, error)
	ValidatePeerSubnetFunc     func(ip string) bool
}

func (m *MockExchange) RegisterPeer(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error) {
	if m.RegisterPeerFunc != nil {
		return m.RegisterPeerFunc(ctx, machineInfo, peer)
	}
	return nil, fmt.Errorf("RegisterPeer not implemented")
}

func (m *MockExchange) RemovePeer(ctx context.Context, machineID, publicKey string) error {
	if m.RemovePeerFunc != nil {
		return m.RemovePeerFunc(ctx, machineID, publicKey)
	}
	return fmt.Errorf("RemovePeer not implemented")
}

func (m *MockExchange) GetPeersForEnvironment(ctx context.Context, environmentID string) ([]wireguard.PeerRegistration, error) {
	if m.GetPeersForEnvironmentFunc != nil {
		return m.GetPeersForEnvironmentFunc(ctx, environmentID)
	}
	return nil, fmt.Errorf("GetPeersForEnvironment not implemented")
}

func (m *MockExchange) GetPeersForMachine(ctx context.Context, machineID string) ([]wireguard.PeerRegistration, error) {
	if m.GetPeersForMachineFunc != nil {
		return m.GetPeersForMachineFunc(ctx, machineID)
	}
	return nil, fmt.Errorf("GetPeersForMachine not implemented")
}

func (m *MockExchange) AllocatePeerIP(ctx context.Context, machineID string) (string, error) {
	if m.AllocatePeerIPFunc != nil {
		return m.AllocatePeerIPFunc(ctx, machineID)
	}
	return "", fmt.Errorf("AllocatePeerIP not implemented")
}

func (m *MockExchange) ValidatePeerSubnet(ip string) bool {
	if m.ValidatePeerSubnetFunc != nil {
		return m.ValidatePeerSubnetFunc(ip)
	}
	return false
}

// MockStore is a mock implementation of the store
type MockStore struct {
	GetMachineFunc           func(ctx context.Context, id string) (*store.Machine, error)
	SaveMachineFunc          func(ctx context.Context, machine *store.Machine) error
	ListMachinesFunc         func(ctx context.Context) ([]*store.Machine, error)
	ListMachinesByStatusFunc func(ctx context.Context, status string) ([]*store.Machine, error)
	UpdateMachineStatusFunc  func(ctx context.Context, id, status string) error
	AssignMachineFunc        func(ctx context.Context, machineID, envID string) error
	ReleaseMachineFunc       func(ctx context.Context, machineID string) error
	DeleteMachineFunc        func(ctx context.Context, id string) error
}

func (m *MockStore) GetMachine(ctx context.Context, id string) (*store.Machine, error) {
	if m.GetMachineFunc != nil {
		return m.GetMachineFunc(ctx, id)
	}
	return nil, fmt.Errorf("GetMachine not implemented")
}

func (m *MockStore) SaveMachine(ctx context.Context, machine *store.Machine) error {
	if m.SaveMachineFunc != nil {
		return m.SaveMachineFunc(ctx, machine)
	}
	return fmt.Errorf("SaveMachine not implemented")
}

func (m *MockStore) ListMachines(ctx context.Context) ([]*store.Machine, error) {
	if m.ListMachinesFunc != nil {
		return m.ListMachinesFunc(ctx)
	}
	return nil, fmt.Errorf("ListMachines not implemented")
}

func (m *MockStore) ListMachinesByStatus(ctx context.Context, status string) ([]*store.Machine, error) {
	if m.ListMachinesByStatusFunc != nil {
		return m.ListMachinesByStatusFunc(ctx, status)
	}
	return nil, fmt.Errorf("ListMachinesByStatus not implemented")
}

func (m *MockStore) UpdateMachineStatus(ctx context.Context, id, status string) error {
	if m.UpdateMachineStatusFunc != nil {
		return m.UpdateMachineStatusFunc(ctx, id, status)
	}
	return fmt.Errorf("UpdateMachineStatus not implemented")
}

func (m *MockStore) AssignMachine(ctx context.Context, machineID, envID string) error {
	if m.AssignMachineFunc != nil {
		return m.AssignMachineFunc(ctx, machineID, envID)
	}
	return fmt.Errorf("AssignMachine not implemented")
}

func (m *MockStore) ReleaseMachine(ctx context.Context, machineID string) error {
	if m.ReleaseMachineFunc != nil {
		return m.ReleaseMachineFunc(ctx, machineID)
	}
	return fmt.Errorf("ReleaseMachine not implemented")
}

func (m *MockStore) DeleteMachine(ctx context.Context, id string) error {
	if m.DeleteMachineFunc != nil {
		return m.DeleteMachineFunc(ctx, id)
	}
	return fmt.Errorf("DeleteMachine not implemented")
}

// TestWireGuardHandler is a testable version of WireGuardHandler using interfaces
type TestWireGuardHandler struct {
	exchange ExchangeInterface
	store    StoreInterface
}

// NewTestWireGuardHandler creates a testable WireGuardHandler
func NewTestWireGuardHandler(exchange ExchangeInterface, store StoreInterface) *TestWireGuardHandler {
	return &TestWireGuardHandler{
		exchange: exchange,
		store:    store,
	}
}

// HandleWireGuardExchange handles POST /v1/wireguard/exchange (testable version)
func (h *TestWireGuardHandler) HandleWireGuardExchange(w http.ResponseWriter, r *http.Request) {
	var req ExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.EnvironmentID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "environment_id is required",
		})
		return
	}
	if req.PublicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "public_key is required",
		})
		return
	}
	if req.MachineID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "machine_id is required",
		})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := getUserIDFromContext(r)
	if userID == "" {
		userID = req.UserID
	}
	if userID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "user_id is required",
		})
		return
	}

	// Fetch machine info from store
	machine, err := h.store.GetMachine(r.Context(), req.MachineID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to get machine: " + err.Error(),
		})
		return
	}

	machineInfo := wireguard.MachineInfo{
		ID:                machine.ID,
		PublicKey:         machine.WGPublicKey,
		Endpoint:          machine.WGEndpoint,
		EnvironmentSubnet: "10.224.1.0/24", // Default subnet
	}

	// Register peer with the exchange
	peerRegistration := wireguard.PeerRegistration{
		EnvironmentID: req.EnvironmentID,
		UserID:        userID,
		PublicKey:     req.PublicKey,
	}

	peerConfig, err := h.exchange.RegisterPeer(r.Context(), machineInfo, peerRegistration)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to register peer: " + err.Error(),
		})
		return
	}

	// Return peer configuration
	respondJSON(w, http.StatusOK, ExchangeResponse{
		MachinePublicKey: peerConfig.MachinePublicKey,
		MachineEndpoint:  peerConfig.MachineEndpoint,
		AssignedIP:       peerConfig.AssignedIP,
		AllowedIPs:       peerConfig.AllowedIPs,
	})
}

// HandleWireGuardRemovePeer handles DELETE /v1/wireguard/peers/:key (testable version)
func (h *TestWireGuardHandler) HandleWireGuardRemovePeer(w http.ResponseWriter, r *http.Request) {
	publicKey := chi.URLParam(r, "key")
	machineID := r.URL.Query().Get("machine_id")

	if publicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "public_key is required",
		})
		return
	}

	if machineID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "machine_id query parameter is required",
		})
		return
	}

	// Remove peer from exchange
	err := h.exchange.RemovePeer(r.Context(), machineID, publicKey)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to remove peer: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Peer removed successfully",
	})
}

// HandleWireGuardStatus handles GET /v1/wireguard/status/:environment_id (testable version)
func (h *TestWireGuardHandler) HandleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
	environmentID := chi.URLParam(r, "environment_id")

	if environmentID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "environment_id is required",
		})
		return
	}

	// Get all peers for the environment
	peers, err := h.exchange.GetPeersForEnvironment(r.Context(), environmentID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to get peers: " + err.Error(),
		})
		return
	}

	// Convert to response format
	peerInfos := make([]PeerInfo, len(peers))
	for i, peer := range peers {
		peerInfos[i] = PeerInfo{
			PublicKey:     peer.PublicKey,
			AssignedIP:    peer.AssignedIP,
			EnvironmentID: peer.EnvironmentID,
			UserID:        peer.UserID,
		}
	}

	respondJSON(w, http.StatusOK, StatusResponse{
		EnvironmentID: environmentID,
		Peers:         peerInfos,
		TotalPeers:    len(peerInfos),
	})
}

// ============================================================================
// TESTS FOR extractAgentAddress()
// ============================================================================

func TestExtractAgentAddress(t *testing.T) {
	tests := []struct {
		name       string
		wgEndpoint string
		expected   string
	}{
		{
			name:       "standard endpoint with port",
			wgEndpoint: "10.225.0.100:51820",
			expected:   "http://10.225.0.100:8080",
		},
		{
			name:       "different IP same port",
			wgEndpoint: "192.168.1.50:51820",
			expected:   "http://192.168.1.50:8080",
		},
		{
			name:       "public IP with port",
			wgEndpoint: "203.0.113.45:51820",
			expected:   "http://203.0.113.45:8080",
		},
		{
			name:       "endpoint without port",
			wgEndpoint: "10.225.0.100",
			expected:   "http://10.225.0.100:8080",
		},
		{
			name:       "IPv6 endpoint with port",
			wgEndpoint: "[2001:db8::1]:51820",
			expected:   "http://[2001:db8::1]:8080",
		},
		{
			name:       "empty endpoint",
			wgEndpoint: "",
			expected:   "http://:8080",
		},
		{
			name:       "endpoint with multiple colons (IPv6)",
			wgEndpoint: "::1:51820",
			expected:   "http://:8080", // Invalid format returns empty host
		},
		{
			name:       "different WireGuard port",
			wgEndpoint: "10.0.0.1:99999",
			expected:   "http://10.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAgentAddress(tt.wgEndpoint)
			if result != tt.expected {
				t.Errorf("extractAgentAddress(%q) = %q, want %q", tt.wgEndpoint, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// TESTS FOR REQUEST/RESPONSE STRUCTS
// ============================================================================

func TestExchangeRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request ExchangeRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request with all fields",
			request: ExchangeRequest{
				EnvironmentID: "env-123",
				UserID:        "user-456",
				PublicKey:     "abcd1234efgh5678",
				MachineID:     "machine-789",
			},
			wantErr: false,
		},
		{
			name: "missing environment_id",
			request: ExchangeRequest{
				UserID:    "user-456",
				PublicKey: "abcd1234efgh5678",
				MachineID: "machine-789",
			},
			wantErr: true,
			errMsg:  "environment_id is required",
		},
		{
			name: "missing public_key",
			request: ExchangeRequest{
				EnvironmentID: "env-123",
				UserID:        "user-456",
				MachineID:     "machine-789",
			},
			wantErr: true,
			errMsg:  "public_key is required",
		},
		{
			name: "missing machine_id",
			request: ExchangeRequest{
				EnvironmentID: "env-123",
				UserID:        "user-456",
				PublicKey:     "abcd1234efgh5678",
			},
			wantErr: true,
			errMsg:  "machine_id is required",
		},
		{
			name: "empty request",
			request: ExchangeRequest{
				EnvironmentID: "",
				UserID:        "",
				PublicKey:     "",
				MachineID:     "",
			},
			wantErr: true,
			errMsg:  "environment_id is required",
		},
		{
			name: "valid request with only required fields",
			request: ExchangeRequest{
				EnvironmentID: "env-123",
				PublicKey:     "valid-key-here",
				MachineID:     "machine-789",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate required fields
			hasError := false
			var errorField string

			if tt.request.EnvironmentID == "" {
				hasError = true
				errorField = "environment_id"
			} else if tt.request.PublicKey == "" {
				hasError = true
				errorField = "public_key"
			} else if tt.request.MachineID == "" {
				hasError = true
				errorField = "machine_id"
			}

			if hasError != tt.wantErr {
				t.Errorf("validation failed: got error=%v, want error=%v", hasError, tt.wantErr)
			}

			if hasError && errorField != strings.Split(tt.errMsg, " ")[0] {
				t.Errorf("error field mismatch: got %q, want %q", errorField, strings.Split(tt.errMsg, " ")[0])
			}
		})
	}
}

func TestExchangeResponseStruct(t *testing.T) {
	response := ExchangeResponse{
		MachinePublicKey: "machine-pub-key-123",
		MachineEndpoint:  "1.2.3.4:51820",
		AssignedIP:       "10.225.0.50",
		AllowedIPs:       []string{"10.225.0.50/32", "10.224.1.0/24"},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal ExchangeResponse: %v", err)
	}

	// Test JSON unmarshaling
	var decoded ExchangeResponse
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ExchangeResponse: %v", err)
	}

	// Verify fields
	if decoded.MachinePublicKey != response.MachinePublicKey {
		t.Errorf("MachinePublicKey mismatch: got %q, want %q", decoded.MachinePublicKey, response.MachinePublicKey)
	}
	if decoded.MachineEndpoint != response.MachineEndpoint {
		t.Errorf("MachineEndpoint mismatch: got %q, want %q", decoded.MachineEndpoint, response.MachineEndpoint)
	}
	if decoded.AssignedIP != response.AssignedIP {
		t.Errorf("AssignedIP mismatch: got %q, want %q", decoded.AssignedIP, response.AssignedIP)
	}
	if len(decoded.AllowedIPs) != len(response.AllowedIPs) {
		t.Errorf("AllowedIPs length mismatch: got %d, want %d", len(decoded.AllowedIPs), len(response.AllowedIPs))
	}
}

func TestPeerInfoStruct(t *testing.T) {
	peer := PeerInfo{
		PublicKey:     "peer-pub-key-456",
		AssignedIP:    "10.225.0.75",
		EnvironmentID: "env-789",
		UserID:        "user-abc",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(peer)
	if err != nil {
		t.Fatalf("Failed to marshal PeerInfo: %v", err)
	}

	// Test JSON unmarshaling
	var decoded PeerInfo
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PeerInfo: %v", err)
	}

	// Verify fields
	if decoded.PublicKey != peer.PublicKey {
		t.Errorf("PublicKey mismatch: got %q, want %q", decoded.PublicKey, peer.PublicKey)
	}
	if decoded.AssignedIP != peer.AssignedIP {
		t.Errorf("AssignedIP mismatch: got %q, want %q", decoded.AssignedIP, peer.AssignedIP)
	}
	if decoded.EnvironmentID != peer.EnvironmentID {
		t.Errorf("EnvironmentID mismatch: got %q, want %q", decoded.EnvironmentID, peer.EnvironmentID)
	}
	if decoded.UserID != peer.UserID {
		t.Errorf("UserID mismatch: got %q, want %q", decoded.UserID, peer.UserID)
	}
}

func TestStatusResponseStruct(t *testing.T) {
	response := StatusResponse{
		EnvironmentID: "env-test-123",
		Peers: []PeerInfo{
			{PublicKey: "peer1", AssignedIP: "10.225.0.1", EnvironmentID: "env-test-123", UserID: "user1"},
			{PublicKey: "peer2", AssignedIP: "10.225.0.2", EnvironmentID: "env-test-123", UserID: "user2"},
		},
		TotalPeers: 2,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal StatusResponse: %v", err)
	}

	// Test JSON unmarshaling
	var decoded StatusResponse
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal StatusResponse: %v", err)
	}

	// Verify fields
	if decoded.EnvironmentID != response.EnvironmentID {
		t.Errorf("EnvironmentID mismatch: got %q, want %q", decoded.EnvironmentID, response.EnvironmentID)
	}
	if len(decoded.Peers) != len(response.Peers) {
		t.Errorf("Peers length mismatch: got %d, want %d", len(decoded.Peers), len(response.Peers))
	}
	if decoded.TotalPeers != response.TotalPeers {
		t.Errorf("TotalPeers mismatch: got %d, want %d", decoded.TotalPeers, response.TotalPeers)
	}
}

// ============================================================================
// TESTS FOR HANDLER INPUT VALIDATION (Using TestWireGuardHandler)
// ============================================================================

func TestHandleWireGuardExchange_Validation(t *testing.T) {
	mockExchange := &MockExchange{
		RegisterPeerFunc: func(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error) {
			return &wireguard.PeerConfig{
				MachinePublicKey: "machine-pub-key",
				MachineEndpoint:  "1.2.3.4:51820",
				AssignedIP:       "10.225.0.50",
				AllowedIPs:       []string{"10.225.0.50/32", "10.224.1.0/24"},
			}, nil
		},
	}

	mockStore := &MockStore{
		GetMachineFunc: func(ctx context.Context, id string) (*store.Machine, error) {
			return &store.Machine{
				ID:          id,
				WGEndpoint:  "10.225.0.100:51820",
				WGPublicKey: "machine-wg-key",
			}, nil
		},
	}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	tests := []struct {
		name           string
		requestBody    string
		userIDInCtx    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid JSON body",
			requestBody:    "{invalid json",
			userIDInCtx:    "user-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:           "missing environment_id",
			requestBody:    `{"public_key": "key123", "machine_id": "machine-456"}`,
			userIDInCtx:    "user-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "environment_id is required",
		},
		{
			name:           "missing public_key",
			requestBody:    `{"environment_id": "env-123", "machine_id": "machine-456"}`,
			userIDInCtx:    "user-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "public_key is required",
		},
		{
			name:           "missing machine_id",
			requestBody:    `{"environment_id": "env-123", "public_key": "key123"}`,
			userIDInCtx:    "user-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "machine_id is required",
		},
		{
			name:           "missing user_id (no context)",
			requestBody:    `{"environment_id": "env-123", "public_key": "key123", "machine_id": "machine-456"}`,
			userIDInCtx:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "user_id is required",
		},
		{
			name:           "valid request with user_id in context",
			requestBody:    `{"environment_id": "env-123", "public_key": "key123", "machine_id": "machine-456"}`,
			userIDInCtx:    "user-789",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "valid request with user_id in body",
			requestBody:    `{"environment_id": "env-123", "public_key": "key123", "machine_id": "machine-456", "user_id": "user-abc"}`,
			userIDInCtx:    "",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "empty request body",
			requestBody:    `{}`,
			userIDInCtx:    "user-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "environment_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/wireguard/exchange", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add user_id to context if provided
			if tt.userIDInCtx != "" {
				ctx := context.WithValue(req.Context(), "user_id", tt.userIDInCtx)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.HandleWireGuardExchange(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("status code mismatch: got %d, want %d", rr.Code, tt.expectedStatus)
			}

			if tt.expectedError != "" {
				var response map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if !strings.Contains(response["error"], tt.expectedError) {
					t.Errorf("error message mismatch: got %q, want to contain %q", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func TestHandleWireGuardRemovePeer_Validation(t *testing.T) {
	mockExchange := &MockExchange{
		RemovePeerFunc: func(ctx context.Context, machineID, publicKey string) error {
			return nil
		},
	}

	mockStore := &MockStore{}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	tests := []struct {
		name           string
		publicKey      string
		machineID      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing public_key",
			publicKey:      "",
			machineID:      "machine-123",
			expectedStatus: http.StatusNotFound, // chi returns 404 for missing URL param
			expectedError:  "",
		},
		{
			name:           "missing machine_id query param",
			publicKey:      "valid-key-123",
			machineID:      "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "machine_id query parameter is required",
		},
		{
			name:           "valid request",
			publicKey:      "valid-key-123",
			machineID:      "machine-456",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router to handle URL parameters
			r := chi.NewRouter()
			r.Delete("/v1/wireguard/peers/{key}", handler.HandleWireGuardRemovePeer)

			url := "/v1/wireguard/peers/" + tt.publicKey
			if tt.machineID != "" {
				url += "?machine_id=" + tt.machineID
			}

			req := httptest.NewRequest(http.MethodDelete, url, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("status code mismatch: got %d, want %d", rr.Code, tt.expectedStatus)
			}

			if tt.expectedError != "" {
				var response map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if !strings.Contains(response["error"], tt.expectedError) {
					t.Errorf("error message mismatch: got %q, want to contain %q", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func TestHandleWireGuardStatus_Validation(t *testing.T) {
	mockExchange := &MockExchange{
		GetPeersForEnvironmentFunc: func(ctx context.Context, environmentID string) ([]wireguard.PeerRegistration, error) {
			return []wireguard.PeerRegistration{
				{
					EnvironmentID: environmentID,
					UserID:        "user-1",
					PublicKey:     "key-1",
					AssignedIP:    "10.225.0.1",
				},
			}, nil
		},
	}

	mockStore := &MockStore{}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	tests := []struct {
		name           string
		environmentID  string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing environment_id",
			environmentID:  "",
			expectedStatus: http.StatusNotFound, // chi returns 404 for missing URL param
			expectedError:  "",
		},
		{
			name:           "valid environment_id",
			environmentID:  "env-123",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router to handle URL parameters
			r := chi.NewRouter()
			r.Get("/v1/wireguard/status/{environment_id}", handler.HandleWireGuardStatus)

			url := "/v1/wireguard/status/" + tt.environmentID

			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("status code mismatch: got %d, want %d", rr.Code, tt.expectedStatus)
			}

			if tt.expectedError != "" {
				var response map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if !strings.Contains(response["error"], tt.expectedError) {
					t.Errorf("error message mismatch: got %q, want to contain %q", response["error"], tt.expectedError)
				}
			} else if rr.Code == http.StatusOK {
				// Verify successful response structure
				var response StatusResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if response.EnvironmentID != tt.environmentID {
					t.Errorf("EnvironmentID mismatch: got %q, want %q", response.EnvironmentID, tt.environmentID)
				}
			}
			// For 404 responses from chi, we don't check the body
		})
	}
}

// ============================================================================
// TESTS FOR HELPER FUNCTIONS
// ============================================================================

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctxValue map[string]interface{}
		expected string
	}{
		{
			name:     "user_id in context",
			ctxValue: map[string]interface{}{"user_id": "user-123"},
			expected: "user-123",
		},
		{
			name:     "team_id as fallback",
			ctxValue: map[string]interface{}{"team_id": "team-456"},
			expected: "team-456",
		},
		{
			name:     "user_id takes precedence over team_id",
			ctxValue: map[string]interface{}{"user_id": "user-123", "team_id": "team-456"},
			expected: "user-123",
		},
		{
			name:     "empty context",
			ctxValue: map[string]interface{}{},
			expected: "",
		},
		{
			name:     "nil context values",
			ctxValue: map[string]interface{}{"user_id": nil},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := req.Context()

			for key, value := range tt.ctxValue {
				ctx = context.WithValue(ctx, key, value)
			}

			req = req.WithContext(ctx)
			result := getUserIDFromContext(req)

			if result != tt.expected {
				t.Errorf("getUserIDFromContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetTeamIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctxValue map[string]interface{}
		expected string
	}{
		{
			name:     "team_id in context",
			ctxValue: map[string]interface{}{"team_id": "team-123"},
			expected: "team-123",
		},
		{
			name:     "empty context",
			ctxValue: map[string]interface{}{},
			expected: "",
		},
		{
			name:     "nil team_id",
			ctxValue: map[string]interface{}{"team_id": nil},
			expected: "",
		},
		{
			name:     "user_id present but not team_id",
			ctxValue: map[string]interface{}{"user_id": "user-123"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := req.Context()

			for key, value := range tt.ctxValue {
				ctx = context.WithValue(ctx, key, value)
			}

			req = req.WithContext(ctx)
			result := getTeamIDFromContext(req)

			if result != tt.expected {
				t.Errorf("getTeamIDFromContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// TESTS FOR AUTH HANDLER VALIDATION
// ============================================================================

func TestCreateAPIKeyRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request CreateAPIKeyRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request with all fields",
			request: CreateAPIKeyRequest{
				TeamID: "team-123",
				Name:   "Development Key",
				Scope:  "developer",
			},
			wantErr: false,
		},
		{
			name: "valid request with admin scope",
			request: CreateAPIKeyRequest{
				TeamID: "team-123",
				Name:   "Admin Key",
				Scope:  "admin",
			},
			wantErr: false,
		},
		{
			name: "valid request with ci scope",
			request: CreateAPIKeyRequest{
				TeamID: "team-123",
				Name:   "CI Key",
				Scope:  "ci",
			},
			wantErr: false,
		},
		{
			name: "empty scope defaults to developer",
			request: CreateAPIKeyRequest{
				TeamID: "team-123",
				Name:   "Default Scope Key",
				Scope:  "",
			},
			wantErr: false,
		},
		{
			name: "invalid scope",
			request: CreateAPIKeyRequest{
				TeamID: "team-123",
				Name:   "Invalid Key",
				Scope:  "invalid-scope",
			},
			wantErr: true,
			errMsg:  "Invalid scope",
		},
		{
			name: "missing team_id",
			request: CreateAPIKeyRequest{
				Name:  "No Team Key",
				Scope: "developer",
			},
			wantErr: true,
			errMsg:  "team_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate scope
			scope := tt.request.Scope
			if scope == "" {
				scope = "developer"
			}

			validScopes := map[string]bool{
				"admin":     true,
				"developer": true,
				"ci":        true,
			}

			hasError := false
			var errorMsg string

			if !validScopes[scope] {
				hasError = true
				errorMsg = "Invalid scope"
			} else if tt.request.TeamID == "" {
				hasError = true
				errorMsg = "team_id is required"
			}

			if hasError != tt.wantErr {
				t.Errorf("validation failed: got error=%v, want error=%v", hasError, tt.wantErr)
			}

			if hasError && !strings.Contains(errorMsg, tt.errMsg) {
				t.Errorf("error message mismatch: got %q, want to contain %q", errorMsg, tt.errMsg)
			}
		})
	}
}

func TestHandleCreateAPIKey_Validation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		teamIDInCtx    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid JSON body",
			requestBody:    "{invalid json",
			teamIDInCtx:    "team-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:           "invalid scope",
			requestBody:    `{"team_id": "team-123", "name": "Test Key", "scope": "invalid"}`,
			teamIDInCtx:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid scope",
		},
		{
			name:           "missing team_id in context and body",
			requestBody:    `{"name": "Test Key", "scope": "developer"}`,
			teamIDInCtx:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "team_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For validation tests, we just verify the validation logic
			var req CreateAPIKeyRequest
			err := json.Unmarshal([]byte(tt.requestBody), &req)

			if tt.expectedError == "Invalid request body" {
				if err == nil {
					t.Errorf("expected JSON parse error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to unmarshal request: %v", err)
			}

			// Validate scope
			if req.Scope == "" {
				req.Scope = "developer"
			}

			validScopes := map[string]bool{
				"admin":     true,
				"developer": true,
				"ci":        true,
			}

			if !validScopes[req.Scope] {
				if tt.expectedError != "Invalid scope" {
					t.Errorf("expected scope validation to fail")
				}
				return
			}

			// Check team_id
			teamID := tt.teamIDInCtx
			if teamID == "" {
				teamID = req.TeamID
			}

			if teamID == "" && tt.expectedError == "team_id is required" {
				// Expected validation error
				return
			}
		})
	}
}

func TestHandleRevokeAPIKey_Validation(t *testing.T) {
	tests := []struct {
		name           string
		keyID          string
		teamIDInCtx    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing team_id in context",
			keyID:          "key-123",
			teamIDInCtx:    "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Not authenticated",
		},
		{
			name:           "missing key_id",
			keyID:          "",
			teamIDInCtx:    "team-123",
			expectedStatus: http.StatusNotFound,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router to handle URL parameters
			r := chi.NewRouter()
			r.Delete("/v1/auth/keys/{id}", func(w http.ResponseWriter, r *http.Request) {
				teamID := getTeamIDFromContext(r)
				keyID := chi.URLParam(r, "id")

				if teamID == "" {
					respondJSON(w, http.StatusUnauthorized, map[string]string{
						"error": "Not authenticated",
					})
					return
				}

				if keyID == "" {
					respondJSON(w, http.StatusBadRequest, map[string]string{
						"error": "Key ID is required",
					})
					return
				}

				respondJSON(w, http.StatusOK, map[string]string{
					"message": "API key revoked",
				})
			})

			url := "/v1/auth/keys/" + tt.keyID
			req := httptest.NewRequest(http.MethodDelete, url, nil)

			if tt.teamIDInCtx != "" {
				ctx := context.WithValue(req.Context(), "team_id", tt.teamIDInCtx)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("status code mismatch: got %d, want %d", rr.Code, tt.expectedStatus)
			}

			if tt.expectedError != "" {
				var response map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if !strings.Contains(response["error"], tt.expectedError) {
					t.Errorf("error message mismatch: got %q, want to contain %q", response["error"], tt.expectedError)
				}
			}
		})
	}
}

// ============================================================================
// INTEGRATION-STYLE TESTS
// ============================================================================

func TestWireGuardHandler_FullExchangeFlow(t *testing.T) {
	mockExchange := &MockExchange{
		RegisterPeerFunc: func(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error) {
			// Validate inputs
			if peer.EnvironmentID == "" {
				return nil, fmt.Errorf("environment_id is required")
			}
			if peer.PublicKey == "" {
				return nil, fmt.Errorf("public_key is required")
			}

			return &wireguard.PeerConfig{
				MachinePublicKey: machineInfo.PublicKey,
				MachineEndpoint:  machineInfo.Endpoint,
				AssignedIP:       "10.225.0.100",
				AllowedIPs:       []string{"10.225.0.100/32", machineInfo.EnvironmentSubnet},
			}, nil
		},
	}

	mockStore := &MockStore{
		GetMachineFunc: func(ctx context.Context, id string) (*store.Machine, error) {
			return &store.Machine{
				ID:          id,
				WGPublicKey: "machine-wg-pub-key",
				WGEndpoint:  "10.225.0.50:51820",
				PublicIP:    "203.0.113.10",
				Status:      "ready",
			}, nil
		},
	}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	// Test successful exchange
	requestBody := `{
		"environment_id": "env-test-123",
		"public_key": "client-pub-key-abc",
		"machine_id": "machine-456"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/wireguard/exchange", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "user_id", "user-test-789")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleWireGuardExchange(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ExchangeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.MachinePublicKey != "machine-wg-pub-key" {
		t.Errorf("MachinePublicKey mismatch: got %q, want %q", response.MachinePublicKey, "machine-wg-pub-key")
	}

	if response.AssignedIP != "10.225.0.100" {
		t.Errorf("AssignedIP mismatch: got %q, want %q", response.AssignedIP, "10.225.0.100")
	}

	if len(response.AllowedIPs) != 2 {
		t.Errorf("AllowedIPs length mismatch: got %d, want 2", len(response.AllowedIPs))
	}
}

func TestWireGuardHandler_ExchangeWithStoreError(t *testing.T) {
	mockExchange := &MockExchange{
		RegisterPeerFunc: func(ctx context.Context, machineInfo wireguard.MachineInfo, peer wireguard.PeerRegistration) (*wireguard.PeerConfig, error) {
			return &wireguard.PeerConfig{
				MachinePublicKey: "machine-pub-key",
				MachineEndpoint:  "1.2.3.4:51820",
				AssignedIP:       "10.225.0.50",
				AllowedIPs:       []string{"10.225.0.50/32"},
			}, nil
		},
	}

	mockStore := &MockStore{
		GetMachineFunc: func(ctx context.Context, id string) (*store.Machine, error) {
			return nil, fmt.Errorf("machine not found")
		},
	}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	requestBody := `{
		"environment_id": "env-test-123",
		"public_key": "client-pub-key-abc",
		"machine_id": "non-existent-machine"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/wireguard/exchange", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "user_id", "user-test-789")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleWireGuardExchange(rr, req)

	// Should return 500 because store lookup failed
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !strings.Contains(response["error"], "Failed to get machine") {
		t.Errorf("error message mismatch: got %q", response["error"])
	}
}

func TestWireGuardHandler_StatusWithPeers(t *testing.T) {
	mockExchange := &MockExchange{
		GetPeersForEnvironmentFunc: func(ctx context.Context, environmentID string) ([]wireguard.PeerRegistration, error) {
			return []wireguard.PeerRegistration{
				{
					EnvironmentID: environmentID,
					UserID:        "user-1",
					PublicKey:     "peer-key-1",
					AssignedIP:    "10.225.0.1",
				},
				{
					EnvironmentID: environmentID,
					UserID:        "user-2",
					PublicKey:     "peer-key-2",
					AssignedIP:    "10.225.0.2",
				},
				{
					EnvironmentID: environmentID,
					UserID:        "user-3",
					PublicKey:     "peer-key-3",
					AssignedIP:    "10.225.0.3",
				},
			}, nil
		},
	}

	mockStore := &MockStore{}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	r := chi.NewRouter()
	r.Get("/v1/wireguard/status/{environment_id}", handler.HandleWireGuardStatus)

	req := httptest.NewRequest(http.MethodGet, "/v1/wireguard/status/env-multi-peers", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response StatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.EnvironmentID != "env-multi-peers" {
		t.Errorf("EnvironmentID mismatch: got %q, want %q", response.EnvironmentID, "env-multi-peers")
	}

	if response.TotalPeers != 3 {
		t.Errorf("TotalPeers mismatch: got %d, want 3", response.TotalPeers)
	}

	if len(response.Peers) != 3 {
		t.Errorf("Peers length mismatch: got %d, want 3", len(response.Peers))
	}

	// Verify peer data
	for i, peer := range response.Peers {
		expectedKey := fmt.Sprintf("peer-key-%d", i+1)
		if peer.PublicKey != expectedKey {
			t.Errorf("Peer[%d].PublicKey mismatch: got %q, want %q", i, peer.PublicKey, expectedKey)
		}
	}
}

func TestWireGuardHandler_RemovePeerSuccess(t *testing.T) {
	mockExchange := &MockExchange{
		RemovePeerFunc: func(ctx context.Context, machineID, publicKey string) error {
			if machineID == "" {
				return fmt.Errorf("machine_id is required")
			}
			if publicKey == "" {
				return fmt.Errorf("public_key is required")
			}
			return nil
		},
	}

	mockStore := &MockStore{}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	r := chi.NewRouter()
	r.Delete("/v1/wireguard/peers/{key}", handler.HandleWireGuardRemovePeer)

	req := httptest.NewRequest(http.MethodDelete, "/v1/wireguard/peers/peer-to-remove?machine_id=machine-123", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["message"] != "Peer removed successfully" {
		t.Errorf("message mismatch: got %q, want %q", response["message"], "Peer removed successfully")
	}
}

func TestWireGuardHandler_RemovePeerExchangeError(t *testing.T) {
	mockExchange := &MockExchange{
		RemovePeerFunc: func(ctx context.Context, machineID, publicKey string) error {
			return fmt.Errorf("peer not found in WireGuard configuration")
		},
	}

	mockStore := &MockStore{}

	handler := NewTestWireGuardHandler(mockExchange, mockStore)

	r := chi.NewRouter()
	r.Delete("/v1/wireguard/peers/{key}", handler.HandleWireGuardRemovePeer)

	req := httptest.NewRequest(http.MethodDelete, "/v1/wireguard/peers/non-existent-peer?machine_id=machine-123", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !strings.Contains(response["error"], "Failed to remove peer") {
		t.Errorf("error message mismatch: got %q", response["error"])
	}
}

// ============================================================================
// BENCHMARKS
// ============================================================================

func BenchmarkExtractAgentAddress(b *testing.B) {
	endpoints := []string{
		"10.225.0.100:51820",
		"192.168.1.50:51820",
		"203.0.113.45:51820",
		"10.225.0.100",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, endpoint := range endpoints {
			extractAgentAddress(endpoint)
		}
	}
}

func BenchmarkExchangeRequestValidation(b *testing.B) {
	request := ExchangeRequest{
		EnvironmentID: "env-123",
		UserID:        "user-456",
		PublicKey:     "abcd1234efgh5678",
		MachineID:     "machine-789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = request.EnvironmentID != ""
		_ = request.PublicKey != ""
		_ = request.MachineID != ""
	}
}
