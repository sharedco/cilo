package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides HTTP client for communicating with cilo-agent on worker machines.
// The agent runs on WireGuard IP (e.g., 10.225.0.100:8080) and server connects
// through the WireGuard tunnel.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new agent client.
// agentAddr should be the full address including protocol, e.g., "http://10.225.0.100:8080"
func NewClient(agentAddr string) *Client {
	return &Client{
		baseURL: agentAddr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UpRequest is the request body for POST /environment/up
type UpRequest struct {
	WorkspacePath string `json:"workspace_path"`
	EnvName       string `json:"env_name"`
	Subnet        string `json:"subnet"`
	Build         bool   `json:"build,omitempty"`
	Recreate      bool   `json:"recreate,omitempty"`
}

// UpResponse is the response for POST /environment/up
type UpResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"` // service name -> IP
}

// AddPeerRequest is the request body for POST /wireguard/add-peer
type AddPeerRequest struct {
	PublicKey  string `json:"public_key"`
	AllowedIPs string `json:"allowed_ips"`
}

// Error represents an error response from the agent
type Error struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("agent error (status %d): %s", e.StatusCode, e.Message)
}

// EnvironmentUp starts an environment on the agent.
// POST /environment/up
func (c *Client) EnvironmentUp(ctx context.Context, req UpRequest) (*UpResponse, error) {
	url := c.baseURL + "/environment/up"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var upResp UpResponse
	if err := json.NewDecoder(resp.Body).Decode(&upResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &upResp, nil
}

// EnvironmentDown stops an environment on the agent.
// POST /environment/down
func (c *Client) EnvironmentDown(ctx context.Context, envName string) error {
	url := c.baseURL + "/environment/down"

	reqBody := map[string]string{"env_name": envName}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// EnvironmentStatus gets the status of an environment from the agent.
// GET /environment/status?env_name={envName}
// Returns a map of service names to their status information.
func (c *Client) EnvironmentStatus(ctx context.Context, envName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/environment/status?env_name=%s", c.baseURL, envName)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var statuses map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return statuses, nil
}

// AddPeer adds a WireGuard peer to the agent.
// POST /wireguard/add-peer
func (c *Client) AddPeer(ctx context.Context, req AddPeerRequest) error {
	url := c.baseURL + "/wireguard/add-peer"

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// RemovePeer removes a WireGuard peer from the agent.
// DELETE /wireguard/remove-peer/{key}
func (c *Client) RemovePeer(ctx context.Context, publicKey string) error {
	url := fmt.Sprintf("%s/wireguard/remove-peer/%s", c.baseURL, publicKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// Health checks the health status of the agent.
// GET /health
func (c *Client) Health(ctx context.Context) error {
	url := c.baseURL + "/health"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// parseError parses an error response from the agent.
func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp Error
	errResp.StatusCode = resp.StatusCode

	if len(body) > 0 {
		// Try to parse as JSON error
		if err := json.Unmarshal(body, &errResp); err != nil {
			// If JSON parsing fails, use the raw body as the message
			errResp.Message = string(body)
		}
	} else {
		errResp.Message = http.StatusText(resp.StatusCode)
	}

	return &errResp
}
