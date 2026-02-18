// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:    "cloud",
	Short:  "Cloud commands have been removed",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(`╔════════════════════════════════════════════════════════════════╗`)
		fmt.Println(`║  CLOUD COMMANDS REMOVED - ARCHITECTURE CLEANUP COMPLETE       ║`)
		fmt.Println(`╚════════════════════════════════════════════════════════════════╝`)
		fmt.Println()
		fmt.Println("The 'cilo cloud' command namespace has been removed from cilo.")
		fmt.Println()
		fmt.Println("WHAT HAPPENED:")
		fmt.Println("  • The central server architecture has been discontinued")
		fmt.Println("  • All cloud commands (login, up, down, destroy, status, logs, connect)")
		fmt.Println("    have been removed from the CLI")
		fmt.Println("  • The server-managed cloud environments are no longer supported")
		fmt.Println()
		fmt.Println("WHAT'S NEXT:")
		fmt.Println("  • cilo now focuses exclusively on local Docker-based isolation")
		fmt.Println("  • Use 'cilo up', 'cilo down', 'cilo destroy' for local environments")
		fmt.Println("  • A new peer-to-peer architecture (cilod) is planned for future releases")
		fmt.Println()
		fmt.Println("CLEANUP ACTIONS:")
		fmt.Println("  1. Remove any cloud server references from your scripts")
		fmt.Println("  2. Clear cloud credentials: rm ~/.cilo/cloud-credentials.json")
		fmt.Println("  3. Update CI/CD pipelines that used 'cilo cloud' commands")
		fmt.Println()
		fmt.Println("LOCAL COMMANDS (still available):")
		fmt.Println("  cilo up <env>       - Create and start local environment")
		fmt.Println("  cilo down <env>     - Stop local environment")
		fmt.Println("  cilo destroy <env>  - Remove local environment")
		fmt.Println("  cilo list           - List all environments")
		fmt.Println("  cilo status         - Show environment status")
		fmt.Println("  cilo logs <env>     - View environment logs")
		fmt.Println()
		fmt.Println("For more information, see: https://github.com/sharedco/cilo")
		fmt.Println()
		return fmt.Errorf("cloud commands have been removed - see message above")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
