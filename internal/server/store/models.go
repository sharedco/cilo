// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package store

import (
	"time"
)

// Team represents a customer organization
type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey represents an authentication token for a team
type APIKey struct {
	ID        string     `json:"id"`
	TeamID    string     `json:"team_id"`
	KeyHash   string     `json:"-"` // Never expose in JSON
	Prefix    string     `json:"prefix"`
	Scope     string     `json:"scope"` // "read", "write", "admin"
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

// Environment represents a Cilo development environment
type Environment struct {
	ID        string               `json:"id"`
	TeamID    string               `json:"team_id"`
	Name      string               `json:"name"`
	Project   string               `json:"project"`
	Format    string               `json:"format"` // "docker-compose", "devcontainer"
	MachineID *string              `json:"machine_id,omitempty"`
	Status    string               `json:"status"` // "pending", "provisioning", "ready", "error", "destroyed"
	Subnet    string               `json:"subnet"` // e.g., "10.100.1.0/24"
	Services  []EnvironmentService `json:"services,omitempty"`
	Peers     []EnvironmentPeer    `json:"peers,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
	CreatedBy string               `json:"created_by"` // User ID or API key ID
	Source    string               `json:"source"`     // Git URL or upload reference
}

// EnvironmentService represents a service within an environment (stored as JSONB)
type EnvironmentService struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// EnvironmentPeer represents a connected peer (stored as JSONB)
type EnvironmentPeer struct {
	UserID      string    `json:"user_id"`
	WGPublicKey string    `json:"wg_public_key"`
	AssignedIP  string    `json:"assigned_ip"`
	ConnectedAt time.Time `json:"connected_at"`
}

// Machine represents a VM host for environments
type Machine struct {
	ID           string    `json:"id"`
	ProviderID   string    `json:"provider_id"`   // Cloud provider's VM ID
	ProviderType string    `json:"provider_type"` // "hetzner", "digitalocean", etc.
	PublicIP     string    `json:"public_ip"`
	WGPublicKey  string    `json:"wg_public_key"`
	WGEndpoint   string    `json:"wg_endpoint"` // IP:port for WireGuard
	Status       string    `json:"status"`      // "provisioning", "ready", "error", "destroyed"
	AssignedEnv  *string   `json:"assigned_env,omitempty"`
	SSHHost      string    `json:"ssh_host"`
	SSHUser      string    `json:"ssh_user"`
	Region       string    `json:"region"`
	Size         string    `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
}

// UsageRecord tracks environment usage for billing
type UsageRecord struct {
	ID            string     `json:"id"`
	TeamID        string     `json:"team_id"`
	EnvironmentID string     `json:"environment_id"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	DurationSec   int        `json:"duration_sec"`
}
