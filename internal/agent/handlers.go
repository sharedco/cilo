// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleHealth returns the health status of the agent.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "healthy",
		"machine_id": s.config.MachineID,
	})
}

// ============================================================================
// Auth Handlers
// ============================================================================

// HandleAuthConnect handles POST /auth/connect
func (s *Server) HandleAuthConnect(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleConnect(w, r)
}

// HandleAuthChallenge handles POST /auth/challenge
func (s *Server) HandleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleChallenge(w, r)
}

// HandleAuthDisconnect handles DELETE /auth/disconnect
func (s *Server) HandleAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleDisconnect(w, r)
}

// ============================================================================
// Environment Handlers
// ============================================================================

// HandleListEnvironments handles GET /environments
func (s *Server) HandleListEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := s.envManager.List(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, ListEnvironmentsResponse{Environments: envs})
}

// HandleEnvironmentUp handles POST /environments/:name/up
func (s *Server) HandleEnvironmentUp(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	var req EnvironmentUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	upReq := UpRequest{
		EnvName:       envName,
		WorkspacePath: req.WorkspacePath,
		Build:         req.Build,
		Recreate:      req.Recreate,
	}

	resp, err := s.envManager.Up(r.Context(), upReq)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	upResp := EnvironmentUpResponse{
		Name:     envName,
		Status:   resp.Status,
		Services: resp.Services,
	}

	respondJSON(w, http.StatusOK, upResp)
}

// HandleEnvironmentDown handles POST /environments/:name/down
func (s *Server) HandleEnvironmentDown(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	var req EnvironmentDownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := s.envManager.Down(r.Context(), envName); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, EnvironmentDownResponse{
		Name:   envName,
		Status: "stopped",
	})
}

// HandleEnvironmentDestroy handles DELETE /environments/:name
func (s *Server) HandleEnvironmentDestroy(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	if err := s.envManager.Destroy(r.Context(), envName); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, EnvironmentDestroyResponse{
		Name:   envName,
		Status: "destroyed",
	})
}

// HandleEnvironmentStatus handles GET /environments/:name/status
func (s *Server) HandleEnvironmentStatus(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	services, err := s.envManager.Status(r.Context(), envName)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var serviceDetails []ServiceStatusDetail
	for name, status := range services {
		serviceDetails = append(serviceDetails, ServiceStatusDetail{
			Name:   name,
			State:  status.State,
			Status: status.Status,
			Health: status.Health,
		})
	}

	overallStatus := "stopped"
	if len(services) > 0 {
		running := 0
		for _, s := range services {
			if s.State == "running" {
				running++
			}
		}
		if running == len(services) {
			overallStatus = "running"
		} else if running > 0 {
			overallStatus = "partial"
		}
	}

	respondJSON(w, http.StatusOK, EnvironmentStatusResponse{
		Name:     envName,
		Status:   overallStatus,
		Services: serviceDetails,
	})
}

// HandleEnvironmentLogs handles GET /environments/:name/logs
func (s *Server) HandleEnvironmentLogs(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	service := r.URL.Query().Get("service")
	if service == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "service query parameter is required"})
		return
	}

	follow := r.URL.Query().Get("follow") == "true"

	if r.Header.Get("Upgrade") == "websocket" {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "WebSocket streaming not yet implemented"})
		return
	}

	logs, err := s.envManager.Logs(r.Context(), envName, service, follow)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer logs.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, logs); err != nil {
		return
	}
}

// HandleEnvironmentExec handles POST /environments/:name/exec
func (s *Server) HandleEnvironmentExec(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	if r.Header.Get("Upgrade") == "websocket" {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "WebSocket exec not yet implemented"})
		return
	}

	respondJSON(w, http.StatusBadRequest, map[string]string{"error": "WebSocket upgrade required"})
}

// ============================================================================
// WireGuard Handlers
// ============================================================================

// HandleWireGuardExchange handles POST /wireguard/exchange
func (s *Server) HandleWireGuardExchange(w http.ResponseWriter, r *http.Request) {
	var req WireGuardExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.PublicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "public_key is required"})
		return
	}

	assignedIP, err := s.peerStore.AllocatePeerIP(req.PublicKey)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	allowedIPs := []string{assignedIP + "/32"}
	if err := s.wgManager.AddPeer(r.Context(), req.PublicKey, allowedIPs); err != nil {
		s.peerStore.RemovePeer(req.PublicKey)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	serverPublicKey := s.wgManager.GetPublicKey()
	serverEndpoint := s.config.WGAddress
	if idx := strings.Index(serverEndpoint, "/"); idx != -1 {
		serverEndpoint = serverEndpoint[:idx] + ":" + "51820"
	}

	respondJSON(w, http.StatusOK, WireGuardExchangeResponse{
		ServerPublicKey: serverPublicKey,
		ServerEndpoint:  serverEndpoint,
		AssignedIP:      assignedIP,
		AllowedIPs:      []string{"10.225.0.0/16"},
	})
}

// HandleWireGuardRemovePeer handles DELETE /wireguard/peers/:key
func (s *Server) HandleWireGuardRemovePeer(w http.ResponseWriter, r *http.Request) {
	publicKey := chi.URLParam(r, "key")
	if publicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "public key is required"})
		return
	}

	if err := s.wgManager.RemovePeer(r.Context(), publicKey); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := s.peerStore.RemovePeer(publicKey); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, WireGuardRemovePeerResponse{
		PublicKey: publicKey,
		Status:    "removed",
	})
}

// HandleWireGuardStatus handles GET /wireguard/status
func (s *Server) HandleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.wgManager.GetStatus(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	peers := make([]WireGuardPeer, len(status.Peers))
	for i, p := range status.Peers {
		assignedIP, _ := s.peerStore.GetPeerIP(p.PublicKey)
		peers[i] = WireGuardPeer{
			PublicKey:       p.PublicKey,
			Endpoint:        p.Endpoint,
			AllowedIPs:      p.AllowedIPs,
			LatestHandshake: p.LastHandshake,
			RxBytes:         p.RxBytes,
			TxBytes:         p.TxBytes,
			AssignedIP:      assignedIP,
		}
	}

	respondJSON(w, http.StatusOK, WireGuardStatusResponse{
		Interface:  s.config.WGInterface,
		PublicKey:  status.PublicKey,
		ListenPort: s.config.WGListenPort,
		Address:    s.config.WGAddress,
		Peers:      peers,
	})
}

// ============================================================================
// Workspace Sync Handlers
// ============================================================================

// HandleWorkspaceSync handles POST /sync/:name
func (s *Server) HandleWorkspaceSync(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "environment name is required"})
		return
	}

	var req WorkspaceSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "workspace sync not yet implemented"})
}

// ============================================================================
// Legacy Handlers
// ============================================================================

func (s *Server) handleUp(w http.ResponseWriter, r *http.Request) {
	var req UpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := s.envManager.Up(r.Context(), req)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnvName string `json:"env_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.EnvName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "env_name is required"})
		return
	}

	if err := s.envManager.Down(r.Context(), req.EnvName); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	envName := r.URL.Query().Get("env_name")
	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "env_name query parameter is required"})
		return
	}

	statuses, err := s.envManager.Status(r.Context(), envName)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, statuses)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	envName := r.URL.Query().Get("env_name")
	follow := r.URL.Query().Get("follow") == "true"

	if envName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "env_name query parameter is required"})
		return
	}

	if service == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "service name is required"})
		return
	}

	logs, err := s.envManager.Logs(r.Context(), envName, service, follow)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer logs.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, logs); err != nil {
		return
	}
}

func (s *Server) handleAddPeer(w http.ResponseWriter, r *http.Request) {
	var req AddPeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.PublicKey) != 44 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "public_key must be 44 characters"})
		return
	}

	allowedIPs := splitAllowedIPs(req.AllowedIPs)
	if len(allowedIPs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "allowed_ips is required"})
		return
	}

	if err := s.wgManager.AddPeer(r.Context(), req.PublicKey, allowedIPs); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "peer added"})
}

func (s *Server) handleRemovePeer(w http.ResponseWriter, r *http.Request) {
	publicKey := chi.URLParam(r, "key")
	if publicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "public key is required"})
		return
	}

	if err := s.wgManager.RemovePeer(r.Context(), publicKey); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "peer removed"})
}

func (s *Server) handleWGStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.wgManager.GetStatus(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func splitAllowedIPs(allowedIPs string) []string {
	if allowedIPs == "" {
		return nil
	}
	parts := strings.Split(allowedIPs, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
