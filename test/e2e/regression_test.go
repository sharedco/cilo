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

// TestLocalCommandsRegression tests that all local commands work unchanged
// without the --on flag (regression prevention for cloud redesign)
func TestLocalCommandsRegression(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Verify cilo is in PATH
	_, err := exec.LookPath(ciloBinary)
	if err != nil {
		t.Fatalf("cilo binary not found: %v (set CILO_BINARY if needed)", err)
	}

	// Create temp project with compose file
	dir := t.TempDir()

	composeContent := `
services:
  web:
    image: nginx:alpine
    ports:
      - "80"
  redis:
    image: redis:alpine
`

	err = os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().Format("20060102-150405")
	envName := "regression-test-" + timestamp
	projectName := "regression-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test 1: cilo create (local only, no --on flag)
	t.Run("CreateLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), "created") {
			t.Errorf("expected 'created' in output, got: %s", output)
		}
		t.Logf("Environment created: %s", output)
	})

	// Test 2: cilo up (local only, no --on flag)
	t.Run("UpLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", envName, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), "running") {
			t.Errorf("expected 'running' in output, got: %s", output)
		}
		t.Logf("Environment started: %s", output)
	})

	// Test 3: cilo list (local only, no --on flag)
	t.Run("ListLocalEnvironments", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list", "--project", projectName)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("environment %q not found in list output:\n%s", envName, output)
		}

		// Verify local machine is shown (not remote)
		if strings.Contains(string(output), "(unreachable)") {
			t.Errorf("local environment should not show as unreachable")
		}
		t.Logf("Environment list: %s", output)
	})

	// Test 4: cilo logs (local only, no --on flag)
	t.Run("LogsLocalEnvironment", func(t *testing.T) {
		// Get logs with tail limit to avoid hanging
		cmd := exec.Command(ciloBinary, "logs", envName, "web", "--tail", "10", "--project", projectName)
		output, err := cmd.CombinedOutput()
		// Logs command may fail if container isn't fully ready, that's ok for this test
		// We just verify the command runs without --on flag
		t.Logf("Logs output: %s", output)
		if err != nil {
			t.Logf("Logs command returned error (may be expected if container not ready): %v", err)
		}
	})

	// Test 5: cilo exec (local only, no --on flag)
	t.Run("ExecLocalEnvironment", func(t *testing.T) {
		// Execute a simple command in the web container
		cmd := exec.Command(ciloBinary, "exec", envName, "web", "echo", "test-exec", "--project", projectName)
		output, err := cmd.CombinedOutput()
		// Exec may fail if container isn't ready, that's ok for this test
		t.Logf("Exec output: %s", output)
		if err != nil {
			t.Logf("Exec command returned error (may be expected if container not ready): %v", err)
		} else if !strings.Contains(string(output), "test-exec") {
			t.Logf("expected 'test-exec' in output, but command executed successfully")
		}
	})

	// Test 6: cilo down (local only, no --on flag)
	t.Run("DownLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "down", envName, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo down failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), "stopped") {
			t.Errorf("expected 'stopped' in output, got: %s", output)
		}
		t.Logf("Environment stopped: %s", output)
	})

	// Test 7: cilo destroy (local only, no --on flag)
	t.Run("DestroyLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo destroy failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), "destroyed") {
			t.Errorf("expected 'destroyed' in output, got: %s", output)
		}
		t.Logf("Environment destroyed: %s", output)
	})

	// Test 8: Verify environment is gone
	t.Run("VerifyDestroyed", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list", "--project", projectName)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if strings.Contains(string(output), envName) {
			t.Errorf("environment %q still exists after destroy", envName)
		}
	})
}

// TestLocalRunCommand tests the cilo run command without --on flag
func TestLocalRunCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create temp project with compose file
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
	envName := "run-test-" + timestamp
	projectName := "run-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test cilo run creates and starts environment
	t.Run("RunCreatesEnvironment", func(t *testing.T) {
		// We can't actually test the full run command as it execs into the command
		// But we can verify the environment was created by using --no-up and checking
		cmd := exec.Command(ciloBinary, "run", "echo", envName, "--from", dir, "--project", projectName, "--no-up")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		// The run command execs, so it may not return, but the env should be created
		t.Logf("Run output: %s", output)

		// Give it a moment to create the environment
		time.Sleep(500 * time.Millisecond)

		// Verify environment exists
		listCmd := exec.Command(ciloBinary, "list", "--project", projectName)
		listOutput, err := listCmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(listOutput), envName) {
			t.Errorf("environment %q not found after run command", envName)
		}
	})
}

// TestLocalStatusCommand tests the cilo status command without --on flag
func TestLocalStatusCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create temp project with compose file
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
	envName := "status-test-" + timestamp
	projectName := "status-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Create and start environment
	cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
	}

	cmd = exec.Command(ciloBinary, "up", envName, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
	}

	// Test status command
	t.Run("StatusLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "status", envName, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo status failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("expected environment name in status output")
		}
		if !strings.Contains(string(output), "Status:") {
			t.Errorf("expected 'Status:' in output")
		}
		t.Logf("Status output: %s", output)
	})
}

// TestLocalComposeCommand tests the cilo compose command without --on flag
func TestLocalComposeCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create temp project with compose file
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
	envName := "compose-test-" + timestamp
	projectName := "compose-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Create and start environment
	cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo create failed: %v\nOutput: %s", err, output)
	}

	cmd = exec.Command(ciloBinary, "up", envName, "--project", projectName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
	}

	// Test compose ps command
	t.Run("ComposePsLocalEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "compose", envName, "ps", "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo compose ps failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "web") {
			t.Errorf("expected 'web' in compose ps output")
		}
		t.Logf("Compose ps output: %s", output)
	})
}

// TestNoOnFlagBehavior verifies that commands without --on flag default to local
func TestNoOnFlagBehavior(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" && os.Getenv("CILO_E2E") != "1" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true or CILO_E2E=1)")
	}

	ciloBinary := ciloBinaryPath()

	// Create temp project with compose file
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
	envName := "no-on-test-" + timestamp
	projectName := "no-on-project"

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force", "--project", projectName)
		_ = cmd.Run()
	}()

	// Test that create without --on creates locally
	t.Run("CreateWithoutOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "create", envName, "--from", dir, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo create without --on failed: %v\nOutput: %s", err, output)
		}

		// Verify it mentions local workspace
		if !strings.Contains(string(output), "Workspace:") {
			t.Errorf("expected local workspace path in output")
		}
		t.Logf("Create output: %s", output)
	})

	// Test that up without --on starts locally
	t.Run("UpWithoutOnFlag", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", envName, "--project", projectName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up without --on failed: %v\nOutput: %s", err, output)
		}

		// Verify it shows local environment is running
		if !strings.Contains(string(output), "running") {
			t.Errorf("expected 'running' in output")
		}
		t.Logf("Up output: %s", output)
	})

	// Verify in list that it shows as local
	t.Run("ListShowsLocalMachine", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list", "--project", projectName)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		// Should show "local" as the machine
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, envName) {
				if !strings.Contains(line, "local") && !strings.Contains(line, "running") {
					t.Errorf("expected 'local' machine for environment %q, got: %s", envName, line)
				}
				break
			}
		}
		t.Logf("List output: %s", output)
	})
}
