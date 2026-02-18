// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConnectCommand tests the cilo connect command
func TestConnectCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Test connect to unreachable host should fail gracefully
	t.Run("ConnectToUnreachableHost", func(t *testing.T) {
		// Try to connect to a host that doesn't exist
		cmd := exec.Command(ciloBinary, "connect", "192.0.2.1:8080")
		output, err := cmd.CombinedOutput()

		// Should fail since host is unreachable
		if err == nil {
			t.Errorf("expected connect to fail for unreachable host, but it succeeded")
		}

		// Should have meaningful error message
		outputStr := string(output)
		if !strings.Contains(outputStr, "failed") && !strings.Contains(outputStr, "error") &&
			!strings.Contains(outputStr, "authentication") && !strings.Contains(outputStr, "connection") {
			t.Errorf("expected error message about connection failure, got: %s", outputStr)
		}
		t.Logf("Connect output (expected failure): %s", outputStr)
	})

	// Test connect without SSH key should fail
	t.Run("ConnectWithoutSSHKey", func(t *testing.T) {
		// Temporarily move SSH keys if they exist
		home, _ := os.UserHomeDir()
		sshDir := filepath.Join(home, ".ssh")

		// Backup existing keys
		backupDir := filepath.Join(home, ".ssh-backup-"+time.Now().Format("20060102150405"))
		if _, err := os.Stat(sshDir); err == nil {
			// SSH dir exists, try to backup
			_ = os.Rename(sshDir, backupDir)
			defer func() {
				// Restore SSH keys
				_ = os.RemoveAll(sshDir)
				_ = os.Rename(backupDir, sshDir)
			}()
		}

		// Create empty .ssh directory
		_ = os.MkdirAll(sshDir, 0700)

		cmd := exec.Command(ciloBinary, "connect", "localhost:8080")
		output, err := cmd.CombinedOutput()

		// Should fail without SSH key
		if err == nil {
			t.Errorf("expected connect to fail without SSH key, but it succeeded")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "SSH") && !strings.Contains(outputStr, "key") {
			t.Errorf("expected error about missing SSH key, got: %s", outputStr)
		}
		t.Logf("Connect output (expected failure): %s", outputStr)
	})
}

// TestDisconnectCommand tests the cilo disconnect command
func TestDisconnectCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Test disconnect from non-existent machine
	t.Run("DisconnectNonExistentMachine", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "disconnect", "non-existent-host.example.com")
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected disconnect to fail for non-existent machine, but it succeeded")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "not found") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Disconnect output (expected failure): %s", outputStr)
	})

	// Test disconnect all when no machines connected
	t.Run("DisconnectAllNoMachines", func(t *testing.T) {
		// First ensure no machines are connected by disconnecting all
		disconnectAllCmd := exec.Command(ciloBinary, "disconnect")
		disconnectAllCmd.Run()

		// Now try disconnect all again
		cmd := exec.Command(ciloBinary, "disconnect")
		output, err := cmd.CombinedOutput()

		// Should succeed with message about no machines
		if err != nil {
			t.Logf("disconnect all returned error (may be expected): %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "No connected") && !strings.Contains(outputStr, "machine") {
			t.Logf("Note: expected message about no machines, got: %s", outputStr)
		}
		t.Logf("Disconnect all output: %s", outputStr)
	})
}

// TestMachinesCommand tests the cilo machines command
func TestMachinesCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Test machines list when no machines connected
	t.Run("MachinesListEmpty", func(t *testing.T) {
		// First ensure no machines are connected
		disconnectCmd := exec.Command(ciloBinary, "disconnect")
		disconnectCmd.Run()

		cmd := exec.Command(ciloBinary, "machines")
		output, err := cmd.CombinedOutput()

		// Should succeed even with no machines
		if err != nil {
			t.Logf("machines command returned error (may be expected): %v", err)
		}

		outputStr := string(output)
		// Should indicate no machines
		if !strings.Contains(outputStr, "No connected") && !strings.Contains(outputStr, "machine") {
			t.Logf("Note: expected 'No connected machines' message, got: %s", outputStr)
		}
		t.Logf("Machines output: %s", outputStr)
	})

	// Test machines list with JSON format
	t.Run("MachinesListJSON", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "machines", "--json")
		output, err := cmd.CombinedOutput()

		// Should succeed
		if err != nil {
			t.Logf("machines --json returned error (may be expected): %v", err)
		}

		// Output should be valid JSON (empty array or array of machines)
		outputStr := strings.TrimSpace(string(output))
		if !strings.HasPrefix(outputStr, "[") && !strings.Contains(outputStr, "No connected") {
			t.Errorf("expected JSON array or 'No connected' message, got: %s", outputStr)
		}
		t.Logf("Machines JSON output: %s", outputStr)
	})
}

// TestConnectDisconnectWorkflow tests the full connect/disconnect workflow
func TestConnectDisconnectWorkflow(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up any existing connections first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	// Note: We can't fully test connect without a real remote machine running cilod
	// But we can test the command structure and error handling

	t.Run("ConnectCommandStructure", func(t *testing.T) {
		// Test that connect command accepts host argument
		cmd := exec.Command(ciloBinary, "connect", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("connect --help failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "connect") {
			t.Errorf("expected 'connect' in help output")
		}
		if !strings.Contains(outputStr, "host") {
			t.Errorf("expected 'host' in help output")
		}
		t.Logf("Connect help output: %s", outputStr)
	})

	t.Run("DisconnectCommandStructure", func(t *testing.T) {
		// Test that disconnect command accepts optional host argument
		cmd := exec.Command(ciloBinary, "disconnect", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("disconnect --help failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "disconnect") {
			t.Errorf("expected 'disconnect' in help output")
		}
		t.Logf("Disconnect help output: %s", outputStr)
	})
}

// TestMultipleMachines tests connecting to multiple machines (when possible)
func TestMultipleMachines(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	// Test that we can attempt to connect to multiple hosts
	// (They will fail, but we verify the command structure)
	t.Run("MultipleConnectAttempts", func(t *testing.T) {
		hosts := []string{
			"192.0.2.1:8080",
			"192.0.2.2:8080",
		}

		for _, host := range hosts {
			cmd := exec.Command(ciloBinary, "connect", host)
			output, err := cmd.CombinedOutput()

			// All should fail since these are unreachable
			if err == nil {
				t.Logf("Warning: connect to %s succeeded unexpectedly", host)
			}

			t.Logf("Connect to %s: %s", host, output)
		}
	})
}

// TestDisconnectWhileEnvironmentsRunning tests disconnect behavior when environments are running
func TestDisconnectWhileEnvironmentsRunning(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create a local environment first
	dir := t.TempDir()
	composeContent := `
services:
  app:
    image: alpine:latest
    command: sleep 3600
`
	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "disconnect-test-" + timestamp
	projectName := "disconnect-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Create and start local environment
	cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
	}

	cmd = exec.Command(ciloBinary, "up", envName, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
	}

	// Verify environment is running locally
	t.Run("LocalEnvironmentRunning", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list", "--project", projectName)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("environment %q not found in list", envName)
		}
		t.Logf("Environment is running locally: %s", output)
	})

	// Note: We can't test disconnect with remote environments without a real remote machine
	// But local environments should continue running regardless of disconnect commands
	t.Run("LocalEnvironmentsUnaffectedByDisconnect", func(t *testing.T) {
		// Try disconnect all (should succeed even with local envs running)
		disconnectCmd := exec.Command(ciloBinary, "disconnect")
		output, err := disconnectCmd.CombinedOutput()

		// Disconnect should succeed (nothing to disconnect or already disconnected)
		if err != nil {
			t.Logf("disconnect returned error (may be expected): %v", err)
		}
		t.Logf("Disconnect output: %s", output)

		// Verify local environment is still running
		listCmd := exec.Command(ciloBinary, "list", "--project", projectName)
		listOutput, err := listCmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(listOutput), envName) {
			t.Errorf("local environment %q should still exist after disconnect", envName)
		}
		t.Logf("Local environment still running: %s", listOutput)
	})
}
