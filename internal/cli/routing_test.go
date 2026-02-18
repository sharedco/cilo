// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/spf13/cobra"
)

// ============================================================================
// Test: Up command with --on flag routes to cilod client
// ============================================================================
func TestUpWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "up"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().Bool("build", false, "Build images")
	cmd.Flags().Bool("recreate", false, "Recreate containers")
	cmd.Flags().StringSlice("shared", []string{}, "Shared services")
	cmd.Flags().StringSlice("isolate", []string{}, "Isolate services")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}

	if remoteTarget.Client == nil {
		t.Error("Expected non-nil cilod client")
	}
}

// ============================================================================
// Test: Down command with --on flag routes to cilod client
// ============================================================================
func TestDownWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "down"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("project", "", "Project name")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}
}

// ============================================================================
// Test: Destroy command with --on flag routes to cilod client
// ============================================================================
func TestDestroyWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "destroy"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().Bool("keep-workspace", false, "Keep workspace")
	cmd.Flags().Bool("force", false, "Force destroy")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}
}

// ============================================================================
// Test: Run command with --on flag routes to cilod client
// ============================================================================
func TestRunWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("from", "", "Source path")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().Bool("no-up", false, "Don't start environment")
	cmd.Flags().Bool("no-create", false, "Don't create if missing")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}
}

// ============================================================================
// Test: --on flag with unknown machine returns "not connected" error
// ============================================================================
func TestOnFlagNotConnected(t *testing.T) {
	// Create a mock command with --on flag pointing to unknown machine
	cmd := &cobra.Command{Use: "up"}
	cmd.Flags().String("on", "", "Target machine")

	// Set the --on flag to an unknown machine
	cmd.Flags().Set("on", "unknown-machine")

	// Test resolveTarget - should return error
	_, err := resolveTarget(cmd)
	if err == nil {
		t.Fatal("Expected error for unknown machine, got nil")
	}

	// Verify error message contains "not connected" and suggests "cilo connect"
	errStr := err.Error()
	if !containsSubstring(errStr, "not connected") && !containsSubstring(errStr, "not found") {
		t.Errorf("Error message should contain 'not connected' or 'not found', got: %s", errStr)
	}

	if !containsSubstring(errStr, "cilo connect") {
		t.Errorf("Error message should suggest 'cilo connect', got: %s", errStr)
	}
}

// ============================================================================
// Test: Commands without --on flag use local target (regression test)
// ============================================================================
func TestCommandsWithoutOnFlag(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{
			name: "up without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "up"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				c.Flags().Bool("build", false, "Build images")
				c.Flags().Bool("recreate", false, "Recreate containers")
				return c
			}(),
		},
		{
			name: "down without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "down"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				return c
			}(),
		},
		{
			name: "destroy without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "destroy"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				c.Flags().Bool("force", false, "Force destroy")
				return c
			}(),
		},
		{
			name: "run without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "run"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				c.Flags().Bool("no-up", false, "Don't start")
				return c
			}(),
		},
		{
			name: "logs without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "logs"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				c.Flags().Bool("follow", false, "Follow logs")
				return c
			}(),
		},
		{
			name: "exec without --on",
			cmd: func() *cobra.Command {
				c := &cobra.Command{Use: "exec"}
				c.Flags().String("on", "", "Target machine")
				c.Flags().String("project", "", "Project name")
				c.Flags().Bool("interactive", true, "Interactive")
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't set --on flag - should default to local

			target, err := resolveTarget(tt.cmd)
			if err != nil {
				t.Fatalf("resolveTarget failed: %v", err)
			}

			// Verify it's a local target
			_, ok := target.(LocalTarget)
			if !ok {
				t.Fatalf("Expected LocalTarget when --on not specified, got %T", target)
			}
		})
	}
}

// ============================================================================
// Test: Logs command with --on flag
// ============================================================================
func TestLogsWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "logs"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().Bool("follow", false, "Follow logs")
	cmd.Flags().Int("tail", 100, "Number of lines")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}
}

// ============================================================================
// Test: Exec command with --on flag
// ============================================================================
func TestExecWithOnFlag(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "test-machine")
	defer cleanupMockMachine(machineDir)

	// Create a mock command with --on flag
	cmd := &cobra.Command{Use: "exec"}
	cmd.Flags().String("on", "", "Target machine")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().Bool("interactive", true, "Interactive")
	cmd.Flags().Bool("tty", true, "Allocate TTY")

	// Set the --on flag
	cmd.Flags().Set("on", "test-machine")

	// Test resolveTarget
	target, err := resolveTarget(cmd)
	if err != nil {
		t.Fatalf("resolveTarget failed: %v", err)
	}

	// Verify it's a remote target
	remoteTarget, ok := target.(RemoteTarget)
	if !ok {
		t.Fatalf("Expected RemoteTarget, got %T", target)
	}

	if remoteTarget.Machine != "test-machine" {
		t.Errorf("Expected machine 'test-machine', got '%s'", remoteTarget.Machine)
	}
}

// ============================================================================
// Test: Target interface methods
// ============================================================================
func TestLocalTarget(t *testing.T) {
	target := LocalTarget{}

	if target.IsRemote() {
		t.Error("LocalTarget.IsRemote() should return false")
	}

	if target.GetMachine() != "" {
		t.Errorf("LocalTarget.GetMachine() should return empty string, got '%s'", target.GetMachine())
	}

	if target.GetClient() != nil {
		t.Error("LocalTarget.GetClient() should return nil")
	}
}

func TestRemoteTarget(t *testing.T) {
	client := cilod.NewClient("http://localhost:8080", "test-token")
	target := RemoteTarget{
		Machine: "test-machine",
		Client:  client,
	}

	if !target.IsRemote() {
		t.Error("RemoteTarget.IsRemote() should return true")
	}

	if target.GetMachine() != "test-machine" {
		t.Errorf("RemoteTarget.GetMachine() should return 'test-machine', got '%s'", target.GetMachine())
	}

	if target.GetClient() != client {
		t.Error("RemoteTarget.GetClient() should return the client")
	}
}

// ============================================================================
// Test: GetMachine function (machine state lookup)
// ============================================================================
func TestGetMachine(t *testing.T) {
	// Setup: Create a mock machine state
	machineDir := setupMockMachine(t, "existing-machine")
	defer cleanupMockMachine(machineDir)

	// Test getting existing machine
	machine, err := GetMachine("existing-machine")
	if err != nil {
		t.Fatalf("GetMachine failed for existing machine: %v", err)
	}

	if machine == nil {
		t.Fatal("GetMachine returned nil for existing machine")
	}

	if machine.Host != "existing-machine" {
		t.Errorf("Expected host 'existing-machine', got '%s'", machine.Host)
	}

	// Test getting non-existent machine
	machine, err = GetMachine("non-existent-machine")
	if err != nil {
		t.Errorf("GetMachine should not return error for non-existent machine, got: %v", err)
	}

	if machine != nil {
		t.Error("GetMachine should return nil for non-existent machine")
	}
}

// ============================================================================
// Test: ListConnectedMachines function
// ============================================================================
func TestListConnectedMachines(t *testing.T) {
	// Setup: Create mock machine states
	machineDir1 := setupMockMachine(t, "machine-1")
	defer cleanupMockMachine(machineDir1)

	machineDir2 := setupMockMachine(t, "machine-2")
	defer cleanupMockMachine(machineDir2)

	// Test listing machines
	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}

	// Should find at least our 2 mock machines
	foundMachine1 := false
	foundMachine2 := false
	for _, m := range machines {
		if m.Host == "machine-1" {
			foundMachine1 = true
		}
		if m.Host == "machine-2" {
			foundMachine2 = true
		}
	}

	if !foundMachine1 {
		t.Error("ListConnectedMachines should include 'machine-1'")
	}

	if !foundMachine2 {
		t.Error("ListConnectedMachines should include 'machine-2'")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func setupMockMachine(t *testing.T, host string) string {
	t.Helper()

	// Get the cilo home directory
	ciloHome := os.Getenv("HOME")
	if ciloHome == "" {
		t.Skip("HOME environment variable not set")
	}

	machinesDir := filepath.Join(ciloHome, ".cilo", "machines", host)

	// Create the machine directory
	if err := os.MkdirAll(machinesDir, 0755); err != nil {
		t.Fatalf("Failed to create mock machine directory: %v", err)
	}

	// Create a mock state.json
	stateData := fmt.Sprintf(`{
		"host": "%s",
		"token": "test-token",
		"wg_ip": "10.225.0.1",
		"connected_at": "2026-01-01T00:00:00Z"
	}`, host)

	statePath := filepath.Join(machinesDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateData), 0644); err != nil {
		t.Fatalf("Failed to write mock state file: %v", err)
	}

	return machinesDir
}

func cleanupMockMachine(machineDir string) {
	// Remove the entire machine directory
	parentDir := filepath.Dir(machineDir)
	os.RemoveAll(parentDir)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
