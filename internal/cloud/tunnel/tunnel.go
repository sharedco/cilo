// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package tunnel

import (
	"fmt"
	"time"
)

// Tunnel represents a complete WireGuard tunnel configuration
type Tunnel struct {
	manager    *Manager
	Interface  string
	PrivateKey string
	PublicKey  string
	ListenPort int
	Address    string // Local IP address (e.g., "10.225.0.1/24")
}

// Config for creating a new tunnel
type Config struct {
	Interface  string
	ListenPort int
	Address    string // Local IP in CIDR notation
}

// New creates a new WireGuard tunnel
func New(cfg Config) (*Tunnel, error) {
	// Generate key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}

	return &Tunnel{
		Interface:  cfg.Interface,
		PrivateKey: keyPair.PrivateKey,
		PublicKey:  keyPair.PublicKey,
		ListenPort: cfg.ListenPort,
		Address:    cfg.Address,
	}, nil
}

// Setup creates and configures the WireGuard interface
func (t *Tunnel) Setup() error {
	// Create network interface
	actualName, err := CreateInterface(t.Interface)
	if err != nil {
		return fmt.Errorf("create interface: %w", err)
	}

	t.Interface = actualName

	// Create manager
	manager, err := NewManager(t.Interface)
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}
	t.manager = manager

	// Configure WireGuard
	if err := manager.Configure(t.PrivateKey, t.ListenPort); err != nil {
		return fmt.Errorf("configure wireguard: %w", err)
	}

	// Add IP address
	if t.Address != "" {
		if err := AddAddress(t.Interface, t.Address); err != nil {
			return fmt.Errorf("add address: %w", err)
		}
	}

	// Bring interface up
	if err := SetInterfaceUp(t.Interface); err != nil {
		return fmt.Errorf("set interface up: %w", err)
	}

	return nil
}

// AddPeer adds a peer to the tunnel
func (t *Tunnel) AddPeer(publicKey string, endpoint string, allowedIPs []string) error {
	if t.manager == nil {
		return fmt.Errorf("tunnel not set up")
	}

	keepalive := 25 * time.Second // NAT traversal keepalive
	return t.manager.AddPeer(publicKey, endpoint, allowedIPs, keepalive)
}

// RemovePeer removes a peer from the tunnel
func (t *Tunnel) RemovePeer(publicKey string) error {
	if t.manager == nil {
		return fmt.Errorf("tunnel not set up")
	}
	return t.manager.RemovePeer(publicKey)
}

// Close tears down the tunnel
func (t *Tunnel) Close() error {
	if t.manager != nil {
		t.manager.Close()
	}
	return RemoveInterface(t.Interface)
}

// GetStats returns peer statistics
func (t *Tunnel) GetStats() ([]PeerStats, error) {
	if t.manager == nil {
		return nil, fmt.Errorf("tunnel not set up")
	}
	return t.manager.GetPeerStats()
}
