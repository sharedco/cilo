// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to create SSH signer: %v", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create SSH public key: %v", err)
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(sshPublicKey))

	// Create auth handler with stub verifier
	authHandler := NewAuthHandler(&stubSSHVerifier{authorizedKey: sshPublicKey}, "/tmp/peers.json")

	// Setup router with auth middleware
	r := chi.NewRouter()
	r.Post("/auth/challenge", authHandler.HandleChallenge)
	r.Post("/auth/connect", authHandler.HandleConnect)
	r.With(authHandler.AuthMiddleware).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	t.Run("valid SSH signature is accepted", func(t *testing.T) {
		// Request challenge from server
		chReqBody, _ := json.Marshal(ChallengeRequest{PublicKey: publicKeyStr})
		chReq := httptest.NewRequest(http.MethodPost, "/auth/challenge", bytes.NewReader(chReqBody))
		chReq.Header.Set("Content-Type", "application/json")
		chRR := httptest.NewRecorder()
		r.ServeHTTP(chRR, chReq)
		if chRR.Code != http.StatusOK {
			t.Fatalf("Expected challenge status 200, got %d: %s", chRR.Code, chRR.Body.String())
		}
		var chResp ChallengeResponse
		if err := json.Unmarshal(chRR.Body.Bytes(), &chResp); err != nil {
			t.Fatalf("Failed to parse challenge response: %v", err)
		}

		sig, err := signer.Sign(rand.Reader, []byte(chResp.Challenge))
		if err != nil {
			t.Fatalf("Failed to sign challenge: %v", err)
		}

		// Build connect request
		reqBody := ConnectRequest{
			PublicKey:       publicKeyStr,
			Challenge:       chResp.Challenge,
			Signature:       base64.StdEncoding.EncodeToString(sig.Blob),
			SignatureFormat: sig.Format,
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
		// Request challenge from server
		chReqBody, _ := json.Marshal(ChallengeRequest{PublicKey: publicKeyStr})
		chReq := httptest.NewRequest(http.MethodPost, "/auth/challenge", bytes.NewReader(chReqBody))
		chReq.Header.Set("Content-Type", "application/json")
		chRR := httptest.NewRecorder()
		r.ServeHTTP(chRR, chReq)
		if chRR.Code != http.StatusOK {
			t.Fatalf("Expected challenge status 200, got %d: %s", chRR.Code, chRR.Body.String())
		}
		var chResp ChallengeResponse
		if err := json.Unmarshal(chRR.Body.Bytes(), &chResp); err != nil {
			t.Fatalf("Failed to parse challenge response: %v", err)
		}

		// Generate a different key to create invalid signature
		wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		wrongSigner, _ := ssh.NewSignerFromKey(wrongKey)
		wrongSig, _ := wrongSigner.Sign(rand.Reader, []byte(chResp.Challenge))

		reqBody := ConnectRequest{
			PublicKey:       publicKeyStr,
			Challenge:       chResp.Challenge,
			Signature:       base64.StdEncoding.EncodeToString(wrongSig.Blob),
			SignatureFormat: wrongSig.Format,
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
		authHandler.mu.Lock()
		authHandler.sessions[expiredToken] = &Session{
			Token:     expiredToken,
			PublicKey: publicKeyStr,
			ExpiresAt: time.Now().Add(-time.Hour), // Expired 1 hour ago
		}
		authHandler.mu.Unlock()

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

// stubSSHVerifier is a stub implementation for testing
// In RED state, this returns hardcoded values
// In GREEN state, this will perform actual SSH signature verification
type stubSSHVerifier struct {
	authorizedKey ssh.PublicKey
}

func (s *stubSSHVerifier) Verify(publicKey string, challenge string, signature string, signatureFormat string) error {
	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	sigAlgo := parsedKey.Type()
	if signatureFormat != "" {
		sigAlgo = signatureFormat
	}
	sig := &ssh.Signature{
		Format: sigAlgo,
		Blob:   sigBytes,
	}

	return parsedKey.Verify([]byte(challenge), sig)
}

func (s *stubSSHVerifier) GenerateChallenge() (string, error) {
	return generateTestChallenge(), nil
}
