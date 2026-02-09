// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sharedco/cilo/internal/server/agent"
	"github.com/sharedco/cilo/internal/server/store"
)

type RegisterMachineRequest struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	SSHUser string `json:"ssh_user"`
	Region  string `json:"region,omitempty"`
	Size    string `json:"size,omitempty"`
}

type MachineResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Host         string  `json:"host"`
	Status       string  `json:"status"`
	AssignedEnv  *string `json:"assigned_env,omitempty"`
	ProviderType string  `json:"provider_type"`
	CreatedAt    string  `json:"created_at"`
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func (s *Server) handleValidateAuth(w http.ResponseWriter, r *http.Request) {
	teamID := getTeamID(r)
	if teamID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Not authenticated",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"team_id":   teamID,
		"team_name": "team-default",
		"scope":     getScope(r),
	})
}

// Health check handler
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Status handler
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "operational",
		"version": "0.1.0",
	})
}

// API Key handlers

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleCreateAPIKey(w, r)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleListAPIKeys(w, r)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	s.authHandler.HandleRevokeAPIKey(w, r)
}

type CreateEnvironmentRequest struct {
	Name    string `json:"name"`
	Project string `json:"project"`
	Format  string `json:"format"`
	Source  string `json:"source"`
}

type CreateEnvironmentResponse struct {
	Environment *store.Environment `json:"environment"`
	Machine     *store.Machine     `json:"machine"`
}

// ensure store types are imported
var _ = &store.Environment{}
var _ = &store.Machine{}

var subnetCounter uint32 = 0

func allocateSubnet() string {
	counter := atomic.AddUint32(&subnetCounter, 1)
	subnetNum := (counter % 254) + 1
	return fmt.Sprintf("10.224.%d.0/24", subnetNum)
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	if req.Name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
		return
	}
	if req.Project == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "project is required",
		})
		return
	}
	if req.Format == "" {
		req.Format = "docker-compose"
	}

	teamID := getTeamID(r)
	if teamID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unable to identify team",
		})
		return
	}

	createdBy := getKeyID(r)

	machines, err := s.store.ListMachinesByStatus(ctx, "ready")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to list machines: " + err.Error(),
		})
		return
	}

	var availableMachine *store.Machine
	for _, m := range machines {
		if m.AssignedEnv == nil {
			availableMachine = m
			break
		}
	}

	if availableMachine == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no machines available",
		})
		return
	}

	subnet := allocateSubnet()
	envID := uuid.New().String()

	env := &store.Environment{
		ID:        envID,
		TeamID:    teamID,
		Name:      req.Name,
		Project:   req.Project,
		Format:    req.Format,
		MachineID: &availableMachine.ID,
		Status:    "provisioning",
		Subnet:    subnet,
		Services:  []store.EnvironmentService{},
		Peers:     []store.EnvironmentPeer{},
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		Source:    req.Source,
	}

	if err := s.store.CreateEnvironment(ctx, env); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create environment: " + err.Error(),
		})
		return
	}

	if err := s.store.AssignMachine(ctx, availableMachine.ID, envID); err != nil {
		_ = s.store.DeleteEnvironment(ctx, envID)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to assign machine: " + err.Error(),
		})
		return
	}

	if err := s.store.UpdateMachineStatus(ctx, availableMachine.ID, "assigned"); err != nil {
		fmt.Printf("Warning: failed to update machine status: %v\n", err)
	}

	go s.provisionEnvironment(env, availableMachine)

	resp := CreateEnvironmentResponse{
		Environment: &store.Environment{
			ID:        envID,
			Name:      req.Name,
			Project:   req.Project,
			Status:    "provisioning",
			Subnet:    subnet,
			MachineID: &availableMachine.ID,
		},
		Machine: &store.Machine{
			ID:       availableMachine.ID,
			PublicIP: availableMachine.PublicIP,
			Status:   "assigned",
		},
	}
	respondJSON(w, http.StatusAccepted, resp)
}

func (s *Server) provisionEnvironment(env *store.Environment, machine *store.Machine) {
	ctx := context.Background()

	agentAddr := fmt.Sprintf("http://%s:8080", machine.WGEndpoint)
	agentClient := agent.NewClient(agentAddr)

	upReq := agent.UpRequest{
		WorkspacePath: fmt.Sprintf("/var/lib/cilo/envs/%s", env.ID),
		EnvName:       env.Name,
		Subnet:        env.Subnet,
		Build:         true,
		Recreate:      false,
	}

	upResp, err := agentClient.EnvironmentUp(ctx, upReq)
	if err != nil {
		if updateErr := s.store.UpdateEnvironmentStatus(ctx, env.ID, "error"); updateErr != nil {
			fmt.Printf("Error: failed to update environment status to error: %v\n", updateErr)
		}
		return
	}

	services := make([]store.EnvironmentService, 0, len(upResp.Services))
	for name, ip := range upResp.Services {
		services = append(services, store.EnvironmentService{
			Name: name,
			IP:   ip,
			Port: 80,
		})
	}

	if err := s.store.UpdateEnvironmentServices(ctx, env.ID, services); err != nil {
		fmt.Printf("Warning: failed to update environment services: %v\n", err)
	}
	if err := s.store.UpdateEnvironmentStatus(ctx, env.ID, "ready"); err != nil {
		fmt.Printf("Error: failed to update environment status to ready: %v\n", err)
	}
}

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "not yet implemented",
	})
}

func (s *Server) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "not yet implemented",
	})
}

func (s *Server) handleDestroyEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	envID := chi.URLParam(r, "envID")

	if envID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "environment ID is required",
		})
		return
	}

	env, err := s.store.GetEnvironment(ctx, envID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "environment not found",
		})
		return
	}

	if env.MachineID != nil && *env.MachineID != "" {
		if err := s.store.AssignMachine(ctx, *env.MachineID, ""); err != nil {
			fmt.Printf("Warning: failed to unassign machine: %v\n", err)
		}
		if err := s.store.UpdateMachineStatus(ctx, *env.MachineID, "ready"); err != nil {
			fmt.Printf("Warning: failed to update machine status: %v\n", err)
		}
	}

	if err := s.store.DeleteEnvironment(ctx, envID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete environment: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "environment destroyed",
	})
}

func (s *Server) handleSyncEnvironment(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "not yet implemented",
	})
}

// WireGuard handlers

func (s *Server) handleWireGuardExchange(w http.ResponseWriter, r *http.Request) {
	s.wgHandler.HandleWireGuardExchange(w, r)
}

func (s *Server) handleRemovePeer(w http.ResponseWriter, r *http.Request) {
	s.wgHandler.HandleWireGuardRemovePeer(w, r)
}

func (s *Server) handleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
	s.wgHandler.HandleWireGuardStatus(w, r)
}

func (s *Server) handleRegisterMachine(w http.ResponseWriter, r *http.Request) {
	var req RegisterMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" || req.Host == "" || req.SSHUser == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name, host, and ssh_user are required"})
		return
	}

	machine := &store.Machine{
		ID:           fmt.Sprintf("manual-%s", req.Name),
		ProviderID:   req.Host,
		ProviderType: "manual",
		PublicIP:     req.Host,
		Status:       "ready",
		SSHHost:      req.Host,
		SSHUser:      req.SSHUser,
		Region:       req.Region,
		Size:         req.Size,
		CreatedAt:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.store.SaveMachine(ctx, machine); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save machine"})
		return
	}

	resp := MachineResponse{
		ID:           machine.ID,
		Name:         req.Name,
		Host:         machine.SSHHost,
		Status:       machine.Status,
		AssignedEnv:  machine.AssignedEnv,
		ProviderType: machine.ProviderType,
		CreatedAt:    machine.CreatedAt.Format(time.RFC3339),
	}

	respondJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	machines, err := s.store.ListMachines(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list machines"})
		return
	}

	var resp []MachineResponse
	for _, m := range machines {
		name := m.ID
		if len(m.ID) > 7 && m.ID[:7] == "manual-" {
			name = m.ID[7:]
		}
		resp = append(resp, MachineResponse{
			ID:           m.ID,
			Name:         name,
			Host:         m.SSHHost,
			Status:       m.Status,
			AssignedEnv:  m.AssignedEnv,
			ProviderType: m.ProviderType,
			CreatedAt:    m.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRemoveMachine(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "machineID")
	if machineID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "machine ID is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	machine, err := s.store.GetMachine(ctx, machineID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "machine not found"})
		return
	}

	if machine.AssignedEnv != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "machine is assigned to environment"})
		return
	}

	if err := s.store.DeleteMachine(ctx, machineID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete machine"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "machine removed successfully"})
}
