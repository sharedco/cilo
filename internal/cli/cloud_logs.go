// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cloudLogsCmd = &cobra.Command{
	Use:   "logs <name> [service]",
	Short: "View logs from a cloud environment",
	Long: `View logs from a cloud environment.

Without a service name, shows aggregated logs from all services.
With a service name, shows logs from that specific service.

Examples:
  cilo cloud logs agent-1           # All services
  cilo cloud logs agent-1 api       # Just the api service
  cilo cloud logs agent-1 -f        # Follow logs`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCloudLogs,
}

func init() {
	cloudLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	cloudLogsCmd.Flags().Int("tail", 100, "Number of lines to show")
	cloudCmd.AddCommand(cloudLogsCmd)
}

func runCloudLogs(cmd *cobra.Command, args []string) error {
	envName := args[0]
	service := ""
	if len(args) > 1 {
		service = args[1]
	}

	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetInt("tail")

	fmt.Printf("Logs for %s", envName)
	if service != "" {
		fmt.Printf("/%s", service)
	}
	fmt.Printf(" (tail=%d, follow=%v)\n", tail, follow)

	// TODO: Load cloud auth
	// TODO: Connect to environment via WireGuard
	// TODO: GET http://agent/environment/logs/:service
	// TODO: Stream logs to stdout

	return fmt.Errorf("not implemented")
}
