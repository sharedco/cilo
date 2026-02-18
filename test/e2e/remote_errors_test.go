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

// TestOnFlagWithoutConnect tests using --on without prior connect
func TestOnFlagWithoutConnect(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up any existing connections
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	// Create a test project
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
	envName := "error-test-" + timestamp
	projectName := "error-project"
	machineName := "unconnected-machine.example.com"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test up with --on but no connect
	t.Run("UpOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", envName, "--on", machineName, "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected up with --on to fail when machine not connected")
		}

		outputStr := string(output)
		// Should have clear error message about needing to connect first
		if !strings.Contains(outputStr, "not connected") {
			t.Errorf("expected 'not connected' error message, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error to mention 'connect' command, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})

	// Test down with --on but no connect
	t.Run("DownOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "down", envName, "--on", machineName, "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected down with --on to fail when machine not connected")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not connected, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})

	// Test destroy with --on but no connect
	t.Run("DestroyOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "destroy", envName, "--on", machineName, "--force", "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected destroy with --on to fail when machine not connected")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not connected, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})

	// Test run with --on but no connect
	t.Run("RunOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "run", "echo", envName, "--on", machineName, "--from", dir, "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected run with --on to fail when machine not connected")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not connected, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})

	// Test logs with --on but no connect
	t.Run("LogsOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "logs", envName, "--on", machineName, "--tail", "10", "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected logs with --on to fail when machine not connected")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not connected, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})

	// Test exec with --on but no connect
	t.Run("ExecOnWithoutConnect", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "exec", envName, "app", "echo", "test", "--on", machineName, "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected exec with --on to fail when machine not connected")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not connected, got: %s", outputStr)
		}
		t.Logf("Error message: %s", outputStr)
	})
}

// TestConnectToUnreachableHost tests connecting to an unreachable host
func TestConnectToUnreachableHost(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	unreachableHosts := []string{
		"192.0.2.1:8080",               // TEST-NET-1, should be unreachable
		"198.51.100.1:8080",            // TEST-NET-2, should be unreachable
		"203.0.113.1:8080",             // TEST-NET-3, should be unreachable
		"unreachable.example.com:8080", // Likely unreachable
	}

	for _, host := range unreachableHosts {
		t.Run("ConnectTo_"+strings.ReplaceAll(host, ":", "_"), func(t *testing.T) {
			cmd := exec.Command(ciloBinary, "connect", host)
			output, err := cmd.CombinedOutput()

			// Should fail
			if err == nil {
				t.Errorf("expected connect to %s to fail", host)
			}

			outputStr := string(output)
			// Should have meaningful error
			if !strings.Contains(outputStr, "failed") && !strings.Contains(outputStr, "error") &&
				!strings.Contains(outputStr, "authentication") && !strings.Contains(outputStr, "connection") &&
				!strings.Contains(outputStr, "timeout") && !strings.Contains(outputStr, "refused") {
				t.Errorf("expected meaningful error message for unreachable host, got: %s", outputStr)
			}
			t.Logf("Connect to %s error (expected): %s", host, outputStr)
		})
	}
}

// TestOperationsOnDisconnectedMachine tests operations on a disconnected machine
func TestOperationsOnDisconnectedMachine(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	disconnectedMachine := "disconnected-test-machine.example.com"

	// Create a test project
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
	envName := "disconnected-test-" + timestamp
	projectName := "disconnected-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test all operations on disconnected machine
	operations := []struct {
		name string
		args []string
	}{
		{"up", []string{"up", envName, "--on", disconnectedMachine, "--project", projectName}},
		{"down", []string{"down", envName, "--on", disconnectedMachine, "--project", projectName}},
		{"destroy", []string{"destroy", envName, "--on", disconnectedMachine, "--force", "--project", projectName}},
		{"logs", []string{"logs", envName, "--on", disconnectedMachine, "--tail", "10", "--project", projectName}},
		{"exec", []string{"exec", envName, "app", "echo", "test", "--on", disconnectedMachine, "--project", projectName}},
		{"run", []string{"run", "echo", envName, "--on", disconnectedMachine, "--from", dir, "--project", projectName}},
	}

	for _, op := range operations {
		t.Run(op.name+"OnDisconnected", func(t *testing.T) {
			cmd := exec.Command(ciloBinary, op.args...)
			if op.name == "run" {
				cmd.Dir = dir
			}
			output, err := cmd.CombinedOutput()

			// All should fail
			if err == nil {
				t.Errorf("expected %s on disconnected machine to fail", op.name)
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
				t.Errorf("expected error about machine not connected for %s, got: %s", op.name, outputStr)
			}
			t.Logf("%s on disconnected machine error (expected): %s", op.name, outputStr)
		})
	}
}

// TestConnectWithInvalidHost tests connect with invalid host formats
func TestConnectWithInvalidHost(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	invalidHosts := []string{
		"",                    // Empty host
		":8080",               // Missing host
		"not-a-valid-host!!!", // Invalid characters
	}

	for _, host := range invalidHosts {
		if host == "" {
			// Skip empty host test - cobra will handle missing arg
			continue
		}
		t.Run("ConnectWithInvalidHost_"+strings.ReplaceAll(host, ":", "_"), func(t *testing.T) {
			cmd := exec.Command(ciloBinary, "connect", host)
			output, err := cmd.CombinedOutput()

			// Should fail
			if err == nil {
				t.Errorf("expected connect with invalid host '%s' to fail", host)
			}

			outputStr := string(output)
			t.Logf("Connect with invalid host '%s' output: %s", host, outputStr)
		})
	}
}

// TestDoubleConnect tests connecting to already connected machine
func TestDoubleConnect(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	// We can't test this without a real connected machine
	// But we can verify the command structure
	t.Run("ConnectCommandHelp", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "connect", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("connect --help failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "connect") {
			t.Errorf("expected 'connect' in help output")
		}
		t.Logf("Connect help: %s", outputStr)
	})
}

// TestErrorMessagesAreClear tests that error messages are user-friendly
func TestErrorMessagesAreClear(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Clean up first
	disconnectCmd := exec.Command(ciloBinary, "disconnect")
	disconnectCmd.Run()

	t.Run("OnFlagErrorMessage", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", "test-env", "--on", "nonexistent-machine")
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected error for --on with nonexistent machine")
		}

		outputStr := string(output)
		// Error message should be clear and actionable
		if !strings.Contains(outputStr, "not connected") {
			t.Errorf("error message should say machine is 'not connected', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "cilo connect") {
			t.Errorf("error message should suggest 'cilo connect', got: %s", outputStr)
		}
		t.Logf("Clear error message: %s", outputStr)
	})

	t.Run("ConnectErrorMessage", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "connect", "192.0.2.1:8080")
		output, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Errorf("expected error for connect to unreachable host")
		}

		outputStr := string(output)
		// Error should indicate connection/authentication failure
		if !strings.Contains(outputStr, "failed") && !strings.Contains(outputStr, "error") &&
			!strings.Contains(outputStr, "authentication") && !strings.Contains(outputStr, "connection") {
			t.Errorf("error message should indicate failure, got: %s", outputStr)
		}
		t.Logf("Connect error message: %s", outputStr)
	})
}

// TestLocalCommandsIgnoreOnFlag tests that local commands work even with --on flag issues
func TestLocalCommandsIgnoreOnFlag(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create a test project
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
	envName := "local-ignore-test-" + timestamp
	projectName := "local-ignore-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test that local commands work without --on flag
	t.Run("LocalCreateWithoutOn", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create without --on failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "created") {
			t.Errorf("expected 'created' in output, got: %s", output)
		}
		t.Logf("Local create succeeded: %s", output)
	})

	// Test that local up works without --on flag
	t.Run("LocalUpWithoutOn", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", envName, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up without --on failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "running") {
			t.Errorf("expected 'running' in output, got: %s", output)
		}
		t.Logf("Local up succeeded: %s", output)
	})

	// Verify local environment exists
	t.Run("VerifyLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list", "--project", projectName)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("environment %q not found in list", envName)
		}
		t.Logf("Local environment verified: %s", output)
	})
}
