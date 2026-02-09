// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cloudStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show status of cloud environments",
	Long: `Show status of cloud environments.

Without arguments, lists all environments for the current team.
With a name argument, shows detailed status of that environment.

Examples:
  cilo cloud status           # List all
  cilo cloud status agent-1   # Show details for agent-1`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCloudStatus,
}

func init() {
	cloudStatusCmd.Flags().Bool("json", false, "Output as JSON")
	cloudCmd.AddCommand(cloudStatusCmd)
}

func runCloudStatus(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	if len(args) == 0 {
		// List all environments
		fmt.Println("Cloud Environments:")
		fmt.Println("  (none)")

		// TODO: Load cloud auth
		// TODO: GET /v1/environments
		// TODO: Format output (table or JSON)
	} else {
		envName := args[0]
		fmt.Printf("Environment: %s\n", envName)
		fmt.Printf("  Status: unknown\n")

		// TODO: GET /v1/environments/:id
		// TODO: Show detailed status including services, IPs, peers
	}

	if jsonOutput {
		fmt.Println("{}")
	}

	return nil
}
