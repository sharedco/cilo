// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package vm

import (
	"context"
	"time"
)

// Machine represents a VM in the pool
type Machine struct {
	ID           string
	ProviderID   string // Provider-specific ID (e.g., Hetzner server ID)
	ProviderType string // "manual", "hetzner", "aws"
	PublicIP     string
	PrivateIP    string
	WGPublicKey  string
	WGEndpoint   string
	Status       string // "provisioning", "ready", "assigned", "draining", "destroying"
	AssignedEnv  string // Environment ID if assigned
	SSHHost      string // For manual provider
	SSHUser      string
	Region       string
	Size         string
	CreatedAt    time.Time
	LastHealthAt time.Time
}

// MachineStatus constants
const (
	MachineStatusProvisioning = "provisioning"
	MachineStatusReady        = "ready"
	MachineStatusAssigned     = "assigned"
	MachineStatusDraining     = "draining"
	MachineStatusDestroying   = "destroying"
	MachineStatusFailed       = "failed"
)

// ProvisionConfig contains configuration for provisioning a new machine
type ProvisionConfig struct {
	Name     string
	Size     string   // e.g., "cx31" for Hetzner
	Region   string   // e.g., "nbg1"
	ImageID  string   // Custom image ID
	SSHKeys  []string // SSH key IDs or fingerprints
	Labels   map[string]string
	UserData string // Cloud-init user data
}

// Provider is the interface for VM provisioning
type Provider interface {
	// Name returns the provider name
	Name() string

	// Provision creates a new VM
	Provision(ctx context.Context, config ProvisionConfig) (*Machine, error)

	// Destroy removes a VM
	Destroy(ctx context.Context, providerID string) error

	// List returns all VMs managed by this provider
	List(ctx context.Context) ([]*Machine, error)

	// HealthCheck checks if a machine is healthy
	HealthCheck(ctx context.Context, providerID string) (bool, error)

	// GetMachine returns a single machine by provider ID
	GetMachine(ctx context.Context, providerID string) (*Machine, error)
}
