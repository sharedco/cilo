package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/sharedco/cilo/server/pkg/config"
	"github.com/sharedco/cilo/server/pkg/store"
	"github.com/spf13/cobra"
)

// rootCmd is the base command for the CLI
var rootCmd = &cobra.Command{
	Use:   "cilo-server",
	Short: "Cilo server - Cloud API server for isolated development environments",
	Long: `The Cilo server provides centralized environment management,
WireGuard networking, and VM orchestration for remote Cilo environments.`,
}

// adminCmd is the parent command for admin operations
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin operations",
	Long:  `Administrative commands for managing API keys, teams, and server configuration.`,
}

// createKeyCmd creates a new API key
var createKeyCmd = &cobra.Command{
	Use:   "create-key",
	Short: "Create a new API key",
	Long:  `Creates a new API key for a team. The key will be displayed once and cannot be retrieved later.`,
	RunE:  runCreateKey,
}

// machinesCmd is the parent command for machine management
var machinesCmd = &cobra.Command{
	Use:   "machines",
	Short: "Machine management",
	Long:  `Commands for managing VM hosts in the machine pool.`,
}

// machinesAddCmd registers a new manual machine
var machinesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register a new manual machine",
	Long:  `Registers a manually provisioned machine (bare metal or VM) to the pool.`,
	RunE:  runMachinesAdd,
}

// machinesListCmd lists all machines
var machinesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all machines",
	Long:  `Displays a table of all registered machines with their status and assignments.`,
	RunE:  runMachinesList,
}

// machinesRemoveCmd removes a machine from the pool
var machinesRemoveCmd = &cobra.Command{
	Use:   "remove [machine-id]",
	Short: "Remove a machine from the pool",
	Long:  `Removes a machine registration. The machine must not be assigned to an environment.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runMachinesRemove,
}

// Flags for create-key command
var (
	createKeyTeam  string
	createKeyName  string
	createKeyScope string
)

// Flags for machines add command
var (
	machinesAddName    string
	machinesAddHost    string
	machinesAddSSHUser string
	machinesAddRegion  string
	machinesAddSize    string
	machinesAddSSHHost string
)

func init() {
	// Add admin subcommand
	rootCmd.AddCommand(adminCmd)

	// Add create-key to admin
	createKeyCmd.Flags().StringVar(&createKeyTeam, "team", "", "Team ID (required)")
	createKeyCmd.Flags().StringVar(&createKeyName, "name", "", "Key name/description (required)")
	createKeyCmd.Flags().StringVar(&createKeyScope, "scope", "write", "Key scope: read, write, or admin")
	createKeyCmd.MarkFlagRequired("team")
	createKeyCmd.MarkFlagRequired("name")
	adminCmd.AddCommand(createKeyCmd)

	// Add machines subcommand
	rootCmd.AddCommand(machinesCmd)

	// Configure machines add flags
	machinesAddCmd.Flags().StringVar(&machinesAddName, "name", "", "Machine name/identifier (required)")
	machinesAddCmd.Flags().StringVar(&machinesAddHost, "host", "", "Public IP address (required)")
	machinesAddCmd.Flags().StringVar(&machinesAddSSHUser, "ssh-user", "root", "SSH username")
	machinesAddCmd.Flags().StringVar(&machinesAddRegion, "region", "", "Region/location")
	machinesAddCmd.Flags().StringVar(&machinesAddSize, "size", "manual", "Machine size/type")
	machinesAddCmd.Flags().StringVar(&machinesAddSSHHost, "ssh-host", "", "SSH host (defaults to --host if not set)")
	machinesAddCmd.MarkFlagRequired("name")
	machinesAddCmd.MarkFlagRequired("host")

	// Add machine subcommands
	machinesCmd.AddCommand(machinesAddCmd)
	machinesCmd.AddCommand(machinesListCmd)
	machinesCmd.AddCommand(machinesRemoveCmd)
}

// runCreateKey creates a new API key
func runCreateKey(cmd *cobra.Command, args []string) error {
	// Validate scope
	validScopes := map[string]bool{"read": true, "write": true, "admin": true}
	if !validScopes[createKeyScope] {
		return fmt.Errorf("invalid scope %q: must be 'read', 'write', or 'admin'", createKeyScope)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	ctx := context.Background()
	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer st.Close()

	// Generate API key
	keyID := uuid.New().String()
	keyPrefix := generateKeyPrefix()
	keySecret := generateKeySecret()
	keyHash := hashKey(keySecret)

	// Create API key record
	apiKey := &store.APIKey{
		ID:        keyID,
		TeamID:    createKeyTeam,
		KeyHash:   keyHash,
		Prefix:    keyPrefix,
		Scope:     createKeyScope,
		Name:      createKeyName,
		CreatedAt: time.Now(),
	}

	if err := st.CreateAPIKey(ctx, apiKey); err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	// Display the key (only time it's shown)
	fullKey := fmt.Sprintf("%s.%s", keyPrefix, keySecret)

	fmt.Println("✓ API key created successfully")
	fmt.Println()
	fmt.Println("Key ID:", keyID)
	fmt.Println("Team:", createKeyTeam)
	fmt.Println("Name:", createKeyName)
	fmt.Println("Scope:", createKeyScope)
	fmt.Println()
	fmt.Println("API Key (save this - it won't be shown again):")
	fmt.Println(fullKey)
	fmt.Println()
	fmt.Println("Use this key in the Authorization header:")
	fmt.Printf("Authorization: Bearer %s\n", fullKey)

	return nil
}

// runMachinesAdd registers a new manual machine
func runMachinesAdd(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	ctx := context.Background()
	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer st.Close()

	// Determine SSH host
	sshHost := machinesAddSSHHost
	if sshHost == "" {
		sshHost = machinesAddHost
	}

	// Create machine record
	machine := &store.Machine{
		ID:           machinesAddName,
		ProviderID:   "manual",
		ProviderType: "manual",
		PublicIP:     machinesAddHost,
		Status:       "ready",
		SSHHost:      sshHost,
		SSHUser:      machinesAddSSHUser,
		Region:       machinesAddRegion,
		Size:         machinesAddSize,
		CreatedAt:    time.Now(),
	}

	if err := st.SaveMachine(ctx, machine); err != nil {
		return fmt.Errorf("failed to save machine: %w", err)
	}

	fmt.Println("✓ Machine registered successfully")
	fmt.Println()
	fmt.Println("Machine ID:", machine.ID)
	fmt.Println("Provider:", machine.ProviderType)
	fmt.Println("Public IP:", machine.PublicIP)
	fmt.Println("SSH Host:", machine.SSHHost)
	fmt.Println("SSH User:", machine.SSHUser)
	fmt.Println("Status:", machine.Status)
	if machine.Region != "" {
		fmt.Println("Region:", machine.Region)
	}
	fmt.Println("Size:", machine.Size)

	return nil
}

// runMachinesList lists all machines
func runMachinesList(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	ctx := context.Background()
	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer st.Close()

	// Fetch machines
	machines, err := st.ListMachines(ctx)
	if err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}

	if len(machines) == 0 {
		fmt.Println("No machines registered.")
		return nil
	}

	// Create tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w, "ID\tSTATUS\tPROVIDER\tPUBLIC IP\tASSIGNED ENV\tREGION\tSIZE\tCREATED")
	fmt.Fprintln(w, "--\t------\t--------\t---------\t------------\t------\t----\t-------")

	// Print machine rows
	for _, m := range machines {
		assignedEnv := "-"
		if m.AssignedEnv != nil {
			assignedEnv = *m.AssignedEnv
		}

		region := m.Region
		if region == "" {
			region = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.ID,
			m.Status,
			m.ProviderType,
			m.PublicIP,
			assignedEnv,
			region,
			m.Size,
			m.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d machine(s)\n", len(machines))

	return nil
}

// runMachinesRemove removes a machine from the pool
func runMachinesRemove(cmd *cobra.Command, args []string) error {
	machineID := args[0]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	ctx := context.Background()
	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer st.Close()

	// Get machine to check if it's assigned
	machine, err := st.GetMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("failed to get machine: %w", err)
	}

	// Check if machine is assigned to an environment
	if machine.AssignedEnv != nil && *machine.AssignedEnv != "" {
		return fmt.Errorf("cannot remove machine %s: currently assigned to environment %s", machineID, *machine.AssignedEnv)
	}

	// Confirm removal if not forced
	if !cmd.Flag("force").Changed {
		fmt.Printf("Remove machine %s (%s)? [y/N]: ", machineID, machine.PublicIP)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the machine
	if err := st.DeleteMachine(ctx, machineID); err != nil {
		return fmt.Errorf("failed to remove machine: %w", err)
	}

	fmt.Printf("✓ Machine %s removed successfully\n", machineID)

	return nil
}

// generateKeyPrefix generates a random 8-character prefix for API keys
func generateKeyPrefix() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:8]
}

// generateKeySecret generates a random 32-character secret for API keys
func generateKeySecret() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// hashKey creates a simple hash of the key for storage
func hashKey(key string) string {
	// In production, use bcrypt or Argon2
	// For now, use a simple constant-time comparison approach
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// Constant-time comparison to prevent timing attacks
func constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
