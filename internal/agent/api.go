// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

// Package agent provides the cilod API specification and types.
//
// API Overview:
//
// Authentication (SSH Challenge-Response):
//
//	POST   /auth/connect          — SSH key exchange, returns session token
//	DELETE /auth/disconnect        — Invalidate session
//
// Environment Management:
//
//	GET    /environments           — List all environments
//	POST   /environments/:name/up  — Create + start environment
//	POST   /environments/:name/down — Stop environment
//	DELETE /environments/:name     — Destroy environment
//	GET    /environments/:name/status — Get env status
//	GET    /environments/:name/logs  — Stream logs (WebSocket upgrade)
//	POST   /environments/:name/exec — Exec into container (WebSocket upgrade)
//
// WireGuard Peer Management:
//
//	POST   /wireguard/exchange     — WireGuard key exchange
//	DELETE /wireguard/peers/:key   — Remove peer
//	GET    /wireguard/status       — WireGuard interface status
//
// Workspace Sync:
//
//	POST   /sync/:name             — Receive workspace sync
//
// IP Allocation Strategy:
//
// Each cilod instance manages its own /24 subnet independently without a central
// coordinator. The default subnet is 10.225.0.0/24 but is configurable.
//
// Peer IP allocation is stored in a simple JSON file at /var/cilo/peers.json:
//
//	{
//	  "peers": {
//	    "peer_pubkey_1": "10.225.0.2",
//	    "peer_pubkey_2": "10.225.0.3"
//	  },
//	  "next_ip": "10.225.0.4"
//	}
//
// Allocation algorithm:
//  1. Load existing peers.json (create if missing)
//  2. If peer already has IP, return existing
//  3. Otherwise, assign next_ip and increment
//  4. Persist updated peers.json
//
// This design allows each cilod to operate independently while ensuring unique
// IPs within its own subnet. No coordination is needed between cilod instances
// as they manage disjoint IP ranges.
package agent

import (
	"context"
	"net/http"
	"time"
)

// ============================================================================
// API Handler Interface
// ============================================================================

// APIHandler defines all cilod API endpoints as an interface.
// This interface is implemented by Server and used for testing.
type APIHandler interface {
	// Auth handlers
	HandleAuthConnect(w http.ResponseWriter, r *http.Request)
	HandleAuthDisconnect(w http.ResponseWriter, r *http.Request)

	// Environment handlers
	HandleListEnvironments(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentUp(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentDown(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentDestroy(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentStatus(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentLogs(w http.ResponseWriter, r *http.Request)
	HandleEnvironmentExec(w http.ResponseWriter, r *http.Request)

	// WireGuard handlers
	HandleWireGuardExchange(w http.ResponseWriter, r *http.Request)
	HandleWireGuardRemovePeer(w http.ResponseWriter, r *http.Request)
	HandleWireGuardStatus(w http.ResponseWriter, r *http.Request)

	// Sync handlers
	HandleWorkspaceSync(w http.ResponseWriter, r *http.Request)
}

// ============================================================================
// Authentication Types
// ============================================================================

// AuthChallengeRequest requests a new authentication challenge
// Client calls this before attempting authentication
type AuthChallengeRequest struct {
	PublicKey string `json:"public_key"`
}

// AuthChallengeResponse returns a challenge to be signed
// Client must sign this challenge with their SSH private key
type AuthChallengeResponse struct {
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ============================================================================
// Environment Types
// ============================================================================

// ListEnvironmentsResponse contains all environments managed by this cilod
type ListEnvironmentsResponse struct {
	Environments []EnvironmentInfo `json:"environments"`
}

// EnvironmentInfo describes a single environment
type EnvironmentInfo struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // running, stopped, error
	CreatedAt time.Time `json:"created_at"`
	Services  []string  `json:"services"`
	Subnet    string    `json:"subnet"`
}

// EnvironmentUpRequest starts or creates an environment
// POST /environments/:name/up
type EnvironmentUpRequest struct {
	WorkspacePath string `json:"workspace_path,omitempty"` // Optional: override workspace
	Build         bool   `json:"build,omitempty"`          // Rebuild containers
	Recreate      bool   `json:"recreate,omitempty"`       // Force recreate
}

// EnvironmentUpResponse confirms environment is running
type EnvironmentUpResponse struct {
	Name     string            `json:"name"`
	Status   string            `json:"status"`
	Services map[string]string `json:"services"` // service name -> IP
	Subnet   string            `json:"subnet"`
}

// EnvironmentDownRequest stops an environment
// POST /environments/:name/down
type EnvironmentDownRequest struct {
	Force bool `json:"force,omitempty"` // Force stop even if busy
}

// EnvironmentDownResponse confirms environment is stopped
type EnvironmentDownResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// EnvironmentDestroyRequest destroys an environment permanently
// DELETE /environments/:name
type EnvironmentDestroyRequest struct {
	Force bool `json:"force,omitempty"` // Skip confirmation
}

// EnvironmentDestroyResponse confirms environment is destroyed
type EnvironmentDestroyResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// EnvironmentStatusResponse returns detailed environment status
// GET /environments/:name/status
type EnvironmentStatusResponse struct {
	Name       string                `json:"name"`
	Status     string                `json:"status"`
	Services   []ServiceStatusDetail `json:"services"`
	Networks   []NetworkInfo         `json:"networks"`
	LastActive time.Time             `json:"last_active"`
}

// ServiceStatusDetail provides detailed service information
type ServiceStatusDetail struct {
	Name   string        `json:"name"`
	State  string        `json:"state"`  // running, exited, etc.
	Status string        `json:"status"` // Up 2 hours, Exited (0), etc.
	Health string        `json:"health,omitempty"`
	IP     string        `json:"ip,omitempty"`
	Ports  []PortMapping `json:"ports,omitempty"`
}

// PortMapping describes a port mapping
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// NetworkInfo describes a Docker network
type NetworkInfo struct {
	Name    string `json:"name"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// EnvironmentLogsRequest requests logs for a service
// GET /environments/:name/logs?service=&follow=
type EnvironmentLogsRequest struct {
	Service string `json:"service"` // Query param: service name
	Follow  bool   `json:"follow"`  // Query param: stream logs
	Tail    int    `json:"tail"`    // Query param: number of lines
}

// EnvironmentExecRequest executes a command in a container
// POST /environments/:name/exec
// Upgrades to WebSocket for interactive sessions
type EnvironmentExecRequest struct {
	Service string   `json:"service"`         // Target service/container
	Command []string `json:"command"`         // Command to execute
	TTY     bool     `json:"tty,omitempty"`   // Allocate pseudo-TTY
	Stdin   bool     `json:"stdin,omitempty"` // Attach stdin
}

// ============================================================================
// WireGuard Types
// ============================================================================

// WireGuardExchangeRequest initiates peer connection
// POST /wireguard/exchange
type WireGuardExchangeRequest struct {
	PublicKey     string `json:"public_key"`     // Client's WireGuard public key
	EnvironmentID string `json:"environment_id"` // Optional: env to connect to
	UserID        string `json:"user_id"`        // Optional: for multi-user tracking
}

// WireGuardExchangeResponse provides server details for client configuration
// Client uses this to configure their WireGuard interface
type WireGuardExchangeResponse struct {
	ServerPublicKey   string   `json:"server_public_key"` // cilod's WG public key
	ServerEndpoint    string   `json:"server_endpoint"`   // cilod's WG endpoint (IP:port)
	AssignedIP        string   `json:"assigned_ip"`       // IP assigned to client in WG subnet
	AllowedIPs        []string `json:"allowed_ips"`       // Routes through tunnel
	EnvironmentSubnet string   `json:"environment_subnet,omitempty"`
}

// WireGuardRemovePeerRequest removes a peer
// DELETE /wireguard/peers/:key
type WireGuardRemovePeerRequest struct {
	PublicKey string `json:"public_key"` // URL param: peer public key
}

// WireGuardRemovePeerResponse confirms peer removal
type WireGuardRemovePeerResponse struct {
	PublicKey string `json:"public_key"`
	Status    string `json:"status"`
}

// WireGuardStatusResponse returns interface and peer status
// GET /wireguard/status
type WireGuardStatusResponse struct {
	Interface  string          `json:"interface"`
	PublicKey  string          `json:"public_key"`
	ListenPort int             `json:"listen_port"`
	Address    string          `json:"address"`
	Peers      []WireGuardPeer `json:"peers"`
}

// WireGuardPeer describes a connected peer
type WireGuardPeer struct {
	PublicKey       string `json:"public_key"`
	Endpoint        string `json:"endpoint,omitempty"`
	AllowedIPs      string `json:"allowed_ips"`
	LatestHandshake string `json:"latest_handshake,omitempty"`
	RxBytes         int64  `json:"rx_bytes"`
	TxBytes         int64  `json:"tx_bytes"`
	AssignedIP      string `json:"assigned_ip"` // cilod-assigned IP
}

// ============================================================================
// Workspace Sync Types
// ============================================================================

// WorkspaceSyncRequest receives workspace file sync
// POST /sync/:name
// Supports both full sync and incremental (rsync-style) updates
type WorkspaceSyncRequest struct {
	EnvironmentName string     `json:"environment_name"`       // URL param
	SyncType        string     `json:"sync_type"`              // "full" or "incremental"
	Files           []FileSync `json:"files"`                  // Files to sync
	DeletePaths     []string   `json:"delete_paths,omitempty"` // Paths to delete
}

// FileSync describes a single file to sync
type FileSync struct {
	Path    string `json:"path"`     // Relative path in workspace
	Content []byte `json:"content"`  // File content (base64 encoded for JSON)
	Mode    uint32 `json:"mode"`     // File permissions
	ModTime int64  `json:"mod_time"` // Unix timestamp
	Hash    string `json:"hash"`     // SHA256 hash for verification
}

// WorkspaceSyncResponse confirms sync completion
type WorkspaceSyncResponse struct {
	EnvironmentName string   `json:"environment_name"`
	FilesReceived   int      `json:"files_received"`
	FilesUpdated    int      `json:"files_updated"`
	FilesDeleted    int      `json:"files_deleted"`
	Errors          []string `json:"errors,omitempty"`
}

// ============================================================================
// IP Allocation Types
// ============================================================================

// PeerIPAllocation represents the peers.json file structure
// Stored at /var/cilo/peers.json
type PeerIPAllocation struct {
	Peers  map[string]string `json:"peers"`   // pubkey -> IP
	NextIP string            `json:"next_ip"` // Next IP to allocate
}

// IPAllocator manages IP allocation for WireGuard peers
type IPAllocator interface {
	// Allocate assigns an IP to a peer, returning existing if already allocated
	Allocate(ctx context.Context, publicKey string) (string, error)
	// Release removes a peer's IP allocation
	Release(ctx context.Context, publicKey string) error
	// Get retrieves the IP for a peer
	Get(ctx context.Context, publicKey string) (string, error)
	// List returns all allocations
	List(ctx context.Context) (map[string]string, error)
}

// ============================================================================
// WebSocket Types (for logs and exec)
// ============================================================================

// WebSocketMessage is the envelope for WebSocket communication
type WebSocketMessage struct {
	Type     string `json:"type"`                // "stdout", "stderr", "error", "exit"
	Data     []byte `json:"data"`                // Message payload
	ExitCode int    `json:"exit_code,omitempty"` // For exec exit
}

// ExecStream handles bidirectional exec I/O over WebSocket
type ExecStream interface {
	// Send sends data to the container stdin
	Send(data []byte) error
	// Recv receives data from container stdout/stderr
	Recv() (*WebSocketMessage, error)
	// Close closes the stream
	Close() error
}

// LogStream handles unidirectional log streaming over WebSocket
type LogStream interface {
	// Recv receives log lines
	Recv() ([]byte, error)
	// Close closes the stream
	Close() error
}
