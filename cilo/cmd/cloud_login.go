package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/sharedco/cilo/pkg/cloud"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var cloudLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a Cilo server",
	Long: `Authenticate with a Cilo Cloud server.

The server URL and API key are stored in ~/.cilo/cloud-auth.json.

Examples:
  # Interactive login
  cilo cloud login --server https://cilo.internal.company.com
  
  # Login with API key from environment
  cilo cloud login --server https://api.cilocloud.dev --stdin
  
  # For CI: set CILO_API_KEY and CILO_SERVER environment variables`,
	RunE: runCloudLogin,
}

func init() {
	cloudLoginCmd.Flags().String("server", "", "Cilo server URL (required)")
	cloudLoginCmd.Flags().Bool("stdin", false, "Read API key from stdin (for CI)")
	cloudLoginCmd.MarkFlagRequired("server")
	cloudCmd.AddCommand(cloudLoginCmd)
}

func runCloudLogin(cmd *cobra.Command, args []string) error {
	server, _ := cmd.Flags().GetString("server")
	stdin, _ := cmd.Flags().GetBool("stdin")

	var apiKey string

	if stdin {
		// Read from stdin
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
		}
	} else {
		// Interactive prompt
		fmt.Print("API Key: ")
		keyBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		fmt.Println()
		apiKey = string(keyBytes)
	}

	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	client := cloud.NewClient(server, apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	authResp, err := client.ValidateAuth(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	auth := &cloud.Auth{
		Server: server,
		APIKey: apiKey,
		TeamID: authResp.TeamID,
	}

	if err := cloud.SaveAuth(auth); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("✓ Logged in to %s", server)
	if authResp.TeamName != "" {
		fmt.Printf(" (team: %s)", authResp.TeamName)
	}
	fmt.Println()
	return nil
}
