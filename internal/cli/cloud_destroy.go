// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sharedco/cilo/internal/cloud"
	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/spf13/cobra"
)

var cloudDestroyCmd = &cobra.Command{
	Use:   "destroy <name>",
	Short: "Destroy a remote environment",
	Long: `Permanently destroy a remote environment.

This removes all containers, data, and the workspace from the remote machine.
The machine is returned to the pool for reuse.

Example:
  cilo cloud destroy agent-1
  cilo cloud destroy agent-1 --force  # Skip confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudDestroy,
}

func init() {
	cloudDestroyCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	cloudCmd.AddCommand(cloudDestroyCmd)
}

func runCloudDestroy(cmd *cobra.Command, args []string) error {
	envName := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("Are you sure you want to destroy %s? [y/N] ", envName)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	fmt.Printf("Destroying cloud environment: %s\n", envName)

	client, err := cloud.NewClientFromAuth()
	if err != nil {
		return fmt.Errorf("cloud auth: %w (run 'cilo cloud login' first)", err)
	}

	ctx := context.Background()

	env, err := client.GetEnvironmentByName(ctx, envName)
	if err != nil {
		return fmt.Errorf("find environment: %w", err)
	}

	if err := client.DestroyEnvironment(ctx, env.ID); err != nil {
		return fmt.Errorf("destroy environment: %w", err)
	}

	if err := teardownWireGuardDestroy(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to teardown WireGuard: %v\n", err)
	}

	if err := removeEnvironmentDNSDestroy(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove DNS entries: %v\n", err)
	}

	if err := cleanupLocalStateDestroy(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup local state: %v\n", err)
	}

	fmt.Printf("Environment %s destroyed\n", envName)
	return nil
}

func teardownWireGuardDestroy(envName string) error {
	state, err := tunnel.LoadDaemonState()
	if err == nil && state.Running {
		if state.EnvironmentID == envName || strings.Contains(state.EnvironmentID, envName) {
			fmt.Println("Stopping WireGuard tunnel daemon...")
			if err := tunnel.StopDaemon(); err != nil {
				fmt.Printf("  Warning: %v\n", err)
			} else {
				fmt.Println("  ✓ Tunnel daemon stopped")
			}
		}
	}

	tunnel.ClearDaemonState()

	interfaceNames := []string{
		fmt.Sprintf("cilo-%s", envName),
		"cilo0",
		"cilo-wg",
	}

	for _, iface := range interfaceNames {
		if err := tunnel.RemoveInterface(iface); err == nil {
			fmt.Printf("Removed WireGuard interface: %s\n", iface)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tunnelDir := filepath.Join(homeDir, ".cilo", "tunnels")
	entries, err := os.ReadDir(tunnelDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), envName) {
			configPath := filepath.Join(tunnelDir, entry.Name())
			os.Remove(configPath)
		}
	}

	return nil
}

func removeEnvironmentDNSDestroy(envName string) error {
	resolverDir := "/etc/resolver"

	entries, err := os.ReadDir(resolverDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), envName) {
			resolverPath := filepath.Join(resolverDir, entry.Name())
			if err := os.Remove(resolverPath); err == nil {
				fmt.Printf("Removed DNS resolver: %s\n", resolverPath)
			}
		}
	}

	return nil
}

func cleanupLocalStateDestroy(envName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tunnelDir := filepath.Join(homeDir, ".cilo", "tunnels")
	envTunnelDir := filepath.Join(tunnelDir, envName)
	os.RemoveAll(envTunnelDir)

	cloudDir := filepath.Join(homeDir, ".cilo", "cloud")
	envCacheFile := filepath.Join(cloudDir, fmt.Sprintf("%s.json", envName))
	os.Remove(envCacheFile)

	return nil
}
