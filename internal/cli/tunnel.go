// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"
	"os/exec"
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

func init() {
	tunnelCmd.AddCommand(tunnelStatusCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelDaemonCmd)
	rootCmd.AddCommand(tunnelCmd)
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
