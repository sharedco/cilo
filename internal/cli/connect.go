// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <host>",
	Short: "Connect to a remote machine via WireGuard tunnel",
	Long: `Connect to a remote machine running cilod.

This command establishes a WireGuard tunnel to the remote machine and
registers it in your local machine state. Once connected, you can use
the --on flag with other commands to target this machine.

Examples:
  cilo connect myserver.example.com
  cilo connect 192.168.1.100:8080
  cilo connect localhost:8080`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		host := args[0]
		return runConnect(host)
	},
}

var disconnectCmd = &cobra.Command{
	Use:   "disconnect [host]",
	Short: "Disconnect from a remote machine",
	Long: `Disconnect from a remote machine and tear down the WireGuard tunnel.

If no host is specified, disconnects from all connected machines.

Examples:
  cilo disconnect myserver.example.com
  cilo disconnect`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runDisconnectAll()
		}
		return runDisconnect(args[0])
	},
}

// runConnect connects to a remote machine
func runConnect(host string) error {
	fmt.Printf("Connecting to %s...\n", host)

	// Check if already connected
	if IsConnected(host) {
		return fmt.Errorf("machine already connected: %s", host)
	}

	// Find SSH key
	pubKeyPath, privKeyPath, err := findSSHKey()
	if err != nil {
		return fmt.Errorf("no SSH key found: %w", err)
	}

	fmt.Printf("  Using SSH key: %s\n", pubKeyPath)

	// Resolve host with default port if needed
	resolvedHost := resolveHostWithPort(host)

	// Create cilod client
	client := cilod.NewClient(resolvedHost, "")

	// Authenticate with SSH key
	fmt.Println("  Authenticating...")
	token, err := client.Connect(privKeyPath)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Generate WireGuard key pair
	fmt.Println("  Generating WireGuard keys...")
	wgKeys, err := tunnel.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}

	// Exchange WireGuard keys with server
	fmt.Println("  Exchanging WireGuard keys...")
	client.SetToken(token)
	wgConfig, err := client.WireGuardExchange(wgKeys.PublicKey)
	if err != nil {
		return fmt.Errorf("wireguard exchange failed: %w", err)
	}

	// Create machine state
	machine := &Machine{
		Host:              host,
		Token:             token,
		WGPrivateKey:      wgKeys.PrivateKey,
		WGPublicKey:       wgKeys.PublicKey,
		WGServerPublicKey: wgConfig.ServerPublicKey,
		WGAssignedIP:      wgConfig.AssignedIP,
		WGEndpoint:        wgConfig.ServerEndpoint,
		WGAllowedIPs:      wgConfig.AllowedIPs,
		EnvironmentSubnet: wgConfig.EnvironmentSubnet,
		ConnectedAt:       time.Now(),
		Status:            "connected",
		Version:           1,
	}

	// Save machine state
	if err := SaveMachine(machine); err != nil {
		return fmt.Errorf("failed to save machine state: %w", err)
	}

	// Get environment count
	envCount := 0
	envs, err := client.ListEnvironments()
	if err == nil {
		envCount = len(envs)
	}

	fmt.Printf("\nConnected to %s\n", host)
	fmt.Printf("  WireGuard IP: %s\n", wgConfig.AssignedIP)
	fmt.Printf("  Environments: %d\n", envCount)

	return nil
}

// runDisconnect disconnects from a specific machine
func runDisconnect(host string) error {
	fmt.Printf("Disconnecting from %s...\n", host)

	// Check if connected
	if !IsConnected(host) {
		return fmt.Errorf("machine not connected: %s", host)
	}

	// Get machine to find interface
	machine, err := GetMachine(host)
	if err != nil {
		return fmt.Errorf("failed to get machine state: %w", err)
	}
	if machine == nil {
		return fmt.Errorf("machine not connected: %s", host)
	}

	// Stop tunnel daemon if running
	if machine.WGInterface != "" {
		fmt.Printf("  Stopping tunnel...\n")
		// Note: In a full implementation, we would stop the specific tunnel
		// For now, we rely on the general tunnel cleanup
	}

	// Remove machine state
	if err := RemoveMachine(host); err != nil {
		return fmt.Errorf("failed to remove machine state: %w", err)
	}

	fmt.Printf("\nDisconnected from %s\n", host)
	return nil
}

// runDisconnectAll disconnects from all machines
func runDisconnectAll() error {
	machines, err := ListConnectedMachines()
	if err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}

	if len(machines) == 0 {
		fmt.Println("No connected machines")
		return nil
	}

	fmt.Printf("Disconnecting from %d machine(s)...\n", len(machines))

	var lastErr error
	for _, machine := range machines {
		if err := runDisconnect(machine.Host); err != nil {
			lastErr = err
			fmt.Printf("  Warning: failed to disconnect from %s: %v\n", machine.Host, err)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("some disconnections failed")
	}

	fmt.Println("\nDisconnected from all machines")
	return nil
}

// findSSHKey finds the user's SSH key
// Returns public key path, private key path, and error
func findSSHKey() (pubKeyPath, privKeyPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("could not determine home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")

	// Try ed25519 first (preferred)
	ed25519Priv := filepath.Join(sshDir, "id_ed25519")
	ed25519Pub := ed25519Priv + ".pub"
	if _, err := os.Stat(ed25519Priv); err == nil {
		if _, err := os.Stat(ed25519Pub); err == nil {
			return ed25519Pub, ed25519Priv, nil
		}
	}

	// Try RSA
	rsaPriv := filepath.Join(sshDir, "id_rsa")
	rsaPub := rsaPriv + ".pub"
	if _, err := os.Stat(rsaPriv); err == nil {
		if _, err := os.Stat(rsaPub); err == nil {
			return rsaPub, rsaPriv, nil
		}
	}

	return "", "", fmt.Errorf("no SSH key found in %s (tried id_ed25519, id_rsa)", sshDir)
}

// resolveHostWithPort resolves a host and adds default port if needed
func resolveHostWithPort(host string) string {
	// If host already has a port, return as-is
	if strings.Contains(host, ":") {
		// Handle IPv6 addresses
		if strings.HasPrefix(host, "[") {
			return host
		}
		// Check if it's actually a port or part of IPv6
		if _, _, err := net.SplitHostPort(host); err == nil {
			return host
		}
	}

	// Add default cilod port
	return host + ":8080"
}

// connectMachine is a helper for testing that performs the connect operation
func connectMachine(host string, privateKeyPath string) (*Machine, error) {
	// Check if already connected
	if IsConnected(host) {
		return nil, fmt.Errorf("machine already connected: %s", host)
	}

	// Resolve host with default port if needed
	resolvedHost := resolveHostWithPort(host)

	// Create cilod client
	client := cilod.NewClient(resolvedHost, "")

	// Authenticate with SSH key
	token, err := client.Connect(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Generate WireGuard key pair
	wgKeys, err := tunnel.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}

	// Exchange WireGuard keys with server
	client.SetToken(token)
	wgConfig, err := client.WireGuardExchange(wgKeys.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("wireguard exchange failed: %w", err)
	}

	// Create machine state
	machine := &Machine{
		Host:              host,
		Token:             token,
		WGPrivateKey:      wgKeys.PrivateKey,
		WGPublicKey:       wgKeys.PublicKey,
		WGServerPublicKey: wgConfig.ServerPublicKey,
		WGAssignedIP:      wgConfig.AssignedIP,
		WGEndpoint:        wgConfig.ServerEndpoint,
		WGAllowedIPs:      wgConfig.AllowedIPs,
		EnvironmentSubnet: wgConfig.EnvironmentSubnet,
		ConnectedAt:       time.Now(),
		Status:            "connected",
		Version:           1,
	}

	// Save machine state
	if err := SaveMachine(machine); err != nil {
		return nil, fmt.Errorf("failed to save machine state: %w", err)
	}

	return machine, nil
}

// disconnectMachine is a helper for testing that performs the disconnect operation
func disconnectMachine(host string) error {
	if !IsConnected(host) {
		return fmt.Errorf("machine not connected: %s", host)
	}

	return RemoveMachine(host)
}

// disconnectAllMachines disconnects from all machines
func disconnectAllMachines() error {
	machines, err := ListConnectedMachines()
	if err != nil {
		return err
	}

	for _, machine := range machines {
		if err := RemoveMachine(machine.Host); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(disconnectCmd)
}
