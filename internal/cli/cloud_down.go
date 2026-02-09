// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sharedco/cilo/internal/cloud"
	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/spf13/cobra"
)

var cloudDownCmd = &cobra.Command{
	Use:   "down <name>",
	Short: "Stop a remote environment",
	Long: `Stop a remote environment without destroying it.

The environment's data and workspace are preserved.
Use 'cilo cloud up' to restart it, or 'cilo cloud destroy' to remove it.

Example:
  cilo cloud down agent-1`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudDown,
}

func init() {
	cloudCmd.AddCommand(cloudDownCmd)
}

func runCloudDown(cmd *cobra.Command, args []string) error {
	envName := args[0]

	fmt.Printf("Stopping cloud environment: %s\n", envName)

	client, err := cloud.NewClientFromAuth()
	if err != nil {
		return fmt.Errorf("cloud auth: %w (run 'cilo cloud login' first)", err)
	}

	ctx := context.Background()

	env, err := client.GetEnvironmentByName(ctx, envName)
	if err != nil {
		return fmt.Errorf("find environment: %w", err)
	}

	if err := client.StopEnvironment(ctx, env.ID); err != nil {
		return fmt.Errorf("stop environment: %w", err)
	}

	if err := teardownWireGuard(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to teardown WireGuard: %v\n", err)
	}

	if err := removeEnvironmentDNS(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove DNS entries: %v\n", err)
	}

	if err := cleanupLocalState(envName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup local state: %v\n", err)
	}

	fmt.Printf("Environment %s stopped\n", envName)
	return nil
}

func teardownWireGuard(envName string) error {
	interfaceNames := []string{
		fmt.Sprintf("cilo-%s", envName),
		"cilo0",
		"cilo-wg",
	}

	for _, iface := range interfaceNames {
		if err := tunnel.RemoveInterface(iface); err == nil {
			fmt.Printf("Removed WireGuard interface: %s\n", iface)
			return nil
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
		if strings.Contains(entry.Name(), envName) && strings.HasSuffix(entry.Name(), ".conf") {
			configPath := filepath.Join(tunnelDir, entry.Name())
			iface := strings.TrimSuffix(entry.Name(), ".conf")
			fmt.Printf("Shutting down WireGuard: %s\n", iface)
			_ = exec.Command("wg-quick", "down", configPath).Run()
			os.Remove(configPath)
		}
	}

	return nil
}

func removeEnvironmentDNS(envName string) error {
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

func cleanupLocalState(envName string) error {
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
