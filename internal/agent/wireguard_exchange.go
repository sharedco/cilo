// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// JSONPeerStore implements PeerStore using a JSON file for persistence
// Stores peer IP allocations at /var/cilo/peers.json
type JSONPeerStore struct {
	mu       sync.RWMutex
	filePath string
	subnet   string
	data     *PeerIPAllocation
}

// NewJSONPeerStore creates a new JSON-backed peer store
func NewJSONPeerStore(filePath string) (*JSONPeerStore, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create peers directory: %w", err)
	}

	store := &JSONPeerStore{
		filePath: filePath,
		subnet:   "10.225.0.0/24",
		data: &PeerIPAllocation{
			Peers:  make(map[string]string),
			NextIP: "10.225.0.2",
		},
	}

	// Load existing data if present
	if err := store.load(); err != nil {
		// If file doesn't exist, we'll create it on first allocation
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load peers file: %w", err)
		}
	}

	return store, nil
}

// GetPeerIP returns the assigned IP for a peer, or "" if not allocated
func (s *JSONPeerStore) GetPeerIP(publicKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ip, exists := s.data.Peers[publicKey]
	if !exists {
		return "", nil
	}
	return ip, nil
}

// AllocatePeerIP assigns the next available IP to a peer
// Returns existing IP if peer already has an allocation
func (s *JSONPeerStore) AllocatePeerIP(publicKey string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if peer already has an IP
	if ip, exists := s.data.Peers[publicKey]; exists {
		return ip, nil
	}

	// Assign next available IP
	assignedIP := s.data.NextIP
	s.data.Peers[publicKey] = assignedIP

	// Increment next IP
	nextIP, err := incrementIP(assignedIP)
	if err != nil {
		return "", fmt.Errorf("failed to increment IP: %w", err)
	}
	s.data.NextIP = nextIP

	// Persist to disk
	if err := s.save(); err != nil {
		// Rollback on save failure
		delete(s.data.Peers, publicKey)
		s.data.NextIP = assignedIP
		return "", fmt.Errorf("failed to save peers file: %w", err)
	}

	return assignedIP, nil
}

// RemovePeer removes a peer's IP allocation
func (s *JSONPeerStore) RemovePeer(publicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data.Peers, publicKey)

	return s.save()
}

// ListPeers returns all peer allocations
func (s *JSONPeerStore) ListPeers() ([]PeerAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var peers []PeerAllocation
	for pubkey, ip := range s.data.Peers {
		peers = append(peers, PeerAllocation{
			PublicKey: pubkey,
			IP:        ip,
		})
	}
	return peers, nil
}

// load reads the peers file from disk
func (s *JSONPeerStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var allocation PeerIPAllocation
	if err := json.Unmarshal(data, &allocation); err != nil {
		return fmt.Errorf("failed to parse peers file: %w", err)
	}

	if allocation.Peers == nil {
		allocation.Peers = make(map[string]string)
	}

	s.data = &allocation
	return nil
}

// save writes the peers file to disk
func (s *JSONPeerStore) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal peers data: %w", err)
	}

	// Write to temporary file first, then rename for atomicity
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write peers file: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename peers file: %w", err)
	}

	return nil
}

// incrementIP increments the last octet of an IP address
// e.g., 10.225.0.2 -> 10.225.0.3
func incrementIP(ip string) (string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	// Convert to IPv4
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return "", fmt.Errorf("not an IPv4 address: %s", ip)
	}

	// Increment last octet
	if ipv4[3] == 255 {
		return "", fmt.Errorf("IP address overflow: %s", ip)
	}
	ipv4[3]++

	return ipv4.String(), nil
}
