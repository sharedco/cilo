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

// TestFullWorkflow tests the complete environment lifecycle
// Requires: Docker running, cilo binary built
func TestFullWorkflow(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true)")
	}

	// Verify cilo binary exists
	ciloBinary := os.Getenv("CILO_BINARY")
	if ciloBinary == "" {
		ciloBinary = "cilo"
	}

	// Check if cilo is in PATH
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

	envName := "e2e-test-" + strings.ReplaceAll(time.Now().Format("20060102-150405"), ":", "")

	// Ensure cleanup happens
	defer func() {
		t.Log("Cleaning up environment...")
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force")
		_ = cmd.Run()
	}()

	// Test cilo up
	t.Run("CreateEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", envName, "--from", dir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
		}
		t.Logf("Environment created: %s", output)
	})

	// Test cilo list
	t.Run("ListEnvironments", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if !strings.Contains(string(output), envName) {
			t.Errorf("environment %q not found in list output:\n%s", envName, output)
		}
		t.Logf("Environment list: %s", output)
	})

	// Test cilo down
	t.Run("StopEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "down", envName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo down failed: %v\nOutput: %s", err, output)
		}
		t.Logf("Environment stopped: %s", output)
	})

	// Test cilo destroy
	t.Run("DestroyEnvironment", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "destroy", envName, "--force")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo destroy failed: %v\nOutput: %s", err, output)
		}
		t.Logf("Environment destroyed: %s", output)
	})

	// Verify environment is gone
	t.Run("VerifyDestroyed", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		if strings.Contains(string(output), envName) {
			t.Errorf("environment %q still exists after destroy", envName)
		}
	})
}

// TestParallelEnvironments tests running multiple environments simultaneously
func TestParallelEnvironments(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true)")
	}

	ciloBinary := os.Getenv("CILO_BINARY")
	if ciloBinary == "" {
		ciloBinary = "cilo"
	}

	// Create temp project
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
	env1 := "e2e-parallel-1-" + timestamp
	env2 := "e2e-parallel-2-" + timestamp

	// Ensure cleanup
	defer func() {
		exec.Command(ciloBinary, "destroy", env1, "--force").Run()
		exec.Command(ciloBinary, "destroy", env2, "--force").Run()
	}()

	// Create first environment
	t.Run("CreateFirstEnv", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", env1, "--from", dir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed for %s: %v\nOutput: %s", env1, err, output)
		}
	})

	// Create second environment
	t.Run("CreateSecondEnv", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "up", env2, "--from", dir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed for %s: %v\nOutput: %s", env2, err, output)
		}
	})

	// Verify both exist
	t.Run("VerifyBothExist", func(t *testing.T) {
		cmd := exec.Command(ciloBinary, "list")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("cilo list failed: %v", err)
		}

		listOutput := string(output)
		if !strings.Contains(listOutput, env1) {
			t.Errorf("environment %q not found in list", env1)
		}
		if !strings.Contains(listOutput, env2) {
			t.Errorf("environment %q not found in list", env2)
		}
	})

	// Cleanup
	t.Run("DestroyBoth", func(t *testing.T) {
		// Destroy first
		cmd := exec.Command(ciloBinary, "destroy", env1, "--force")
		if err := cmd.Run(); err != nil {
			t.Errorf("failed to destroy %s: %v", env1, err)
		}

		// Destroy second
		cmd = exec.Command(ciloBinary, "destroy", env2, "--force")
		if err := cmd.Run(); err != nil {
			t.Errorf("failed to destroy %s: %v", env2, err)
		}
	})
}

// TestDetectProject tests project detection
func TestDetectProject(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true)")
	}

	ciloBinary := os.Getenv("CILO_BINARY")
	if ciloBinary == "" {
		ciloBinary = "cilo"
	}

	t.Run("ComposeFile", func(t *testing.T) {
		dir := t.TempDir()

		composeContent := `
services:
  test:
    image: alpine
`
		err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Test detection (this would be a cilo detect command if it exists)
		// For now, just verify cilo up can detect it
		envName := "e2e-detect-" + time.Now().Format("20060102-150405")
		defer exec.Command(ciloBinary, "destroy", envName, "--force").Run()

		cmd := exec.Command(ciloBinary, "up", envName, "--from", dir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cilo up failed: %v\nOutput: %s", err, output)
		}

		// Clean up
		cmd = exec.Command(ciloBinary, "destroy", envName, "--force")
		_ = cmd.Run()
	})

	t.Run("NoProject", func(t *testing.T) {
		dir := t.TempDir()

		envName := "e2e-no-project-" + time.Now().Format("20060102-150405")

		cmd := exec.Command(ciloBinary, "up", envName, "--from", dir)
		output, err := cmd.CombinedOutput()

		// Should fail because no project found
		if err == nil {
			t.Errorf("expected cilo up to fail for empty directory, but it succeeded\nOutput: %s", output)
			// Clean up if it somehow succeeded
			exec.Command(ciloBinary, "destroy", envName, "--force").Run()
		}
	})
}
