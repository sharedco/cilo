package cmd

import (
	"fmt"

	"github.com/sharedco/cilo/pkg/cloud"
	"github.com/spf13/cobra"
)

var cloudLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Cilo Cloud",
	Long: `Remove stored Cilo Cloud authentication credentials.

This command removes the API key and server URL stored in ~/.cilo/cloud-auth.json.
After logging out, you'll need to run 'cilo cloud login' again to use cloud features.

Examples:
  cilo cloud logout`,
	RunE: runCloudLogout,
}

func init() {
	cloudCmd.AddCommand(cloudLogoutCmd)
}

func runCloudLogout(cmd *cobra.Command, args []string) error {
	if !cloud.IsLoggedIn() {
		fmt.Println("Not currently logged in to any Cilo server.")
		return nil
	}

	server, err := cloud.GetServerURL()
	if err != nil {
		server = "unknown server"
	}

	if err := cloud.ClearAuth(); err != nil {
		return fmt.Errorf("failed to clear credentials: %w", err)
	}

	fmt.Printf("✓ Logged out from %s\n", server)
	return nil
}
