//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvRenderExample(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker not available: %v", err)
	}

	moduleDir, repoRoot := moduleAndRepoRoots(t)
	fromPath := filepath.Join(repoRoot, "examples", "env-render")
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("missing example: %v", err)
	}

	envName := "e2e-env-render"

	if err := runCilo(moduleDir, "destroy", envName, "--force"); err != nil {
		// ignore destroy errors; env may not exist
		_ = err
	}

	if err := runCilo(moduleDir, "create", envName, "--from", fromPath); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := runCilo(moduleDir, "up", envName); err != nil {
		t.Fatalf("up failed: %v", err)
	}
	defer func() {
		_ = runCilo(moduleDir, "destroy", envName, "--force")
	}()

	workspace, err := runCiloOutput(moduleDir, "path", envName)
	if err != nil {
		t.Fatalf("path failed: %v", err)
	}
	workspace = strings.TrimSpace(workspace)

	content, err := os.ReadFile(filepath.Join(workspace, "envs", "node", ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(content), "node-"+envName) {
		t.Fatalf("expected env to be rendered, got %q", string(content))
	}
}

func TestCiloBasicExample(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker not available: %v", err)
	}

	moduleDir, repoRoot := moduleAndRepoRoots(t)
	fromPath := filepath.Join(repoRoot, "examples", "basic")
	if _, err := os.Stat(fromPath); err != nil {
		t.Fatalf("missing example: %v", err)
	}

	envName := "e2e-basic"
	_ = runCilo(moduleDir, "destroy", envName, "--force")

	if err := runCilo(moduleDir, "create", envName, "--from", fromPath); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := runCilo(moduleDir, "up", envName); err != nil {
		t.Fatalf("up failed: %v", err)
	}
	defer func() {
		_ = runCilo(moduleDir, "destroy", envName, "--force")
	}()

	psOutput, err := runCiloOutput(moduleDir, "compose", envName, "ps")
	if err != nil {
		t.Fatalf("compose ps failed: %v", err)
	}
	if !strings.Contains(psOutput, "nginx") {
		t.Fatalf("expected nginx in compose ps output, got: %s", psOutput)
	}
}

func moduleAndRepoRoots(t *testing.T) (string, string) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	moduleDir := filepath.Clean(filepath.Join(wd, "..", ".."))
	repoRoot := filepath.Clean(filepath.Join(moduleDir, ".."))
	return moduleDir, repoRoot
}

func runCilo(root string, args ...string) error {
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCiloOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}
