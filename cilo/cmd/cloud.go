package cmd

import (
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage remote cloud environments",
	Long: `Cilo Cloud enables running isolated environments on remote infrastructure.

Use 'cilo cloud login' to authenticate with a Cilo server.
Then use 'cilo cloud up' to create and start remote environments.

Examples:
  cilo cloud login --server https://cilo.internal.company.com
  cilo cloud up agent-1 --from .
  cilo cloud status
  cilo cloud logs agent-1
  cilo cloud destroy agent-1`,
}

func init() {
	rootCmd.AddCommand(cloudCmd)
}
