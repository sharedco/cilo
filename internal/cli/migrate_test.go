// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCloudCommandShowsMigrationMessage asserts that 'cilo cloud' now shows
// a migration message explaining that cloud commands have been removed.
func TestCloudCommandShowsMigrationMessage(t *testing.T) {
	// Create a fresh root command for testing
	cmd := &cobra.Command{
		Use:   "cilo",
		Short: "cilo - isolated workspace environments for AI agents",
	}

	// Add the migration command (which is registered as "cloud")
	cmd.AddCommand(migrateCmd)

	// Set args to simulate "cilo cloud" command
	cmd.SetArgs([]string{"cloud"})

	// Execute the command
	err := cmd.Execute()

	// Assert that an error is returned (the migration command returns an error)
	if err == nil {
		t.Fatal("expected error when running 'cilo cloud', got nil")
	}

	// Assert that the error message indicates cloud commands were removed
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "removed") &&
		!strings.Contains(strings.ToLower(errMsg), "cloud commands") {
		t.Fatalf("expected error message about cloud commands being removed, got: %s", errMsg)
	}
}

// TestNoCloudUpSubcommand asserts that 'cilo cloud up' returns an unknown command error
// because cloud subcommands have been removed.
func TestNoCloudUpSubcommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "cilo",
		Short: "cilo - isolated workspace environments for AI agents",
	}

	// Add the migration command (registered as "cloud")
	cmd.AddCommand(migrateCmd)

	cmd.SetArgs([]string{"cloud", "up", "test-env"})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error when running 'cilo cloud up', got nil")
	}

	// Should get "unknown command" for "up" since the migration command
	// doesn't have subcommands
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "unknown") &&
		!strings.Contains(strings.ToLower(errMsg), "not found") &&
		!strings.Contains(strings.ToLower(errMsg), "invalid") &&
		!strings.Contains(strings.ToLower(errMsg), "removed") {
		t.Fatalf("expected error message to contain 'unknown', 'not found', 'invalid', or 'removed', got: %s", errMsg)
	}
}

// TestLocalCommandsStillWork verifies that local commands are still registered
func TestLocalCommandsStillWork(t *testing.T) {
	// Get all commands from the real root command
	commands := rootCmd.Commands()

	// Build a map of command names
	cmdMap := make(map[string]bool)
	for _, cmd := range commands {
		cmdMap[cmd.Name()] = true
	}

	// Verify local commands exist
	localCommands := []string{"init", "setup", "config", "create", "up", "down", "destroy", "list", "status", "logs", "exec", "path", "compose", "dns", "hostnames", "diff", "merge", "network"}
	for _, name := range localCommands {
		if !cmdMap[name] {
			t.Errorf("expected local command '%s' to be registered", name)
		}
	}

	// The cloud command should now be the migration command
	if !cmdMap["cloud"] {
		t.Error("cloud command should be registered (as migration command)")
	}
}

// TestCloudCommandIsHidden verifies that the cloud command is hidden from help
func TestCloudCommandIsHidden(t *testing.T) {
	if !migrateCmd.Hidden {
		t.Error("cloud migration command should be hidden from help")
	}
}
