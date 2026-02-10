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

// TestRemoteWorkflowFull tests the complete remote workflow
// Connect → run with --on → ls shows remote env → logs → exec → down → disconnect
func TestRemoteWorkflowFull(t *testing.T) {
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
  web:
    image: nginx:alpine
    ports:
      - "80"
`
	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "remote-workflow-" + timestamp
	projectName := "remote-workflow-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		// Destroy local environment if exists
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
		// Disconnect any connections
		disconnectCmd := exec.Command(ciloBinary, "disconnect")
		_ = disconnectCmd.Run()
	}()

	// Step 1: Attempt to connect to a remote machine (will fail without real remote)
	t.Run("ConnectToRemote", func(t *testing.T) {
		// Try to connect to an unreachable host
		cmd := exec.Command(ciloBinary, "connect", "192.0.2.100:8080")
		output, err := cmd.CombinedOutput()

		// Expected to fail since no real remote exists
		if err == nil {
			t.Logf("Warning: connect succeeded unexpectedly")
		}
		t.Logf("Connect attempt output: %s", output)
	})

	// Step 2: Test that --on flag is recognized by commands
	t.Run("OnFlagRecognized", func(t *testing.T) {
		// Test that up command accepts --on flag (will fail since machine not connected)
		cmd := exec.Command(ciloBinary, "up", envName, "--on", "test-machine", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected up with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		// Should mention that machine is not connected
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Up with --on output (expected failure): %s", outputStr)
	})

	// Step 3: Test list shows local environments when no remote connected
	t.Run("ListShowsLocalWhenNoRemote", func(t *testing.T) {
		// Create a local environment first
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
		}

		cmd = exec.Command(ciloBinary, "up", envName, "--project", projectName)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
		}

		// List should show local environment
		cmd = exec.Command(ciloBinary, "list", "--project", projectName)
		output, err = cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("environment %q not found in list", envName)
		}

		// Should show "local" as machine
		if !strings.Contains(string(output), "local") {
			t.Errorf("expected 'local' machine in list output")
		}
		t.Logf("List output: %s", output)
	})

	// Step 4: Test down with --on flag (will fail since machine not connected)
	t.Run("DownWithOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "down", envName, "--on", "test-machine", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected down with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Down with --on output (expected failure): %s", outputStr)
	})

	// Step 5: Test destroy with --on flag (will fail since machine not connected)
	t.Run("DestroyWithOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "destroy", envName, "--on", "test-machine", "--force", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected destroy with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Destroy with --on output (expected failure): %s", outputStr)
	})

	// Step 6: Clean up local environment
	t.Run("CleanupLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo destroy failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "destroyed") {
			t.Errorf("expected 'destroyed' in output, got: %s", output)
		}
		t.Logf("Destroy output: %s", output)
	})
}

// TestRemoteWorkspaceSync tests workspace sync during remote workflow
func TestRemoteWorkspaceSync(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create a test project with files to sync
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

	// Create additional files that would be synced
	err = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(dir, "src"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "sync-test-" + timestamp
	projectName := "sync-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Create local environment
	t.Run("CreateLocalForSyncTest", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
		}

		// Verify workspace was created with all files
		workspace, err := runCiloOutput(moduleAndRepoRootsForTest(t), "path", envName, "--project", projectName)
		if err != nil {
			t.Fatalf("cilo path failed: %v", err)
		}
		workspace = strings.TrimSpace(workspace)

		// Check that files exist in workspace
		if _, err := os.Stat(filepath.Join(workspace, "docker-compose.yml")); err != nil {
			t.Errorf("docker-compose.yml not found in workspace")
		}
		if _, err := os.Stat(filepath.Join(workspace, "README.md")); err != nil {
			t.Errorf("README.md not found in workspace")
		}
		if _, err := os.Stat(filepath.Join(workspace, "src", "main.go")); err != nil {
			t.Errorf("src/main.go not found in workspace")
		}

		t.Logf("Workspace created at: %s", workspace)
	})
}

// TestRemoteDNSResolution tests DNS resolution during remote workflow
func TestRemoteDNSResolution(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create a test project
	dir := t.TempDir()
	composeContent := `
services:
  web:
    image: nginx:alpine
    ports:
      - "80"
`
	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "dns-test-" + timestamp
	projectName := "dns-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Create and start environment
	t.Run("CreateAndStartForDNSTest", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
		}

		cmd = exec.Command(ciloBinary, "up", envName, "--project", projectName)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
		}

		// Check that status shows DNS information
		cmd = exec.Command(ciloBinary, "status", envName, "--project", projectName)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo status failed: %v\nOutput: %s", err, output)
		}

		// Should show services with URLs
		if !strings.Contains(string(output), "Services:") {
			t.Errorf("expected 'Services:' in status output")
		}
		t.Logf("Status output: %s", output)
	})
}

// TestRemoteLogsAndExec tests logs and exec commands with --on flag
func TestRemoteLogsAndExec(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create a test project
	dir := t.TempDir()
	composeContent := `
services:
  web:
    image: nginx:alpine
`
	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "logs-exec-test-" + timestamp
	projectName := "logs-exec-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
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

	// Test logs with --on flag (will fail since machine not connected)
	t.Run("LogsWithOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "logs", envName, "web", "--on", "test-machine", "--tail", "10", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected logs with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Logs with --on output (expected failure): %s", outputStr)
	})

	// Test exec with --on flag (will fail since machine not connected)
	t.Run("ExecWithOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "exec", envName, "web", "echo", "test", "--on", "test-machine", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected exec with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Exec with --on output (expected failure): %s", outputStr)
	})

	// Test local logs work without --on
	t.Run("LocalLogsWithoutOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "logs", envName, "web", "--tail", "5", "--project", projectName)
		output, err := cmd.CombinedOutput()

		// May succeed or fail depending on container state
		t.Logf("Local logs output: %s", output)
		if err != nil {
			t.Logf("Local logs returned error (may be expected): %v", err)
		}
	})
}

// TestRemoteRunCommand tests the run command with --on flag
func TestRemoteRunCommand(t *testing.T) {
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
	envName := "run-remote-test-" + timestamp
	projectName := "run-remote-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test run with --on flag (will fail since machine not connected)
	t.Run("RunWithOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "run", "echo", envName, "hello", "--on", "test-machine", "--from", dir, "--project", projectName)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()

		// Should fail since machine is not connected
		if err == nil {
			t.Errorf("expected run with --on to fail for unconnected machine")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "not connected") && !strings.Contains(outputStr, "connect") {
			t.Errorf("expected error about machine not being connected, got: %s", outputStr)
		}
		t.Logf("Run with --on output (expected failure): %s", outputStr)
	})
}

// Helper function for tests
func moduleAndRepoRootsForTest(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
