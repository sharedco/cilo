// Package cloud provides the client for communicating with Cilo Cloud servers.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the Cilo Cloud API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new cloud client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientFromAuth creates a client from stored auth
func NewClientFromAuth() (*Client, error) {
	auth, err := LoadAuth()
	if err != nil {
		return nil, fmt.Errorf("load auth: %w (run 'cilo cloud login' first)", err)
	}
	return NewClient(auth.Server, auth.APIKey), nil
}

// Environment represents a remote environment
type Environment struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Project   string        `json:"project"`
	Format    string        `json:"format"`
	Status    string        `json:"status"`
	Subnet    string        `json:"subnet"`
	MachineID string        `json:"machine_id"`
	Services  []ServiceInfo `json:"services"`
	Peers     []PeerInfo    `json:"peers"`
	CreatedAt string        `json:"created_at"`
}

// ServiceInfo represents a service in an environment
type ServiceInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// PeerInfo represents a connected peer
type PeerInfo struct {
	UserID      string `json:"user_id"`
	WGPublicKey string `json:"wg_public_key"`
	AssignedIP  string `json:"assigned_ip"`
	ConnectedAt string `json:"connected_at"`
}

// Machine represents a VM
type Machine struct {
	ID          string `json:"id"`
	PublicIP    string `json:"public_ip"`
	WGPublicKey string `json:"wg_public_key"`
	WGEndpoint  string `json:"wg_endpoint"`
	Status      string `json:"status"`
}

// CreateEnvironmentRequest is the request for creating an environment
type CreateEnvironmentRequest struct {
	Name      string `json:"name"`
	Project   string `json:"project"`
	Format    string `json:"format"`
	Source    string `json:"source"`               // "cli", "ci", "api"
	CIMode    bool   `json:"ci_mode"`              // CI mode: skip WireGuard, use direct IP
	CITimeout int    `json:"ci_timeout,omitempty"` // Auto-destroy after N minutes (CI mode only)
}

// CreateEnvironmentResponse is the response from creating an environment
type CreateEnvironmentResponse struct {
	Environment *Environment `json:"environment"`
	Machine     *Machine     `json:"machine"`
	PublicIP    string       `json:"public_ip,omitempty"`
	ExpiresAt   string       `json:"expires_at,omitempty"`
}

// WireGuardExchangeRequest is the request for WireGuard key exchange
type WireGuardExchangeRequest struct {
	EnvironmentID string `json:"environment_id"`
	ClientPubKey  string `json:"client_public_key"`
}

// WireGuardExchangeResponse is the response from key exchange
type WireGuardExchangeResponse struct {
	AssignedIP     string `json:"assigned_ip"`
	ServerPubKey   string `json:"server_public_key"`
	ServerEndpoint string `json:"server_endpoint"`
	AllowedIPs     string `json:"allowed_ips"`
}

// CreateEnvironment creates a new remote environment
func (c *Client) CreateEnvironment(ctx context.Context, req CreateEnvironmentRequest) (*CreateEnvironmentResponse, error) {
	var resp CreateEnvironmentResponse
	err := c.post(ctx, "/v1/environments", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEnvironment retrieves an environment by ID
func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var env Environment
	err := c.get(ctx, fmt.Sprintf("/v1/environments/%s", id), &env)
	if err != nil {
		return nil, err
	}
	return &env, nil
}

// GetEnvironmentByName retrieves an environment by name
func (c *Client) GetEnvironmentByName(ctx context.Context, name string) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if env.Name == name {
			return env, nil
		}
	}
	return nil, fmt.Errorf("environment %q not found", name)
}

// ListEnvironments lists all environments
func (c *Client) ListEnvironments(ctx context.Context) ([]*Environment, error) {
	var resp struct {
		Environments []*Environment `json:"environments"`
	}
	err := c.get(ctx, "/v1/environments", &resp)
	if err != nil {
		return nil, err
	}
	return resp.Environments, nil
}

// StopEnvironment stops an environment without destroying it
func (c *Client) StopEnvironment(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/v1/environments/%s/down", id), nil, nil)
}

// DestroyEnvironment destroys an environment
func (c *Client) DestroyEnvironment(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/environments/%s", id))
}

// SyncComplete signals that workspace sync is complete
func (c *Client) SyncComplete(ctx context.Context, envID string) error {
	return c.post(ctx, fmt.Sprintf("/v1/environments/%s/sync", envID), nil, nil)
}

// WireGuardExchange performs WireGuard key exchange
func (c *Client) WireGuardExchange(ctx context.Context, req WireGuardExchangeRequest) (*WireGuardExchangeResponse, error) {
	var resp WireGuardExchangeResponse
	err := c.post(ctx, "/v1/wireguard/exchange", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemovePeer removes a WireGuard peer
func (c *Client) RemovePeer(ctx context.Context, publicKey string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/wireguard/peers/%s", publicKey))
}

// AuthResponse contains authentication validation response
type AuthResponse struct {
	TeamID   string `json:"team_id,omitempty"`
	TeamName string `json:"team_name,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	Valid    bool   `json:"valid"`
}

// ValidateAuth validates the API key with the server
// Returns team information if the key is valid
func (c *Client) ValidateAuth(ctx context.Context) (*AuthResponse, error) {
	var resp AuthResponse
	err := c.get(ctx, "/v1/auth/validate", &resp)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}
	return &resp, nil
}

// HTTP helpers

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, result)
}

func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "cilo-cli/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("server error: %s", errResp.Error)
		}
		return fmt.Errorf("server error: %s", resp.Status)
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
