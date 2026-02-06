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

func TestGitSync(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}

	moduleDir, _ := moduleAndRepoRoots(t)

	// 1. Create a dummy project with a git repo
	projectDir, err := os.MkdirTemp("", "cilo-sync-test-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Init git repo
	runCmd(t, projectDir, "git", "init")
	runCmd(t, projectDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, projectDir, "git", "config", "user.name", "Test User")

	// Create a docker-compose.yml
	composeContent := `
services:
  web:
    image: nginx:alpine
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit initial state
	runCmd(t, projectDir, "git", "add", ".")
	runCmd(t, projectDir, "git", "commit", "-m", "initial commit")

	envName := "e2e-sync-test"
	_ = runCilo(moduleDir, "destroy", envName, "--force", "--project", "sync-test")

	// 2. Create environment
	if err := runCilo(moduleDir, "create", envName, "--from", projectDir, "--project", "sync-test"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer func() {
		_ = runCilo(moduleDir, "destroy", envName, "--force", "--project", "sync-test")
	}()

	// 3. Modify code in the environment (simulating an agent)
	workspace, err := runCiloOutput(moduleDir, "path", envName, "--project", "sync-test")
	if err != nil {
		t.Fatalf("path failed: %v", err)
	}
	workspace = strings.TrimSpace(workspace)

	// Create a new file in env
	newFilePath := filepath.Join(workspace, "agent-code.txt")
	if err := os.WriteFile(newFilePath, []byte("agent was here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Agent commits changes
	runCmd(t, workspace, "git", "add", "agent-code.txt")
	runCmd(t, workspace, "git", "commit", "-m", "agent work")

	// 4. Run cilo diff
	diffOutput, err := runCiloOutput(moduleDir, "diff", envName, "--project", "sync-test")
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if !strings.Contains(diffOutput, "agent was here") {
		t.Fatalf("expected diff to contain agent code, got: %s", diffOutput)
	}

	// 5. Run cilo merge
	if err := runCilo(moduleDir, "merge", envName, "--project", "sync-test"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// 6. Verify changes are in the host project
	hostCodePath := filepath.Join(projectDir, "agent-code.txt")
	if _, err := os.Stat(hostCodePath); err != nil {
		t.Fatalf("agent code not found in host project after merge: %v", err)
	}

	content, _ := os.ReadFile(hostCodePath)
	if string(content) != "agent was here" {
		t.Fatalf("expected 'agent was here', got %q", string(content))
	}
}

func TestNestedGitSync(t *testing.T) {
	if os.Getenv("CILO_E2E") != "1" {
		t.Skip("set CILO_E2E=1 to run")
	}

	moduleDir, _ := moduleAndRepoRoots(t)

	// 1. Create a monorepo structure
	projectDir, _ := os.MkdirTemp("", "cilo-monorepo-*")
	defer os.RemoveAll(projectDir)

	// Root repo
	runCmd(t, projectDir, "git", "init")
	runCmd(t, projectDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, projectDir, "git", "config", "user.name", "Test User")
	os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {web: {image: nginx}}"), 0644)
	runCmd(t, projectDir, "git", "add", ".")
	runCmd(t, projectDir, "git", "commit", "-m", "root commit")

	// Sub repo (nested)
	subDir := filepath.Join(projectDir, "services", "api")
	os.MkdirAll(subDir, 0755)
	runCmd(t, subDir, "git", "init")
	runCmd(t, subDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, subDir, "git", "config", "user.name", "Test User")
	os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0644)
	runCmd(t, subDir, "git", "add", ".")
	runCmd(t, subDir, "git", "commit", "-m", "sub commit")

	envName := "e2e-monorepo-test"
	_ = runCilo(moduleDir, "destroy", envName, "--force", "--project", "monorepo")

	// 2. Create environment
	if err := runCilo(moduleDir, "create", envName, "--from", projectDir, "--project", "monorepo"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer func() {
		_ = runCilo(moduleDir, "destroy", envName, "--force", "--project", "monorepo")
	}()

	workspace, _ := runCiloOutput(moduleDir, "path", envName, "--project", "monorepo")
	workspace = strings.TrimSpace(workspace)

	// 3. Modify both repos in env
	// Root change
	os.WriteFile(filepath.Join(workspace, "root-change.txt"), []byte("root"), 0644)
	runCmd(t, workspace, "git", "add", "root-change.txt")
	runCmd(t, workspace, "git", "commit", "-m", "root agent work")

	// Sub change
	subWorkspace := filepath.Join(workspace, "services", "api")
	os.WriteFile(filepath.Join(subWorkspace, "sub-change.txt"), []byte("sub"), 0644)
	runCmd(t, subWorkspace, "git", "add", "sub-change.txt")
	runCmd(t, subWorkspace, "git", "commit", "-m", "sub agent work")

	// 4. Merge
	if err := runCilo(moduleDir, "merge", envName, "--project", "monorepo"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// 5. Verify both
	if _, err := os.Stat(filepath.Join(projectDir, "root-change.txt")); err != nil {
		t.Errorf("root change missing")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "services", "api", "sub-change.txt")); err != nil {
		t.Errorf("sub change missing")
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("command %s %v failed in %s: %v\nSTDOUT: %s\nSTDERR: %s", name, args, dir, err, out.String(), errOut.String())
	}
}
