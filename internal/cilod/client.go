// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cilod

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client is the cilod API client for CLI-to-cilod communication
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewClient creates a new cilod client
// host can be a hostname (e.g., "cilod.example.com") or IP with port (e.g., "192.168.1.100:8080")
// token is the session token for authenticated requests (can be empty for unauthenticated calls)
func NewClient(host string, token string) *Client {
	baseURL := host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		baseURL = "http://" + host
	}

	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

// SetTimeout sets the HTTP client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// SetRetryPolicy sets the retry policy for transient failures
func (c *Client) SetRetryPolicy(maxRetries int, delay time.Duration) {
	c.maxRetries = maxRetries
	c.retryDelay = delay
}

// SetToken sets the authentication token for subsequent requests
func (c *Client) SetToken(token string) {
	c.token = token
}

// Connect authenticates with the cilod server using SSH key challenge-response
// Returns a session token that must be used for subsequent requests
func (c *Client) Connect(sshPrivateKeyPath string) (string, error) {
	var signer ssh.Signer
	var keyParseErr error

	if sshPrivateKeyPath != "" {
		privateKeyPEM, err := os.ReadFile(sshPrivateKeyPath)
		if err != nil {
			keyParseErr = fmt.Errorf("read private key: %w", err)
		} else {
			signer, err = ssh.ParsePrivateKey(privateKeyPEM)
			if err != nil {
				keyParseErr = fmt.Errorf("parse private key: %w", err)
			}
		}
	}

	if signer == nil {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			if keyParseErr != nil {
				return "", keyParseErr
			}
			return "", fmt.Errorf("no SSH signer available (no key parsed and SSH_AUTH_SOCK is not set)")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			if keyParseErr != nil {
				return "", fmt.Errorf("%v; also failed to connect to ssh-agent: %w", keyParseErr, err)
			}
			return "", fmt.Errorf("connect to ssh-agent: %w", err)
		}
		defer conn.Close()

		ag := agent.NewClient(conn)
		signers, err := ag.Signers()
		if err != nil {
			return "", fmt.Errorf("list ssh-agent signers: %w", err)
		}
		if len(signers) == 0 {
			if keyParseErr != nil {
				return "", fmt.Errorf("%v; also ssh-agent has no keys loaded", keyParseErr)
			}
			return "", fmt.Errorf("ssh-agent has no keys loaded")
		}
		signer = signers[0]
	}

	publicKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	// Step 1: Request challenge
	challengeReq := AuthChallengeRequest{PublicKey: publicKeyStr}
	challengeResp := &AuthChallengeResponse{}
	if err := c.post(context.Background(), "/auth/challenge", challengeReq, challengeResp); err != nil {
		return "", fmt.Errorf("request challenge: %w", err)
	}

	// Step 2: Sign the challenge
	sig, err := signer.Sign(rand.Reader, []byte(challengeResp.Challenge))
	if err != nil {
		return "", fmt.Errorf("sign challenge: %w", err)
	}

	// Step 3: Send connect request with signature
	connectReq := AuthConnectRequest{
		Challenge:       challengeResp.Challenge,
		Signature:       base64.StdEncoding.EncodeToString(sig.Blob),
		SignatureFormat: sig.Format,
		PublicKey:       publicKeyStr,
	}
	connectResp := &AuthConnectResponse{}
	if err := c.post(context.Background(), "/auth/connect", connectReq, connectResp); err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}

	c.token = connectResp.Token
	return connectResp.Token, nil
}

// ListEnvironments returns all environments managed by this cilod
func (c *Client) ListEnvironments() ([]Environment, error) {
	var resp ListEnvironmentsResponse
	if err := c.get(context.Background(), "/environments", &resp); err != nil {
		return nil, err
	}
	return resp.Environments, nil
}

// UpEnvironment creates or starts an environment
func (c *Client) UpEnvironment(name string, opts UpOptions) error {
	req := EnvironmentUpRequest{
		WorkspacePath: opts.WorkspacePath,
		Build:         opts.Build,
		Recreate:      opts.Recreate,
	}
	path := fmt.Sprintf("/environments/%s/up", name)
	var resp EnvironmentUpResponse
	return c.post(context.Background(), path, req, &resp)
}

// DownEnvironment stops an environment
func (c *Client) DownEnvironment(name string) error {
	req := EnvironmentDownRequest{Force: false}
	path := fmt.Sprintf("/environments/%s/down", name)
	var resp EnvironmentDownResponse
	return c.post(context.Background(), path, req, &resp)
}

// DestroyEnvironment permanently destroys an environment
func (c *Client) DestroyEnvironment(name string) error {
	path := fmt.Sprintf("/environments/%s", name)
	return c.delete(context.Background(), path)
}

// GetStatus returns detailed status for an environment
func (c *Client) GetStatus(name string) (*EnvironmentStatus, error) {
	path := fmt.Sprintf("/environments/%s/status", name)
	var status EnvironmentStatus
	if err := c.get(context.Background(), path, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// StreamLogs returns a reader for streaming logs from a service
// The caller must close the returned ReadCloser when done
func (c *Client) StreamLogs(name string, service string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/environments/%s/logs?service=%s", name, service)
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseErrorResponse(resp.StatusCode, body)
	}

	return resp.Body, nil
}

// Exec executes a command in a container
// This is a stub - full WebSocket implementation in Task 11
func (c *Client) Exec(name string, service string, cmd []string) error {
	req := EnvironmentExecRequest{
		Service: service,
		Command: cmd,
		TTY:     false,
		Stdin:   false,
	}
	path := fmt.Sprintf("/environments/%s/exec", name)
	return c.post(context.Background(), path, req, nil)
}

// WireGuardExchange performs WireGuard key exchange with the cilod server
func (c *Client) WireGuardExchange(publicKey string) (*WGConfig, error) {
	req := WireGuardExchangeRequest{PublicKey: publicKey}
	var resp WireGuardExchangeResponse

	if err := c.post(context.Background(), "/wireguard/exchange", req, &resp); err != nil {
		return nil, err
	}

	return &WGConfig{
		ServerPublicKey:   resp.ServerPublicKey,
		ServerEndpoint:    resp.ServerEndpoint,
		AssignedIP:        resp.AssignedIP,
		AllowedIPs:        resp.AllowedIPs,
		EnvironmentSubnet: resp.EnvironmentSubnet,
	}, nil
}

// SyncWorkspace syncs local workspace files to the cilod environment
// This is a stub - full rsync over SSH implementation later
func (c *Client) SyncWorkspace(name string, localPath string) error {
	// For now, just send a sync request to trigger server-side sync
	// Full implementation will use rsync over SSH
	req := WorkspaceSyncRequest{
		EnvironmentName: name,
		SyncType:        "full",
		Files:           []FileSync{},
	}
	path := fmt.Sprintf("/sync/%s", name)
	var resp WorkspaceSyncResponse
	return c.post(context.Background(), path, req, &resp)
}

// ============================================================================
// HTTP Helpers
// ============================================================================

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.doWithRetry("GET", path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doWithRetry("POST", path, body, result)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.doWithRetry("DELETE", path, nil, nil)
}

func (c *Client) doWithRetry(method string, path string, body interface{}, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryDelay * time.Duration(attempt))
		}

		err := c.doRequest(method, path, body, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on client errors (4xx)
		if isClientError(err) {
			return err
		}

		// Continue to retry on server errors (5xx) and network errors
	}

	return fmt.Errorf("after %d attempts: %w", c.maxRetries, lastErr)
}

func (c *Client) doRequest(method string, path string, body interface{}, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.httpClient.Timeout)
	defer cancel()

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.setAuthHeader(req)
	req.Header.Set("User-Agent", "cilo-cli/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp.StatusCode, bodyBytes)
	}

	if result != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) setAuthHeader(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func parseErrorResponse(statusCode int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("%s", errResp.Error)
	}
	return fmt.Errorf("%d %s", statusCode, http.StatusText(statusCode))
}

func isClientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for 4xx status codes in error message
	for code := 400; code < 500; code++ {
		if strings.Contains(errStr, fmt.Sprintf("%d", code)) {
			return true
		}
	}
	return false
}

// EnsureHostPrefix ensures the host has http:// or https:// prefix
func EnsureHostPrefix(host string) string {
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "http://" + host
}

// DefaultCilodPort is the default port for cilod HTTP API
const DefaultCilodPort = 8080

// ResolveCilodHost resolves a cilod host to a full URL
// If host is just an IP or hostname without port, adds default port
func ResolveCilodHost(host string) string {
	host = EnsureHostPrefix(host)

	// Check if port is already specified
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		if len(parts) >= 3 { // has scheme and port
			return host
		}
		// Check if it's just scheme://host without port
		if len(parts) == 2 {
			// Check if second part is a port number or part of host
			if _, err := fmt.Sscanf(parts[1], "%d", new(int)); err == nil {
				return host
			}
		}
	}

	// Add default port
	return host + fmt.Sprintf(":%d", DefaultCilodPort)
}
