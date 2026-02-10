// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cilod

import (
	"time"
)

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

// AuthConnectRequest sends the signed challenge to authenticate
type AuthConnectRequest struct {
	Challenge string `json:"challenge"`
	Signature string `json:"signature"`
	PublicKey string `json:"public_key"`
}

// AuthConnectResponse returns the session token after successful auth
type AuthConnectResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ============================================================================
// Environment Types
// ============================================================================

// Environment represents a cilod environment
type Environment struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // running, stopped, error
	CreatedAt time.Time `json:"created_at"`
	Services  []string  `json:"services"`
	Subnet    string    `json:"subnet"`
}

// ListEnvironmentsResponse contains all environments managed by this cilod
type ListEnvironmentsResponse struct {
	Environments []Environment `json:"environments"`
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

// EnvironmentStatus represents detailed environment status
type EnvironmentStatus struct {
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Services   []ServiceStatus `json:"services"`
	Networks   []NetworkInfo   `json:"networks"`
	LastActive time.Time       `json:"last_active"`
}

// ServiceStatus provides detailed service information
type ServiceStatus struct {
	Name   string `json:"name"`
	State  string `json:"state"`  // running, exited, etc.
	Status string `json:"status"` // Up 2 hours, Exited (0), etc.
	Health string `json:"health,omitempty"`
	IP     string `json:"ip,omitempty"`
}

// NetworkInfo describes a Docker network
type NetworkInfo struct {
	Name    string `json:"name"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
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

// WGConfig is the client-side WireGuard configuration
type WGConfig struct {
	ServerPublicKey   string
	ServerEndpoint    string
	AssignedIP        string
	AllowedIPs        []string
	EnvironmentSubnet string
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
// Client Options
// ============================================================================

// UpOptions provides options for creating/starting an environment
type UpOptions struct {
	WorkspacePath string
	Build         bool
	Recreate      bool
}
