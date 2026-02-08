package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sharedco/cilo/server/pkg/auth"
)

// Context keys for request-scoped values
type contextKey string

const (
	contextKeyTeamID contextKey = "team_id"
	contextKeyKeyID  contextKey = "key_id"
	contextKeyScope  contextKey = "scope"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Missing Authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid Authorization header format. Expected: Bearer <token>",
			})
			return
		}

		token := parts[1]

		if !auth.IsValidKeyFormat(token) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid API key format",
			})
			return
		}

		prefix := auth.ExtractPrefix(token)
		apiKey, err := s.authStore.GetAPIKeyByPrefix(r.Context(), prefix)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid API key",
			})
			return
		}

		if !auth.ValidateAPIKey(token, apiKey.KeyHash) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid API key",
			})
			return
		}

		go s.authStore.UpdateLastUsed(context.Background(), apiKey.ID)

		ctx := context.WithValue(r.Context(), contextKeyTeamID, apiKey.TeamID)
		ctx = context.WithValue(ctx, contextKeyKeyID, apiKey.ID)
		ctx = context.WithValue(ctx, contextKeyScope, apiKey.Scope)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireScope(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope := getScope(r)

			if scope == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			if scope != requiredScope {
				respondJSON(w, http.StatusForbidden, map[string]string{
					"error": "Insufficient permissions. Required scope: " + requiredScope,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions to extract values from context

func getTeamID(r *http.Request) string {
	if teamID, ok := r.Context().Value(contextKeyTeamID).(string); ok {
		return teamID
	}
	return ""
}

func getKeyID(r *http.Request) string {
	if keyID, ok := r.Context().Value(contextKeyKeyID).(string); ok {
		return keyID
	}
	return ""
}

func getScope(r *http.Request) string {
	if scope, ok := r.Context().Value(contextKeyScope).(string); ok {
		return scope
	}
	return ""
}

// withTeamID adds team_id to request context
func withTeamID(ctx context.Context, teamID string) context.Context {
	return context.WithValue(ctx, contextKeyTeamID, teamID)
}

// withKeyID adds key_id to request context
func withKeyID(ctx context.Context, keyID string) context.Context {
	return context.WithValue(ctx, contextKeyKeyID, keyID)
}

// withScope adds scope to request context
func withScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, contextKeyScope, scope)
}
