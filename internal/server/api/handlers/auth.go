// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sharedco/cilo/internal/server/auth"
)

type AuthHandler struct {
	authStore *auth.Store
}

func NewAuthHandler(authStore *auth.Store) *AuthHandler {
	return &AuthHandler{authStore: authStore}
}

type CreateAPIKeyRequest struct {
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
	Scope  string `json:"scope"`
}

type CreateAPIKeyResponse struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Prefix  string `json:"prefix"`
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

func (h *AuthHandler) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	if req.Scope == "" {
		req.Scope = "developer"
	}
	if req.Scope != "admin" && req.Scope != "developer" && req.Scope != "ci" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid scope. Must be: admin, developer, or ci",
		})
		return
	}

	teamID := getTeamIDFromContext(r)
	if teamID == "" {
		teamID = req.TeamID
	}
	if teamID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "team_id is required",
		})
		return
	}

	key, hash, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate API key",
		})
		return
	}

	apiKey, err := h.authStore.CreateAPIKey(r.Context(), teamID, req.Name, req.Scope, hash, prefix)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to store API key",
		})
		return
	}

	respondJSON(w, http.StatusCreated, CreateAPIKeyResponse{
		ID:      apiKey.ID,
		Key:     key,
		Prefix:  prefix,
		Name:    apiKey.Name,
		Scope:   apiKey.Scope,
		Message: "Save this key securely. It will not be shown again.",
	})
}

type APIKeyListItem struct {
	ID        string  `json:"id"`
	Prefix    string  `json:"prefix"`
	Name      string  `json:"name"`
	Scope     string  `json:"scope"`
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used,omitempty"`
}

func (h *AuthHandler) HandleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	teamID := getTeamIDFromContext(r)
	if teamID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Not authenticated",
		})
		return
	}

	keys, err := h.authStore.ListAPIKeys(r.Context(), teamID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to list API keys",
		})
		return
	}

	items := make([]APIKeyListItem, len(keys))
	for i, key := range keys {
		items[i] = APIKeyListItem{
			ID:        key.ID,
			Prefix:    key.Prefix,
			Name:      key.Name,
			Scope:     key.Scope,
			CreatedAt: key.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if key.LastUsed != nil {
			t := key.LastUsed.Format("2006-01-02T15:04:05Z")
			items[i].LastUsed = &t
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys": items,
	})
}

func (h *AuthHandler) HandleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
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

	err := h.authStore.DeleteAPIKey(r.Context(), teamID, keyID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "API key not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "API key revoked",
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func getTeamIDFromContext(r *http.Request) string {
	if v := r.Context().Value("team_id"); v != nil {
		return v.(string)
	}
	return ""
}
