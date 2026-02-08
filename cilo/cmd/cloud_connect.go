package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cloudConnectCmd = &cobra.Command{
	Use:   "connect <name>",
	Short: "Connect to an existing cloud environment",
	Long: `Connect to an existing cloud environment created by another user.

This enables multiple developers to access the same environment,
useful for pair programming or reviewing PR preview environments.

The command:
1. Registers your WireGuard public key with the environment
2. Establishes a WireGuard tunnel
3. Configures local DNS

Example:
  cilo cloud connect pr-42`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudConnect,
}

func init() {
	cloudCmd.AddCommand(cloudConnectCmd)
}

func runCloudConnect(cmd *cobra.Command, args []string) error {
	envName := args[0]

	fmt.Printf("Connecting to cloud environment: %s\n", envName)

	// TODO: Load cloud auth
	// TODO: Generate WireGuard key pair
	// TODO: POST /v1/wireguard/exchange
	// TODO: Setup local WireGuard tunnel
	// TODO: Configure local DNS

	return fmt.Errorf("not implemented")
}
