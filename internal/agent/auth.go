// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHAuthVerifier defines the interface for SSH key authentication
// Implementations verify SSH signatures against public keys
type SSHAuthVerifier interface {
	// Verify checks if the signature is valid for the given public key and challenge
	Verify(publicKey string, challenge string, signature string, signatureFormat string) error
	// GenerateChallenge creates a new random challenge for authentication
	GenerateChallenge() (string, error)
}

// Session represents an authenticated session with a bearer token
type Session struct {
	Token     string    `json:"token"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired returns true if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// AuthHandler handles SSH key authentication and session management
type AuthHandler struct {
	mu          sync.Mutex
	verifier    SSHAuthVerifier
	sessions    map[string]*Session
	challenges  map[string]challengeRecord
	tokenExpiry time.Duration
	peersFile   string
	peerSubnet  string
}

type challengeRecord struct {
	PublicKey string
	ExpiresAt time.Time
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(verifier SSHAuthVerifier, peersFile string) *AuthHandler {
	return &AuthHandler{
		verifier:    verifier,
		sessions:    make(map[string]*Session),
		challenges:  make(map[string]challengeRecord),
		tokenExpiry: 24 * time.Hour,
		peersFile:   peersFile,
		peerSubnet:  "10.225.0.0/24",
	}
}

// ChallengeRequest requests a new authentication challenge.
type ChallengeRequest struct {
	PublicKey string `json:"public_key"`
}

// ConnectRequest is the request body for POST /auth/connect
// Client sends their SSH public key, a challenge, and the signed challenge
type ConnectRequest struct {
	PublicKey string `json:"public_key"` // SSH authorized_key format
	Challenge string `json:"challenge"`  // Random nonce
	Signature string `json:"signature"`  // Base64-encoded SSH signature
	// SignatureFormat is the ssh.Signature.Format returned by the signer.
	SignatureFormat string `json:"signature_format,omitempty"`
}

// ConnectResponse is returned after successful authentication
type ConnectResponse struct {
	Token     string    `json:"token"`      // Bearer token for subsequent requests
	ExpiresAt time.Time `json:"expires_at"` // Token expiration time
}

// HandleConnect handles POST /auth/connect
// Performs SSH challenge-response authentication and issues a session token
func (h *AuthHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.PublicKey == "" || req.Challenge == "" || req.Signature == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	// Validate challenge was issued by this agent and is unexpired.
	h.mu.Lock()
	rec, ok := h.challenges[req.Challenge]
	if ok {
		delete(h.challenges, req.Challenge) // single-use
	}
	h.mu.Unlock()

	if !ok || time.Now().After(rec.ExpiresAt) || rec.PublicKey != req.PublicKey {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "invalid or expired challenge"})
		return
	}

	// Verify the SSH signature
	if err := h.verifier.Verify(req.PublicKey, req.Challenge, req.Signature, req.SignatureFormat); err != nil {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "invalid signature"})
		return
	}

	// Generate session token
	token := generateToken()
	session := &Session{
		Token:     token,
		PublicKey: req.PublicKey,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(h.tokenExpiry),
	}

	h.mu.Lock()
	h.sessions[token] = session
	h.mu.Unlock()

	respondJSON(w, http.StatusOK, ConnectResponse{
		Token:     token,
		ExpiresAt: session.ExpiresAt,
	})
}

// HandleChallenge handles POST /auth/challenge.
// Issues a short-lived, single-use challenge that must be signed by the client.
func (h *AuthHandler) HandleChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.PublicKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "public_key is required"})
		return
	}

	challenge, err := h.verifier.GenerateChallenge()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate challenge"})
		return
	}

	expiresAt := time.Now().Add(5 * time.Minute)

	h.mu.Lock()
	h.challenges[challenge] = challengeRecord{PublicKey: req.PublicKey, ExpiresAt: expiresAt}
	h.mu.Unlock()

	respondJSON(w, http.StatusOK, ChallengeResponse{Challenge: challenge, ExpiresAt: expiresAt})
}

// HandleDisconnect handles DELETE /auth/disconnect
// Invalidates the current session token
func (h *AuthHandler) HandleDisconnect(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token"})
		return
	}

	h.mu.Lock()
	delete(h.sessions, token)
	h.mu.Unlock()
	respondJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// AuthMiddleware validates the bearer token on protected routes
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			return
		}

		if shouldTrustTailnet() && strings.HasPrefix(token, "tailscale-") && isTailnetRemote(r.RemoteAddr) {
			// For direct connections over a Tailnet, we treat the network as the auth boundary.
			// This enables seamless "cilo connect" when Tailscale is already providing access control.
			session := &Session{Token: token, PublicKey: "tailscale", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(365 * 24 * time.Hour)}
			ctx := context.WithValue(r.Context(), "session", session)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		h.mu.Lock()
		session, exists := h.sessions[token]
		h.mu.Unlock()
		if !exists || session.IsExpired() {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		// Store session in context for handlers
		ctx := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken extracts the token from the Authorization header
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

// generateToken creates a random session token
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// DefaultSSHVerifier is the production SSH signature verifier
// In the GREEN phase, this will implement actual SSH signature verification
type DefaultSSHVerifier struct {
	authorizedKeys map[string]ssh.PublicKey
}

// NewDefaultSSHVerifier creates a new SSH verifier
func NewDefaultSSHVerifier() *DefaultSSHVerifier {
	return &DefaultSSHVerifier{
		authorizedKeys: make(map[string]ssh.PublicKey),
	}
}

// AddAuthorizedKey adds a public key to the authorized keys list
func (v *DefaultSSHVerifier) AddAuthorizedKey(publicKey string) error {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	keyStr := string(ssh.MarshalAuthorizedKey(key))
	v.authorizedKeys[keyStr] = key
	return nil
}

// Verify implements SSH signature verification using golang.org/x/crypto/ssh
func (v *DefaultSSHVerifier) Verify(publicKey string, challenge string, signature string, signatureFormat string) error {
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

func shouldTrustTailnet() bool {
	val := os.Getenv("CILO_AGENT_TRUST_TAILSCALE")
	if val == "" {
		return true
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return true
	}
	return b
}

var tailscaleCGNAT = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isTailnetRemote(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return tailscaleCGNAT.Contains(ip4)
}

// GenerateChallenge creates a random challenge for authentication
func (v *DefaultSSHVerifier) GenerateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// PeerAllocation manages IP allocation for WireGuard peers
// Each cilod instance manages its own /24 subnet independently
// IP allocation is stored in /var/cilo/peers.json as a simple JSON mapping:
//
//	{ "peer_pubkey_1": "10.225.0.2", "peer_pubkey_2": "10.225.0.3", ... }
type PeerAllocation struct {
	PublicKey string `json:"public_key"`
	IP        string `json:"ip"`
}

// PeerStore defines the interface for peer IP allocation storage
type PeerStore interface {
	// GetPeerIP returns the assigned IP for a peer, or "" if not allocated
	GetPeerIP(publicKey string) (string, error)
	// AllocatePeerIP assigns the next available IP to a peer
	AllocatePeerIP(publicKey string) (string, error)
	// RemovePeer removes a peer from the allocation
	RemovePeer(publicKey string) error
	// ListPeers returns all peer allocations
	ListPeers() ([]PeerAllocation, error)
}

// ChallengeResponse defines the challenge-response protocol for SSH auth
type ChallengeResponse struct {
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired returns true if the challenge has expired
func (c *ChallengeResponse) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
