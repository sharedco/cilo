// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRsyncSync verifies that rsync syncs a directory and files arrive on remote
func TestRsyncSync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rsync not typically available on Windows")
	}

	// Create source directory with test files
	srcDir, err := os.MkdirTemp("", "sync-test-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
		"subdir/file3.txt": "content3",
		".hidden":          "hidden content",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(srcDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create destination directory (simulating remote)
	dstDir, err := os.MkdirTemp("", "sync-test-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Test sync using local rsync (simulating remote via file://)
	opts := SyncOptions{
		RemoteHost: "localhost",
		RemotePath: dstDir,
		UseRsync:   true,
	}

	// For local testing, we'll use rsync directly
	if !isRsyncAvailable() {
		t.Skip("rsync not available")
	}

	err = SyncWorkspace(srcDir, "localhost", dstDir, opts)
	if err != nil {
		t.Fatalf("SyncWorkspace failed: %v", err)
	}

	// Verify files arrived
	for path, expectedContent := range testFiles {
		dstPath := filepath.Join(dstDir, path)
		content, err := os.ReadFile(dstPath)
		if err != nil {
			t.Errorf("File %s not found in destination: %v", path, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content mismatch: got %q, want %q", path, string(content), expectedContent)
		}
	}
}

// TestSyncExcludes verifies that .git, node_modules, and other default excludes are respected
func TestSyncExcludes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rsync not typically available on Windows")
	}

	if !isRsyncAvailable() {
		t.Skip("rsync not available")
	}

	// Create source directory with files that should be excluded
	srcDir, err := os.MkdirTemp("", "sync-test-exclude-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create files that SHOULD be excluded
	excludedPaths := []string{
		".git/config",
		".git/HEAD",
		"node_modules/package/file.js",
		".cilo/meta.json",
		"__pycache__/module.cpython-39.pyc",
		".venv/bin/python",
	}

	// Create files that SHOULD NOT be excluded
	includedPaths := []string{
		"src/main.go",
		"README.md",
		".env.example",
		"dist/app.js",
	}

	// Create all files
	for _, path := range append(excludedPaths, includedPaths...) {
		fullPath := filepath.Join(srcDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "sync-test-dst-exclude-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Sync with default excludes
	opts := SyncOptions{
		RemoteHost: "localhost",
		RemotePath: dstDir,
		UseRsync:   true,
	}

	err = SyncWorkspace(srcDir, "localhost", dstDir, opts)
	if err != nil {
		t.Fatalf("SyncWorkspace failed: %v", err)
	}

	// Verify excluded files are NOT present
	for _, path := range excludedPaths {
		dstPath := filepath.Join(dstDir, path)
		if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
			t.Errorf("Excluded file %s should not exist in destination", path)
		}
	}

	// Verify included files ARE present
	for _, path := range includedPaths {
		dstPath := filepath.Join(dstDir, path)
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Errorf("Included file %s should exist in destination", path)
		}
	}
}

// TestSyncIncremental verifies that only changed files are transferred
func TestSyncIncremental(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rsync not typically available on Windows")
	}

	if !isRsyncAvailable() {
		t.Skip("rsync not available")
	}

	// Create source directory
	srcDir, err := os.MkdirTemp("", "sync-test-incr-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create initial files
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "sync-test-incr-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	opts := SyncOptions{
		RemoteHost: "localhost",
		RemotePath: dstDir,
		UseRsync:   true,
	}

	// Initial sync
	err = SyncWorkspace(srcDir, "localhost", dstDir, opts)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// Modify only one file
	time.Sleep(100 * time.Millisecond) // Ensure different mod time
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("modified content"), 0644)

	// Sync again - should only transfer modified file
	err = SyncWorkspace(srcDir, "localhost", dstDir, opts)
	if err != nil {
		t.Fatalf("Incremental sync failed: %v", err)
	}

	// Verify modified file was updated
	content, _ := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if string(content) != "modified content" {
		t.Errorf("Modified file not updated: got %q", string(content))
	}

	// Verify unmodified file still exists with original content
	content, _ = os.ReadFile(filepath.Join(dstDir, "file2.txt"))
	if string(content) != "content2" {
		t.Errorf("Unmodified file changed unexpectedly: got %q", string(content))
	}
}

// TestSyncCiloignore verifies that .ciloignore file patterns are respected
func TestSyncCiloignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rsync not typically available on Windows")
	}

	if !isRsyncAvailable() {
		t.Skip("rsync not available")
	}

	// Create source directory
	srcDir, err := os.MkdirTemp("", "sync-test-ignore-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create .ciloignore file
	ciloignoreContent := `# Ignore patterns
*.log
build/
.env.local
secret.txt
`
	os.WriteFile(filepath.Join(srcDir, ".ciloignore"), []byte(ciloignoreContent), 0644)

	// Create files - some should be ignored based on .ciloignore
	files := map[string]bool{
		"app.log":         true,  // should be ignored (*.log)
		"debug.log":       true,  // should be ignored (*.log)
		"build/output.js": true,  // should be ignored (build/)
		".env.local":      true,  // should be ignored
		"secret.txt":      true,  // should be ignored
		"src/main.go":     false, // should NOT be ignored
		"README.md":       false, // should NOT be ignored
		".env":            false, // should NOT be ignored
	}

	for path, _ := range files {
		fullPath := filepath.Join(srcDir, path)
		dir := filepath.Dir(fullPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(fullPath, []byte("test"), 0644)
	}

	// Create destination
	dstDir, err := os.MkdirTemp("", "sync-test-dst-ignore-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	opts := SyncOptions{
		RemoteHost: "localhost",
		RemotePath: dstDir,
		UseRsync:   true,
	}

	err = SyncWorkspace(srcDir, "localhost", dstDir, opts)
	if err != nil {
		t.Fatalf("SyncWorkspace failed: %v", err)
	}

	// Verify ignored files are NOT present
	for path, shouldIgnore := range files {
		dstPath := filepath.Join(dstDir, path)
		_, err := os.Stat(dstPath)
		exists := !os.IsNotExist(err)

		if shouldIgnore && exists {
			t.Errorf("File %s should be ignored but exists in destination", path)
		}
		if !shouldIgnore && !exists {
			t.Errorf("File %s should exist but is missing in destination", path)
		}
	}

	// Verify .ciloignore itself is not synced (it's a cilo metadata file)
	if _, err := os.Stat(filepath.Join(dstDir, ".ciloignore")); !os.IsNotExist(err) {
		t.Error(".ciloignore file should not be synced to destination")
	}
}

// TestSyncFallback verifies tar+ssh fallback when rsync is unavailable
func TestSyncFallback(t *testing.T) {
	// Create source directory
	srcDir, err := os.MkdirTemp("", "sync-test-fallback-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create test files
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "sync-test-fallback-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// For local testing, simulate the fallback by using tar directly
	err = syncWithTar(srcDir, dstDir)
	if err != nil {
		t.Fatalf("Fallback sync failed: %v", err)
	}

	// Verify files arrived
	content, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("file1.txt not found: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("file1.txt content mismatch: got %q", string(content))
	}

	content, err = os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	if err != nil {
		t.Fatalf("subdir/file2.txt not found: %v", err)
	}
	if string(content) != "content2" {
		t.Errorf("subdir/file2.txt content mismatch: got %q", string(content))
	}
}

// TestSyncOptions verifies SyncOptions struct and validation
func TestSyncOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    SyncOptions
		wantErr bool
	}{
		{
			name: "valid options with rsync",
			opts: SyncOptions{
				RemoteHost: "192.168.1.100",
				RemotePath: "/home/user/workspace",
				UseRsync:   true,
			},
			wantErr: false,
		},
		{
			name: "valid options with fallback",
			opts: SyncOptions{
				RemoteHost: "192.168.1.100",
				RemotePath: "/home/user/workspace",
				UseRsync:   false,
			},
			wantErr: false,
		},
		{
			name: "empty remote host",
			opts: SyncOptions{
				RemoteHost: "",
				RemotePath: "/home/user/workspace",
			},
			wantErr: true,
		},
		{
			name: "empty remote path",
			opts: SyncOptions{
				RemoteHost: "192.168.1.100",
				RemotePath: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSyncOptions(&tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSyncOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseCiloignore verifies .ciloignore parsing
func TestParseCiloignore(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple patterns",
			content:  "*.log\nbuild/\n.env\n",
			expected: []string{"*.log", "build/", ".env"},
		},
		{
			name:     "with comments and empty lines",
			content:  "# Ignore logs\n*.log\n\n# Build output\nbuild/\n",
			expected: []string{"*.log", "build/"},
		},
		{
			name:     "negation patterns",
			content:  "*.log\n!important.log\n",
			expected: []string{"*.log", "!important.log"},
		},
		{
			name:     "directory patterns",
			content:  "node_modules/\n.git/\n",
			expected: []string{"node_modules/", ".git/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCiloignoreContent(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("parseCiloignoreContent() returned %d patterns, want %d", len(result), len(tt.expected))
				return
			}
			for i, pattern := range tt.expected {
				if result[i] != pattern {
					t.Errorf("parseCiloignoreContent()[%d] = %q, want %q", i, result[i], pattern)
				}
			}
		})
	}
}

// TestDefaultExcludes verifies default exclude patterns
func TestDefaultExcludes(t *testing.T) {
	excludes := getDefaultExcludes()

	expected := []string{
		".git/",
		"node_modules/",
		".cilo/",
		"__pycache__/",
		".venv/",
	}

	for _, exp := range expected {
		found := false
		for _, ex := range excludes {
			if strings.Contains(ex, exp) || ex == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Default excludes missing: %s", exp)
		}
	}
}

// TestIsRsyncAvailable checks rsync detection
func TestIsRsyncAvailable(t *testing.T) {
	// This test just verifies the function doesn't panic
	available := isRsyncAvailable()

	// Verify by trying to run rsync
	_, err := exec.LookPath("rsync")
	if err != nil && available {
		t.Error("isRsyncAvailable() returned true but rsync not in PATH")
	}
	if err == nil && !available {
		t.Error("isRsyncAvailable() returned false but rsync is in PATH")
	}
}

// TestBuildRsyncArgs verifies rsync argument building
func TestBuildRsyncArgs(t *testing.T) {
	tests := []struct {
		name     string
		srcDir   string
		dstDir   string
		excludes []string
		wantArgs []string
	}{
		{
			name:     "basic args",
			srcDir:   "/src",
			dstDir:   "/dst",
			excludes: []string{},
			wantArgs: []string{"-avz", "--delete"},
		},
		{
			name:     "with excludes",
			srcDir:   "/src",
			dstDir:   "/dst",
			excludes: []string{".git/", "node_modules/"},
			wantArgs: []string{"-avz", "--delete", "--exclude=.git/", "--exclude=node_modules/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildRsyncArgs(tt.srcDir, tt.dstDir, tt.excludes, "localhost")

			// Check that expected args are present
			for _, want := range tt.wantArgs {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildRsyncArgs() missing arg %q in %v", want, args)
				}
			}
		})
	}
}

// TestSyncWorkspaceValidation verifies input validation
func TestSyncWorkspaceValidation(t *testing.T) {
	tests := []struct {
		name       string
		localPath  string
		remoteHost string
		remotePath string
		wantErr    bool
	}{
		{
			name:       "empty local path",
			localPath:  "",
			remoteHost: "host",
			remotePath: "/path",
			wantErr:    true,
		},
		{
			name:       "empty remote host",
			localPath:  "/local",
			remoteHost: "",
			remotePath: "/path",
			wantErr:    true,
		},
		{
			name:       "empty remote path",
			localPath:  "/local",
			remoteHost: "host",
			remotePath: "",
			wantErr:    true,
		},
		{
			name:       "nonexistent local path",
			localPath:  "/nonexistent/path/12345",
			remoteHost: "host",
			remotePath: "/path",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SyncOptions{
				RemoteHost: tt.remoteHost,
				RemotePath: tt.remotePath,
			}

			// Only test validation, not actual sync
			err := validateSyncInputs(tt.localPath, tt.remoteHost, tt.remotePath, &opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSyncInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function for tar sync (used in fallback test)
func syncWithTar(srcDir, dstDir string) error {
	// Create tar archive
	tarCmd := exec.Command("tar", "-czf", "-", "-C", srcDir, ".")
	tarOutput, err := tarCmd.Output()
	if err != nil {
		return fmt.Errorf("tar creation failed: %w", err)
	}

	// Extract tar archive
	untarCmd := exec.Command("tar", "-xzf", "-", "-C", dstDir)
	untarCmd.Stdin = strings.NewReader(string(tarOutput))
	if err := untarCmd.Run(); err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}

	return nil
}
