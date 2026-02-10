// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/agent/config"
)

// WireGuardManager handles WireGuard interface operations
type WireGuardManager struct {
	interfaceName string // e.g., "wg0"
	listenPort    int    // e.g., 51820
	privateKey    string // Agent's WG private key (from config)
	publicKey     string // Derived from private key
	address       string // Interface address, e.g., "10.225.0.100/16"
}

// NewWireGuardManager creates a new WireGuard manager with the given configuration.
func NewWireGuardManager(cfg *config.Config) (*WireGuardManager, error) {
	if cfg.WGPrivateKey == "" {
		return nil, fmt.Errorf("WireGuard private key not configured")
	}

	// Derive public key from private key
	publicKey, err := derivePublicKey(cfg.WGPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return &WireGuardManager{
		interfaceName: cfg.WGInterface,
		listenPort:    cfg.WGListenPort,
		privateKey:    cfg.WGPrivateKey,
		publicKey:     publicKey,
		address:       cfg.WGAddress,
	}, nil
}

// derivePublicKey derives a public key from a private key using wg pubkey
func derivePublicKey(privateKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wg", "pubkey")
	cmd.Stdin = strings.NewReader(privateKey)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// EnsureInterface creates the WireGuard interface if it doesn't exist
func (m *WireGuardManager) EnsureInterface(ctx context.Context) error {
	// Check if interface exists
	checkCmd := exec.CommandContext(ctx, "ip", "link", "show", m.interfaceName)
	if err := checkCmd.Run(); err == nil {
		// Interface exists, nothing to do
		return nil
	}

	// Create the interface
	if err := m.createInterface(ctx); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// Configure the interface
	if err := m.configureInterface(ctx); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	// Bring the interface up
	if err := m.bringUp(ctx); err != nil {
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	if err := m.setupForwarding(ctx); err != nil {
		return fmt.Errorf("failed to setup forwarding: %w", err)
	}

	return nil
}

// createInterface creates the WireGuard interface
func (m *WireGuardManager) createInterface(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ip", "link", "add", "dev", m.interfaceName, "type", "wireguard")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ip link add: %w", err)
	}
	return nil
}

// configureInterface sets up the interface with address and WireGuard settings
func (m *WireGuardManager) configureInterface(ctx context.Context) error {
	// Set IP address
	addrCmd := exec.CommandContext(ctx, "ip", "address", "add", m.address, "dev", m.interfaceName)
	if err := addrCmd.Run(); err != nil {
		return fmt.Errorf("ip address add: %w", err)
	}

	// Set WireGuard private key and listen port
	wgCmd := exec.CommandContext(ctx, "wg", "set", m.interfaceName,
		"listen-port", strconv.Itoa(m.listenPort),
		"private-key", "/dev/stdin")
	wgCmd.Stdin = strings.NewReader(m.privateKey)
	if err := wgCmd.Run(); err != nil {
		return fmt.Errorf("wg set: %w", err)
	}

	return nil
}

// bringUp brings the interface up
func (m *WireGuardManager) bringUp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ip", "link", "set", "up", "dev", m.interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ip link set up: %w", err)
	}
	return nil
}

func (m *WireGuardManager) setupForwarding(ctx context.Context) error {
	if err := exec.CommandContext(ctx, "sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
		return fmt.Errorf("enable ip forwarding: %w", err)
	}

	rules := [][]string{
		{"-I", "DOCKER-USER", "-i", m.interfaceName, "-j", "ACCEPT"},
		{"-I", "DOCKER-USER", "-o", m.interfaceName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-A", "FORWARD", "-i", m.interfaceName, "-j", "ACCEPT"},
		{"-A", "FORWARD", "-o", m.interfaceName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		checkArgs := make([]string, len(rule))
		copy(checkArgs, rule)
		if checkArgs[0] == "-I" || checkArgs[0] == "-A" {
			checkArgs[0] = "-C"
		}
		if exec.CommandContext(ctx, "iptables", checkArgs...).Run() == nil {
			continue
		}
		if err := exec.CommandContext(ctx, "iptables", rule...).Run(); err != nil {
			return fmt.Errorf("iptables %v: %w", rule, err)
		}
	}

	natRule := []string{"-t", "nat", "-A", "POSTROUTING", "-s", "10.225.0.0/16", "-o", "docker0", "-j", "MASQUERADE"}
	natCheck := []string{"-t", "nat", "-C", "POSTROUTING", "-s", "10.225.0.0/16", "-o", "docker0", "-j", "MASQUERADE"}
	if exec.CommandContext(ctx, "iptables", natCheck...).Run() != nil {
		if err := exec.CommandContext(ctx, "iptables", natRule...).Run(); err != nil {
			fmt.Printf("Warning: MASQUERADE on docker0 failed (may use bridge network): %v\n", err)
		}
	}

	return nil
}

func (m *WireGuardManager) cleanupForwarding(ctx context.Context) {
	rules := [][]string{
		{"-D", "DOCKER-USER", "-i", m.interfaceName, "-j", "ACCEPT"},
		{"-D", "DOCKER-USER", "-o", m.interfaceName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-D", "FORWARD", "-i", m.interfaceName, "-j", "ACCEPT"},
		{"-D", "FORWARD", "-o", m.interfaceName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
	}
	for _, rule := range rules {
		exec.CommandContext(ctx, "iptables", rule...).Run()
	}

	natRule := []string{"-t", "nat", "-D", "POSTROUTING", "-s", "10.225.0.0/16", "-o", "docker0", "-j", "MASQUERADE"}
	exec.CommandContext(ctx, "iptables", natRule...).Run()
}

// AddPeer adds a peer to the WireGuard interface
func (m *WireGuardManager) AddPeer(ctx context.Context, publicKey string, allowedIPs []string) error {
	// Validate public key format
	if err := validatePublicKey(publicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Validate allowed IPs
	for _, ip := range allowedIPs {
		if strings.TrimSpace(ip) == "" {
			return fmt.Errorf("empty allowed IP")
		}
	}

	allowedIPsStr := strings.Join(allowedIPs, ",")

	cmd := exec.CommandContext(ctx, "wg", "set", m.interfaceName,
		"peer", publicKey,
		"allowed-ips", allowedIPsStr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	return nil
}

// RemovePeer removes a peer from the WireGuard interface
func (m *WireGuardManager) RemovePeer(ctx context.Context, publicKey string) error {
	// Validate public key format
	if err := validatePublicKey(publicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	cmd := exec.CommandContext(ctx, "wg", "set", m.interfaceName,
		"peer", publicKey, "remove")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	return nil
}

// GetStatus returns current WireGuard interface status
func (m *WireGuardManager) GetStatus(ctx context.Context) (*WGStatusResponse, error) {
	if m == nil {
		return nil, fmt.Errorf("WireGuard manager not initialized")
	}

	if m.interfaceName == "" {
		return nil, fmt.Errorf("WireGuard interface name not configured")
	}

	cmd := exec.CommandContext(ctx, "wg", "show", m.interfaceName, "dump")

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return m.parseWGDump(out.String())
}

// parseWGDump parses the output of 'wg show <interface> dump'
// Format:
// Line 1 (interface): private-key public-key listen-port fwmark
// Line 2+ (peers): public-key preshared-key endpoint allowed-ips latest-handshake transfer-rx transfer-tx persistent-keepalive
func (m *WireGuardManager) parseWGDump(output string) (*WGStatusResponse, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty wg dump output")
	}

	response := &WGStatusResponse{
		Interface: m.interfaceName,
		PublicKey: m.publicKey,
		Peers:     []PeerStatus{},
	}

	// Skip first line (interface info)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		peer, err := m.parsePeerLine(line)
		if err != nil {
			// Log but don't fail on individual peer parse errors
			continue
		}

		response.Peers = append(response.Peers, peer)
	}

	return response, nil
}

// parsePeerLine parses a single peer line from wg dump output
func (m *WireGuardManager) parsePeerLine(line string) (PeerStatus, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return PeerStatus{}, fmt.Errorf("invalid peer line: insufficient fields")
	}

	peer := PeerStatus{
		PublicKey:  fields[0],
		Endpoint:   fields[2],
		AllowedIPs: fields[3],
	}

	// Parse latest handshake (unix timestamp)
	if fields[4] != "0" {
		if timestamp, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			peer.LastHandshake = time.Unix(timestamp, 0).Format(time.RFC3339)
		}
	}

	// Parse transfer stats
	if rxBytes, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
		peer.RxBytes = rxBytes
	}
	if txBytes, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
		peer.TxBytes = txBytes
	}

	return peer, nil
}

// GetPublicKey returns the agent's WireGuard public key
func (m *WireGuardManager) GetPublicKey() string {
	return m.publicKey
}

// validatePublicKey validates that a public key is properly formatted
func validatePublicKey(key string) error {
	// WireGuard public keys are base64-encoded 32-byte values
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("not valid base64: %w", err)
	}

	if len(decoded) != 32 {
		return fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(decoded))
	}

	return nil
}
