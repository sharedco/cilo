// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage WireGuard tunnel for cloud environments",
	Long: `Manage the WireGuard tunnel that connects to remote cloud environments.

The tunnel daemon runs in the background to maintain connectivity to
remote environments created with 'cilo cloud up'.

Commands:
  cilo tunnel status   - Show tunnel status
  cilo tunnel stop     - Stop the tunnel daemon
  cilo tunnel daemon   - Run the tunnel daemon (internal use)`,
}

var tunnelStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tunnel status",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := tunnel.GetDaemonStatus()
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}

		if !state.Running {
			fmt.Println("Tunnel: not running")
			return nil
		}

		fmt.Println("Tunnel: running")
		fmt.Printf("  Interface: %s\n", state.Interface)
		fmt.Printf("  Address: %s\n", state.Address)
		fmt.Printf("  Environment: %s\n", state.EnvironmentID)
		fmt.Printf("  Started: %s\n", state.StartedAt.Format(time.RFC3339))
		fmt.Printf("  PID: %d\n", state.PID)

		return nil
	},
}

var tunnelStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the tunnel daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := tunnel.LoadDaemonState()
		if err != nil || !state.Running {
			fmt.Println("Tunnel is not running")
			return nil
		}

		fmt.Println("Stopping tunnel daemon...")
		if err := tunnel.StopDaemon(); err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}

		fmt.Println("✓ Tunnel stopped")
		return nil
	},
}

var tunnelCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Kill all tunnel processes and clean up state",
	Long: `Forcefully clean up all tunnel state. Use when tunnel is stuck or port conflicts occur.

This command:
  1. Kills ALL cilo tunnel daemon processes
  2. Removes tunnel state files (~/.cilo/tunnel/)
  3. Removes any stale utun interfaces

Requires sudo.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("tunnel clean requires root privileges.\n\nRun: sudo cilo tunnel clean")
		}

		fmt.Println("Cleaning up tunnel state...")

		fmt.Print("  → Killing tunnel processes... ")
		killCmd := exec.Command("pkill", "-9", "-f", "cilo tunnel")
		killCmd.Run()
		fmt.Println("done")

		fmt.Print("  → Removing state files... ")
		tunnelDir, err := tunnel.DaemonDir()
		if err == nil {
			os.RemoveAll(tunnelDir)
		}
		fmt.Println("done")

		fmt.Print("  → Cleaning up interfaces... ")
		for i := 0; i < 20; i++ {
			ifname := fmt.Sprintf("utun%d", i)
			checkCmd := exec.Command("ifconfig", ifname)
			output, err := checkCmd.Output()
			if err != nil {
				continue
			}
			if strings.Contains(string(output), "10.225.0.") {
				exec.Command("ifconfig", ifname, "down").Run()
			}
		}
		fmt.Println("done")

		fmt.Println("✓ Tunnel cleaned up")
		fmt.Println("\nYou can now run: sudo cilo cloud up <name>")
		return nil
	},
}

var tunnelDaemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the tunnel daemon (internal use)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("daemon must run as root")
		}

		cfg, err := tunnel.LoadDaemonConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		daemon, err := tunnel.NewDaemon(cfg)
		if err != nil {
			return fmt.Errorf("create daemon: %w", err)
		}

		return daemon.Run()
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all cilo state from this machine",
	Long: `Nuclear clean - removes ALL cilo state from this machine:

  - Tunnel processes and state
  - Cloud authentication
  - DNS configuration
  - Local environment state

Requires sudo. Use this for a completely fresh start.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("clean requires root privileges.\n\nRun: sudo cilo clean")
		}

		fmt.Println("Removing all cilo state...")

		fmt.Print("  → Killing tunnel processes... ")
		exec.Command("pkill", "-9", "-f", "cilo tunnel").Run()
		fmt.Println("done")

		fmt.Print("  → Removing tunnel state... ")
		tunnelDir, _ := tunnel.DaemonDir()
		os.RemoveAll(tunnelDir)
		fmt.Println("done")

		home, _ := os.UserHomeDir()
		ciloDir := home + "/.cilo"

		fmt.Print("  → Removing cloud auth... ")
		os.Remove(ciloDir + "/cloud-auth.json")
		fmt.Println("done")

		fmt.Print("  → Removing state... ")
		os.Remove(ciloDir + "/state.json")
		fmt.Println("done")

		fmt.Print("  → Removing DNS config... ")
		os.RemoveAll(ciloDir + "/dns")
		fmt.Println("done")

		fmt.Print("  → Removing local environments... ")
		os.RemoveAll(ciloDir + "/envs")
		fmt.Println("done")

		fmt.Print("  → Flushing DNS cache... ")
		exec.Command("dscacheutil", "-flushcache").Run()
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
		fmt.Println("done")

		fmt.Println("✓ All cilo state removed")
		fmt.Println("\nTo start fresh:")
		fmt.Println("  cilo cloud login --server <url>")
		fmt.Println("  sudo cilo cloud up <name>")
		return nil
	},
}

func init() {
	tunnelCmd.AddCommand(tunnelStatusCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelCleanCmd)
	tunnelCmd.AddCommand(tunnelDaemonCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(cleanCmd)
}

func StartTunnelDaemon(cfg *tunnel.DaemonConfig) error {
	if err := tunnel.SaveDaemonConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	logDir, _ := tunnel.DaemonDir()
	os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(
		logDir+"/daemon.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	cmd := exec.Command("sudo", "-b", executable, "tunnel", "daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}
	logFile.Close()

	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		state, err := tunnel.LoadDaemonState()
		if err == nil && state.Running {
			return nil
		}
	}

	return fmt.Errorf("daemon failed to start within 3 seconds")
}
