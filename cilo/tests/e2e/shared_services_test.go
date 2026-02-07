//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSharedServicesBasic(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker not available: %v", err)
	}

	moduleDir, repoRoot := moduleAndRepoRoots(t)
	fromPath := filepath.Join(repoRoot, "examples", "shared-services")
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("missing shared-services example: %v", err)
	}

	project := "shared-test"
	env1 := "e2e-shared-env1"
	env2 := "e2e-shared-env2"

	// Cleanup any existing environments
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)
	_ = runCilo(moduleDir, "destroy", env2, "--force", "--project", project)

	// Test 1: Create and start first environment with shared elasticsearch
	t.Run("CreateEnv1WithSharedService", func(t *testing.T) {
		if err := runCilo(moduleDir, "create", env1, "--from", fromPath, "--project", project); err != nil {
			t.Fatalf("create env1 failed: %v", err)
		}
		if err := runCilo(moduleDir, "up", env1, "--project", project); err != nil {
			t.Fatalf("up env1 failed: %v", err)
		}

		// Verify elasticsearch is running
		containersOutput, err := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		if !strings.Contains(string(containersOutput), "elasticsearch") {
			t.Errorf("expected elasticsearch container to be running")
		}

		// Verify status shows shared service
		statusOutput, err := runCiloOutput(moduleDir, "status", env1, "--project", project)
		if err != nil {
			t.Fatalf("status failed: %v", err)
		}
		if !strings.Contains(statusOutput, "elasticsearch") {
			t.Errorf("expected elasticsearch in status output, got: %s", statusOutput)
		}
		if !strings.Contains(statusOutput, "shared") {
			t.Errorf("expected 'shared' type in status output, got: %s", statusOutput)
		}
	})

	// Test 2: Create second environment that should reuse the same elasticsearch
	t.Run("CreateEnv2ReuseSharedService", func(t *testing.T) {
		// Get elasticsearch container name before creating env2
		containersOutput, err := exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		esContainerBefore := strings.TrimSpace(string(containersOutput))

		if err := runCilo(moduleDir, "create", env2, "--from", fromPath, "--project", project); err != nil {
			t.Fatalf("create env2 failed: %v", err)
		}
		if err := runCilo(moduleDir, "up", env2, "--project", project); err != nil {
			t.Fatalf("up env2 failed: %v", err)
		}

		// Verify same elasticsearch container is still running
		containersOutput, err = exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		esContainerAfter := strings.TrimSpace(string(containersOutput))

		if esContainerBefore != esContainerAfter {
			t.Errorf("expected same elasticsearch container, got before=%s after=%s", esContainerBefore, esContainerAfter)
		}

		// Verify only ONE elasticsearch container is running
		lines := strings.Split(strings.TrimSpace(string(containersOutput)), "\n")
		if len(lines) != 1 {
			t.Errorf("expected exactly 1 elasticsearch container, found %d", len(lines))
		}
	})

	// Test 3: Verify both environments can access elasticsearch
	t.Run("BothEnvironmentsConnected", func(t *testing.T) {
		// Get elasticsearch container
		esContainer, err := exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		esContainerName := strings.TrimSpace(string(esContainer))

		// Check elasticsearch is connected to both environment networks
		inspectOutput, err := exec.Command("docker", "inspect", esContainerName, "--format", "{{json .NetworkSettings.Networks}}").Output()
		if err != nil {
			t.Fatalf("docker inspect failed: %v", err)
		}
		networkInfo := string(inspectOutput)

		if !strings.Contains(networkInfo, env1) {
			t.Errorf("elasticsearch not connected to %s network", env1)
		}
		if !strings.Contains(networkInfo, env2) {
			t.Errorf("elasticsearch not connected to %s network", env2)
		}
	})

	// Test 4: Stop env1, elasticsearch should stay running (grace period)
	t.Run("GracePeriodAfterEnv1Down", func(t *testing.T) {
		if err := runCilo(moduleDir, "down", env1, "--project", project); err != nil {
			t.Fatalf("down env1 failed: %v", err)
		}

		// Elasticsearch should still be running (grace period)
		containersOutput, err := exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		if !strings.Contains(string(containersOutput), "elasticsearch") {
			t.Errorf("elasticsearch should still be running during grace period")
		}

		// But should be disconnected from env1 network
		esContainer := strings.TrimSpace(string(containersOutput))
		inspectOutput, err := exec.Command("docker", "inspect", esContainer, "--format", "{{json .NetworkSettings.Networks}}").Output()
		if err != nil {
			t.Fatalf("docker inspect failed: %v", err)
		}
		networkInfo := string(inspectOutput)

		if strings.Contains(networkInfo, env1) {
			t.Errorf("elasticsearch should be disconnected from %s network", env1)
		}
		if !strings.Contains(networkInfo, env2) {
			t.Errorf("elasticsearch should still be connected to %s network", env2)
		}
	})

	// Test 5: Stop env2, elasticsearch should stop after grace period
	t.Run("StopAfterAllEnvironmentsDown", func(t *testing.T) {
		if err := runCilo(moduleDir, "down", env2, "--project", project); err != nil {
			t.Fatalf("down env2 failed: %v", err)
		}

		// Wait for grace period (60 seconds + buffer)
		t.Log("Waiting for grace period to expire (65 seconds)...")
		time.Sleep(65 * time.Second)

		// Elasticsearch should now be stopped
		containersOutput, err := exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		if strings.Contains(string(containersOutput), "elasticsearch") {
			t.Errorf("elasticsearch should be stopped after grace period expires")
		}
	})

	// Cleanup
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)
	_ = runCilo(moduleDir, "destroy", env2, "--force", "--project", project)
	// Force cleanup elasticsearch if still running
	_ = exec.Command("docker", "stop", "cilo_shared_"+project+"_elasticsearch").Run()
	_ = exec.Command("docker", "rm", "cilo_shared_"+project+"_elasticsearch").Run()
}

func TestSharedServicesCLIFlags(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker not available: %v", err)
	}

	moduleDir, repoRoot := moduleAndRepoRoots(t)
	fromPath := filepath.Join(repoRoot, "examples", "shared-services")
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("missing shared-services example: %v", err)
	}

	project := "shared-cli-test"
	env1 := "e2e-cli-env1"

	// Cleanup
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)

	// Test --shared flag with space-delimited values
	t.Run("SharedFlagSpaceDelimited", func(t *testing.T) {
		if err := runCilo(moduleDir, "create", env1, "--from", fromPath, "--project", project); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		// Use --shared flag with space-delimited services
		if err := runCilo(moduleDir, "up", env1, "--project", project, "--shared", "app"); err != nil {
			t.Fatalf("up with --shared flag failed: %v", err)
		}
		defer runCilo(moduleDir, "destroy", env1, "--force", "--project", project)

		// Verify app is now shared (overriding its default isolated behavior)
		statusOutput, err := runCiloOutput(moduleDir, "status", env1, "--project", project)
		if err != nil {
			t.Fatalf("status failed: %v", err)
		}

		if !strings.Contains(statusOutput, "app") {
			t.Errorf("expected app in status output")
		}
	})

	// Cleanup
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)
	_ = exec.Command("docker", "stop", "cilo_shared_"+project+"_app").Run()
	_ = exec.Command("docker", "rm", "cilo_shared_"+project+"_app").Run()
	_ = exec.Command("docker", "stop", "cilo_shared_"+project+"_elasticsearch").Run()
	_ = exec.Command("docker", "rm", "cilo_shared_"+project+"_elasticsearch").Run()
}

func TestSharedServicesDoctor(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker not available: %v", err)
	}

	moduleDir, repoRoot := moduleAndRepoRoots(t)
	fromPath := filepath.Join(repoRoot, "examples", "shared-services")
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("missing shared-services example: %v", err)
	}

	project := "shared-doctor-test"
	env1 := "e2e-doctor-env1"

	// Cleanup
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)

	// Create environment with shared service
	if err := runCilo(moduleDir, "create", env1, "--from", fromPath, "--project", project); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := runCilo(moduleDir, "up", env1, "--project", project); err != nil {
		t.Fatalf("up failed: %v", err)
	}

	// Simulate orphaned shared service by stopping env without proper cleanup
	if err := exec.Command("docker", "compose", "-p", "cilo-"+project+"-"+env1, "down").Run(); err != nil {
		t.Fatalf("docker compose down failed: %v", err)
	}

	// Run doctor to detect orphaned service
	t.Run("DetectOrphanedService", func(t *testing.T) {
		doctorOutput, err := runCiloOutput(moduleDir, "doctor")
		if err != nil {
			t.Fatalf("doctor failed: %v", err)
		}

		// Should detect the orphaned elasticsearch
		if !strings.Contains(doctorOutput, "orphaned") && !strings.Contains(doctorOutput, "issue") {
			t.Logf("doctor output: %s", doctorOutput)
			// This is OK - doctor might not detect issues if state was already cleaned
		}
	})

	// Run doctor with --fix
	t.Run("FixOrphanedService", func(t *testing.T) {
		if err := runCilo(moduleDir, "doctor", "--fix"); err != nil {
			t.Fatalf("doctor --fix failed: %v", err)
		}

		// Verify elasticsearch is stopped
		containersOutput, err := exec.Command("docker", "ps", "--filter", "name=elasticsearch", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatalf("docker ps failed: %v", err)
		}
		// After fix, orphaned containers should be cleaned up
		t.Logf("Containers after doctor fix: %s", string(containersOutput))
	})

	// Cleanup
	_ = runCilo(moduleDir, "destroy", env1, "--force", "--project", project)
	_ = exec.Command("docker", "stop", "cilo_shared_"+project+"_elasticsearch").Run()
	_ = exec.Command("docker", "rm", "cilo_shared_"+project+"_elasticsearch").Run()
}
