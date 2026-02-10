// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"
)

// TestSSHKeyAuth tests the SSH key authentication protocol
// This test is in RED state - implementation is stub only
func TestSSHKeyAuth(t *testing.T) {
	// Generate a mock SSH key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create SSH public key: %v", err)
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(sshPublicKey))

	// Create auth handler with stub verifier
	authHandler := &AuthHandler{
		verifier: &stubSSHVerifier{
			authorizedKey: sshPublicKey,
		},
		sessions: make(map[string]*Session),
	}

	// Setup router with auth middleware
	r := chi.NewRouter()
	r.Post("/auth/connect", authHandler.HandleConnect)
	r.With(authHandler.AuthMiddleware).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	t.Run("valid SSH signature is accepted", func(t *testing.T) {
		// Create a challenge
		challenge := generateTestChallenge()

		// Sign the challenge with our private key
		signature, err := signChallenge(privateKey, challenge)
		if err != nil {
			t.Fatalf("Failed to sign challenge: %v", err)
		}

		// Build connect request
		reqBody := ConnectRequest{
			PublicKey: publicKeyStr,
			Challenge: challenge,
			Signature: signature,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/auth/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp ConnectResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if resp.Token == "" {
			t.Error("Expected token in response, got empty string")
		}

		if resp.ExpiresAt.IsZero() {
			t.Error("Expected ExpiresAt to be set")
		}
	})

	t.Run("request without auth is rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rr.Code)
		}

		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["error"] != "missing authorization header" {
			t.Errorf("Expected 'missing authorization header' error, got %q", resp["error"])
		}
	})

	t.Run("invalid signature is rejected with 403", func(t *testing.T) {
		// Create a challenge
		challenge := generateTestChallenge()

		// Generate a different key to create invalid signature
		wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		wrongSignature, _ := signChallenge(wrongKey, challenge)

		reqBody := ConnectRequest{
			PublicKey: publicKeyStr,
			Challenge: challenge,
			Signature: wrongSignature,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/auth/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", rr.Code)
		}

		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["error"] != "invalid signature" {
			t.Errorf("Expected 'invalid signature' error, got %q", resp["error"])
		}
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		// Create an expired token manually
		expiredToken := "expired_test_token"
		authHandler.sessions[expiredToken] = &Session{
			Token:     expiredToken,
			PublicKey: publicKeyStr,
			ExpiresAt: time.Now().Add(-time.Hour), // Expired 1 hour ago
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for expired token, got %d", rr.Code)
		}
	})
}

// generateTestChallenge creates a test challenge string
func generateTestChallenge() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// signChallenge signs a challenge with an RSA private key
func signChallenge(privateKey *rsa.PrivateKey, challenge string) (string, error) {
	hash := sha256.Sum256([]byte(challenge))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, 0, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// stubSSHVerifier is a stub implementation for testing
// In RED state, this returns hardcoded values
// In GREEN state, this will perform actual SSH signature verification
type stubSSHVerifier struct {
	authorizedKey ssh.PublicKey
}

func (s *stubSSHVerifier) Verify(publicKey string, challenge string, signature string) error {
	// RED state: stub always returns nil for authorized key
	// GREEN state: will implement actual SSH signature verification
	return nil
}

func (s *stubSSHVerifier) GenerateChallenge() (string, error) {
	return generateTestChallenge(), nil
}
