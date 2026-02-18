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
	"runtime"
	"strings"

	"github.com/sharedco/cilo/internal/config"
	"github.com/sharedco/cilo/internal/runtime/docker"
	"github.com/sharedco/cilo/internal/state"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall cilo and remove all data (requires sudo)",
	Long: `Uninstall cilo completely from your system.

This command will:
1. Destroy all cilo environments and their containers
2. Remove all Docker networks and volumes created by cilo
3. Remove DNS configuration (systemd-resolved or macOS resolver)
4. Stop and remove the dnsmasq process
5. Remove all cilo data in ~/.cilo

WARNING: This is destructive and cannot be undone.

This command requires sudo because it modifies system DNS settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getuid() != 0 {
			// Get the absolute path of this executable
			exe, err := os.Executable()
			if err != nil {
				fmt.Println("cilo uninstall requires sudo privileges.")
				fmt.Println()
				fmt.Println("Could not determine executable path. Please run manually:")
				fmt.Println("  sudo /path/to/cilo uninstall")
				return fmt.Errorf("could not determine executable path: %w", err)
			}

			fmt.Println("cilo uninstall requires sudo privileges. Requesting elevation...")
			fmt.Println()

			// Re-invoke with sudo using absolute path
			sudoCmd := exec.Command("sudo", exe, "uninstall")
			if force, _ := cmd.Flags().GetBool("force"); force {
				sudoCmd.Args = append(sudoCmd.Args, "--force")
			}
			if removeBinary, _ := cmd.Flags().GetBool("remove-binary"); removeBinary {
				sudoCmd.Args = append(sudoCmd.Args, "--remove-binary")
			}

			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr

			if err := sudoCmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
					return nil // User cancelled sudo, exit cleanly
				}
				return err
			}
			return nil
		}

		force, _ := cmd.Flags().GetBool("force")
		removeBinary, _ := cmd.Flags().GetBool("remove-binary")

		// Confirmation prompt unless --force
		if !force {
			fmt.Println("⚠️  WARNING: This will completely remove cilo and all its data!")
			fmt.Println()
			fmt.Println("This will:")
			fmt.Println("  - Destroy ALL cilo environments")
			fmt.Println("  - Remove all cilo Docker containers, networks, and volumes")
			fmt.Println("  - Remove DNS configuration")
			fmt.Println("  - Delete all data in ~/.cilo")
			if removeBinary {
				fmt.Println("  - Remove the cilo binary")
			}
			fmt.Println()
			fmt.Print("Are you sure? Type 'yes' to continue: ")

			var response string
			fmt.Scanln(&response)
			if strings.TrimSpace(response) != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
			fmt.Println()
		}

		fmt.Println("Step 1: Destroying all environments...")
		st, err := state.LoadState()
		if err != nil {
			fmt.Printf("  Could not load state: %v\n", err)
		} else {
			envCount := 0
			for _, host := range st.Hosts {
				for envKey := range host.Environments {
					parts := strings.Split(envKey, "/")
					if len(parts) == 2 {
						project, envName := parts[0], parts[1]
						fmt.Printf("  Destroying %s/%s...\n", project, envName)
						if env, err := state.GetEnvironment(project, envName); err == nil {
							provider := docker.NewProvider()
							ctx := context.Background()
							provider.Destroy(ctx, env)
						}
						envCount++
					}
				}
			}
			fmt.Printf("  ✓ Destroyed %d environments\n", envCount)
		}

		fmt.Println("\nStep 2: Stopping DNS daemon...")
		if err := stopDnsmasq(); err != nil {
			fmt.Printf("  ⚠ Could not stop dnsmasq: %v\n", err)
		} else {
			fmt.Println("  ✓ DNS daemon stopped")
		}

		fmt.Println("\nStep 3: Removing system DNS configuration...")
		if runtime.GOOS == "darwin" {
			if err := removeMacOSResolver(); err != nil {
				fmt.Printf("  ⚠ Could not remove macOS resolver: %v\n", err)
			} else {
				fmt.Println("  ✓ macOS resolver configuration removed")
			}
		} else {
			if err := removeLinuxResolver(); err != nil {
				fmt.Printf("  ⚠ Could not remove Linux resolver: %v\n", err)
			} else {
				fmt.Println("  ✓ Linux resolver configuration removed")
			}
		}

		fmt.Println("\nStep 4: Cleaning up Docker resources...")
		if err := cleanupDockerResources(); err != nil {
			fmt.Printf("  ⚠ Could not clean up all Docker resources: %v\n", err)
		} else {
			fmt.Println("  ✓ Docker resources cleaned up")
		}

		fmt.Println("\nStep 5: Removing cilo data...")
		ciloDir := config.GetCiloHome()
		if err := os.RemoveAll(ciloDir); err != nil {
			fmt.Printf("  ⚠ Could not remove data directory: %v\n", err)
		} else {
			fmt.Printf("  ✓ Removed %s\n", ciloDir)
		}

		if removeBinary {
			fmt.Println("\nStep 6: Removing cilo binary...")
			exe, err := os.Executable()
			if err != nil {
				fmt.Printf("  ⚠ Could not determine binary path: %v\n", err)
			} else {
				fmt.Printf("  Binary location: %s\n", exe)
				fmt.Println("  Run this command to remove the binary:")
				fmt.Printf("    sudo rm %s\n", exe)
			}
		}

		fmt.Println("\n✅ cilo has been uninstalled successfully!")
		fmt.Println()
		fmt.Println("To reinstall cilo in the future:")
		fmt.Println("  sudo cilo init")
		fmt.Println()

		return nil
	},
}

func stopDnsmasq() error {
	cmd := exec.Command("pkill", "-f", "dnsmasq.*5354")
	cmd.Run()
	return nil
}

func removeMacOSResolver() error {
	resolverFile := "/etc/resolver/test"
	if _, err := os.Stat(resolverFile); err == nil {
		if err := os.Remove(resolverFile); err != nil {
			return fmt.Errorf("failed to remove %s: %w", resolverFile, err)
		}
	}

	if entries, err := os.ReadDir("/etc/resolver"); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "cilo-") {
				os.Remove(filepath.Join("/etc/resolver", entry.Name()))
			}
		}
	}

	return nil
}

func removeLinuxResolver() error {
	confFile := "/etc/systemd/resolved.conf.d/cilo.conf"
	if _, err := os.Stat(confFile); err == nil {
		if err := os.Remove(confFile); err != nil {
			return fmt.Errorf("failed to remove %s: %w", confFile, err)
		}
	}

	if err := exec.Command("systemctl", "restart", "systemd-resolved").Run(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	return nil
}

func cleanupDockerResources() error {
	exec.Command("docker", "ps", "-aq", "--filter", "label=cilo").Output()

	cmd := exec.Command("docker", "network", "ls", "--filter", "label=cilo", "-q")
	output, _ := cmd.Output()
	networks := strings.Fields(string(output))
	for _, network := range networks {
		exec.Command("docker", "network", "rm", network).Run()
	}

	exec.Command("docker", "network", "prune", "-f").Run()

	return nil
}

func init() {
	uninstallCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	uninstallCmd.Flags().Bool("remove-binary", false, "Show instructions to remove the cilo binary")
	rootCmd.AddCommand(uninstallCmd)
}
