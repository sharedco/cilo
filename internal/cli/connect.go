// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/sharedco/cilo/internal/dns"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <host>",
	Short: "Connect to a remote machine via WireGuard tunnel",
	Long: `Connect to a remote machine running cilod.

This command establishes a WireGuard tunnel to the remote machine and
registers it in your local machine state. Once connected, you can use
the --on flag with other commands to target this machine.

If you can already SSH to the host without password (e.g., via Tailscale),
the connection will use your existing SSH access. Otherwise, it uses
SSH key challenge-response authentication.

Examples:
  cilo connect myserver.example.com
  cilo connect 192.168.1.100:8080
  cilo connect 100.64.0.1:8080   # For Tailscale - SSH access detected automatically
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

func runConnect(host string) error {
	fmt.Printf("Connecting to %s...\n", host)

	if IsConnected(host) {
		return fmt.Errorf("machine already connected: %s", host)
	}

	resolvedHost := resolveHostWithPort(host)
	useDirect := canConnectDirectly(host)

	var client *cilod.Client
	var token string
	var err error

	if useDirect {
		if isInTailscaleNetwork(strings.Split(host, ":")[0]) {
			fmt.Println("  ✓ Tailscale network detected - using direct connection")
		} else {
			fmt.Println("  ✓ SSH access detected - using direct connection")
		}
		client, token, err = connectViaSSH(host, resolvedHost)
	} else {
		pubKeyPath, privKeyPath, err := findSSHKey()
		if err != nil {
			return fmt.Errorf("no SSH key found: %w", err)
		}
		fmt.Printf("  Using SSH key: %s\n", pubKeyPath)

		client = cilod.NewClient(resolvedHost, "")
		fmt.Println("  Authenticating...")
		token, err = client.Connect(privKeyPath)
	}

	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("  Generating WireGuard keys...")
	wgKeys, err := tunnel.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}

	fmt.Println("  Exchanging WireGuard keys...")
	client.SetToken(token)
	wgConfig, err := client.WireGuardExchange(wgKeys.PublicKey)
	if err != nil {
		return fmt.Errorf("wireguard exchange failed: %w", err)
	}

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

	if err := SaveMachine(machine); err != nil {
		return fmt.Errorf("failed to save machine state: %w", err)
	}

	envCount := 0
	envs, err := client.ListEnvironments()
	if err == nil {
		envCount = len(envs)
	}

	if envCount > 0 {
		fmt.Println("  Setting up DNS entries...")
		remoteMachine := &dns.RemoteMachine{
			Host:         host,
			WGAssignedIP: wgConfig.AssignedIP,
		}
		if err := dns.AddRemoteMachine(remoteMachine, envs); err != nil {
			fmt.Printf("  Warning: failed to add DNS entries: %v\n", err)
		}
	}

	fmt.Printf("\n✓ Connected to %s\n", host)
	fmt.Printf("  WireGuard IP: %s\n", wgConfig.AssignedIP)
	if envCount > 0 {
		fmt.Printf("  Environments: %d\n", envCount)
	}
	fmt.Println("\nUse --on flag to run commands on this machine:")
	fmt.Printf("  cilo up myenv --on %s\n", host)

	return nil
}

func runDisconnect(host string) error {
	fmt.Printf("Disconnecting from %s...\n", host)

	machine, err := GetMachine(host)
	if err != nil {
		return fmt.Errorf("failed to get machine state: %w", err)
	}
	if machine == nil {
		return fmt.Errorf("machine not connected: %s", host)
	}

	fmt.Println("  Removing DNS entries...")
	envs, _ := GetRemoteEnvironments(machine.WGAssignedIP, machine.Token)
	if len(envs) > 0 {
		if err := dns.RemoveRemoteMachine(host); err != nil {
			fmt.Printf("  Warning: failed to remove DNS entries: %v\n", err)
		}
	}

	if machine.WGInterface != "" {
		fmt.Printf("  Stopping tunnel...\n")
	}

	if err := RemoveMachine(host); err != nil {
		return fmt.Errorf("failed to remove machine state: %w", err)
	}

	fmt.Printf("\nDisconnected from %s\n", host)
	return nil
}

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

func canConnectDirectly(host string) bool {
	targetIP := host
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		targetIP = parts[0]
	}

	if isInTailscaleNetwork(targetIP) {
		return true
	}

	return canSSH(host)
}

func isInTailscaleNetwork(ip string) bool {
	cmd := exec.Command("tailscale", "status")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), ip)
}

func canSSH(host string) bool {
	sshHost := host
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		sshHost = parts[0]
	}

	testCmd := exec.Command("ssh", "-o", "ConnectTimeout=3", "-o", "BatchMode=yes", sshHost, "echo", "SSH_OK")
	output, err := testCmd.CombinedOutput()
	return err == nil && strings.Contains(string(output), "SSH_OK")
}

func connectViaSSH(host, resolvedHost string) (*cilod.Client, string, error) {
	sshHost := host
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		sshHost = parts[0]
	}

	fmt.Printf("  Testing SSH connection to %s...\n", sshHost)
	testCmd := exec.Command("ssh", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes", sshHost, "echo", "SSH_OK")
	if output, err := testCmd.CombinedOutput(); err != nil || !strings.Contains(string(output), "SSH_OK") {
		return nil, "", fmt.Errorf("SSH connection failed. Ensure you can 'ssh %s' without password", sshHost)
	}
	fmt.Println("  ✓ SSH connection successful")
	fmt.Println("  Note: Using direct HTTP connection (Tailscale provides network security)")

	client := cilod.NewClient(resolvedHost, "")
	token := fmt.Sprintf("tailscale-%d", time.Now().Unix())

	return client, token, nil
}

func findSSHKey() (pubKeyPath, privKeyPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")

	ed25519Priv := filepath.Join(sshDir, "id_ed25519")
	ed25519Pub := filepath.Join(sshDir, "id_ed25519.pub")
	if _, err := os.Stat(ed25519Priv); err == nil {
		if _, err := os.Stat(ed25519Pub); err == nil {
			return ed25519Pub, ed25519Priv, nil
		}
	}

	rsaPriv := filepath.Join(sshDir, "id_rsa")
	rsaPub := filepath.Join(sshDir, "id_rsa.pub")
	if _, err := os.Stat(rsaPriv); err == nil {
		if _, err := os.Stat(rsaPub); err == nil {
			return rsaPub, rsaPriv, nil
		}
	}

	return "", "", fmt.Errorf("no SSH key found in %s (tried id_ed25519, id_rsa)", sshDir)
}

func resolveHostWithPort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return net.JoinHostPort(host, "8081")
}

func GetRemoteEnvironments(host, token string) ([]string, error) {
	client := cilod.NewClient(host, token)
	envs, err := client.ListEnvironments()
	if err != nil {
		return nil, err
	}

	result := make([]string, len(envs))
	for i, env := range envs {
		result[i] = env.Name
	}
	return result, nil
}

func init() {
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(disconnectCmd)
}
