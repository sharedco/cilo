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
