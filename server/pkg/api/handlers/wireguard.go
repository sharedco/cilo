package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sharedco/cilo/server/pkg/agent"
	"github.com/sharedco/cilo/server/pkg/store"
	"github.com/sharedco/cilo/server/pkg/wireguard"
)

// WireGuardHandler handles WireGuard key exchange and peer management endpoints
type WireGuardHandler struct {
	exchange *wireguard.Exchange
	store    *store.Store
}

// NewWireGuardHandler creates a new WireGuardHandler instance
func NewWireGuardHandler(exchange *wireguard.Exchange, store *store.Store) *WireGuardHandler {
	return &WireGuardHandler{
		exchange: exchange,
		store:    store,
	}
}

// ExchangeRequest represents a WireGuard key exchange request
type ExchangeRequest struct {
	EnvironmentID string `json:"environment_id"`
	UserID        string `json:"user_id"`
	PublicKey     string `json:"public_key"`
	MachineID     string `json:"machine_id"`
}

// ExchangeResponse represents a WireGuard key exchange response
type ExchangeResponse struct {
	MachinePublicKey string   `json:"machine_public_key"`
	MachineEndpoint  string   `json:"endpoint"`
	AssignedIP       string   `json:"assigned_ip"`
	AllowedIPs       []string `json:"allowed_ips"`
}

// HandleWireGuardExchange handles POST /v1/wireguard/exchange
// This endpoint allows a client to exchange their public key with a machine
// and receive the machine's public key, endpoint, and assigned IP
func (h *WireGuardHandler) HandleWireGuardExchange(w http.ResponseWriter, r *http.Request) {
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

	// TODO: Fetch machine info from database
	// For now, we'll use placeholder values
	// In production, this should query the machines table
	machineInfo := wireguard.MachineInfo{
		ID:                req.MachineID,
		PublicKey:         "MACHINE_PUBLIC_KEY_PLACEHOLDER",
		Endpoint:          "1.2.3.4:51820",
		EnvironmentSubnet: "10.224.1.0/24",
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

	if err := h.notifyAgentAddPeer(r.Context(), req.MachineID, req.PublicKey, peerConfig.AssignedIP, machineInfo.EnvironmentSubnet); err != nil {
		fmt.Printf("Warning: failed to notify agent to add peer: %v\n", err)
	}

	// Return peer configuration
	respondJSON(w, http.StatusOK, ExchangeResponse{
		MachinePublicKey: peerConfig.MachinePublicKey,
		MachineEndpoint:  peerConfig.MachineEndpoint,
		AssignedIP:       peerConfig.AssignedIP,
		AllowedIPs:       peerConfig.AllowedIPs,
	})
}

// HandleWireGuardRemovePeer handles DELETE /v1/wireguard/peers/:key
// This endpoint removes a peer from a machine's WireGuard configuration
func (h *WireGuardHandler) HandleWireGuardRemovePeer(w http.ResponseWriter, r *http.Request) {
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

	if err := h.notifyAgentRemovePeer(r.Context(), machineID, publicKey); err != nil {
		fmt.Printf("Warning: failed to notify agent to remove peer: %v\n", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Peer removed successfully",
	})
}

// PeerInfo represents peer information in the status response
type PeerInfo struct {
	PublicKey     string `json:"public_key"`
	AssignedIP    string `json:"assigned_ip"`
	EnvironmentID string `json:"environment_id"`
	UserID        string `json:"user_id"`
}

// StatusResponse represents the WireGuard status response
type StatusResponse struct {
	EnvironmentID string     `json:"environment_id"`
	Peers         []PeerInfo `json:"peers"`
	TotalPeers    int        `json:"total_peers"`
}

// HandleWireGuardStatus handles GET /v1/wireguard/status/:environment_id
// This endpoint returns the status of all peers connected to an environment
func (h *WireGuardHandler) HandleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
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

func (h *WireGuardHandler) notifyAgentAddPeer(ctx context.Context, machineID, publicKey, assignedIP, environmentSubnet string) error {
	machine, err := h.store.GetMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("failed to get machine: %w", err)
	}

	agentAddr := extractAgentAddress(machine.WGEndpoint)
	agentClient := agent.NewClient(agentAddr)

	allowedIPs := fmt.Sprintf("%s/32,%s", assignedIP, environmentSubnet)
	req := agent.AddPeerRequest{
		PublicKey:  publicKey,
		AllowedIPs: allowedIPs,
	}

	if err := agentClient.AddPeer(ctx, req); err != nil {
		return fmt.Errorf("failed to add peer via agent: %w", err)
	}

	return nil
}

// notifyAgentRemovePeer notifies the agent to remove a peer from the WireGuard interface
func (h *WireGuardHandler) notifyAgentRemovePeer(ctx context.Context, machineID, publicKey string) error {
	machine, err := h.store.GetMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("failed to get machine: %w", err)
	}

	agentAddr := extractAgentAddress(machine.WGEndpoint)
	agentClient := agent.NewClient(agentAddr)

	if err := agentClient.RemovePeer(ctx, publicKey); err != nil {
		return fmt.Errorf("failed to remove peer via agent: %w", err)
	}

	return nil
}

// extractAgentAddress extracts the agent address from the WireGuard endpoint
// WGEndpoint format: "10.225.0.100:51820" -> agent at "http://10.225.0.100:8080"
// IPv6 format: "[2001:db8::1]:51820" -> agent at "http://[2001:db8::1]:8080"
func extractAgentAddress(wgEndpoint string) string {
	host, _, err := net.SplitHostPort(wgEndpoint)
	if err != nil {
		// If no port is present, use the whole endpoint as the host
		host = wgEndpoint
		// If the endpoint contains colons but isn't a valid bracketed IPv6, it's invalid
		// Return empty host in that case
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			host = ""
		}
	}
	// If host contains colons, it's an IPv6 address and needs brackets
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:8080", host)
}

// Helper functions

func getUserIDFromContext(r *http.Request) string {
	if v := r.Context().Value("user_id"); v != nil {
		return v.(string)
	}
	// Try team_id as fallback (from auth middleware)
	if v := r.Context().Value("team_id"); v != nil {
		return v.(string)
	}
	return ""
}
