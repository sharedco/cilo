package cloud

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultExcludePatterns(t *testing.T) {
	patterns := DefaultExcludePatterns()

	expectedPatterns := []string{
		".git",
		".cilo",
		"node_modules",
		"__pycache__",
		"*.pyc",
		".env",
		".env.*",
		"*.log",
		".DS_Store",
		"Thumbs.db",
		"vendor",
		"target",
		"dist",
		"build",
		".next",
		".nuxt",
	}

	if len(patterns) != len(expectedPatterns) {
		t.Errorf("Expected %d patterns, got %d", len(expectedPatterns), len(patterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(patterns) {
			t.Errorf("Missing pattern at index %d: %s", i, expected)
			continue
		}
		if patterns[i] != expected {
			t.Errorf("Pattern at index %d: expected %q, got %q", i, expected, patterns[i])
		}
	}

	originalLen := len(patterns)
	patterns = append(patterns, "extra")
	newPatterns := DefaultExcludePatterns()
	if len(newPatterns) != originalLen {
		t.Error("DefaultExcludePatterns should return a new slice each time")
	}
}

func TestLoadProjectIgnore_NoFile(t *testing.T) {
	tempDir := t.TempDir()

	patterns, err := LoadProjectIgnore(tempDir)
	if err != nil {
		t.Fatalf("Expected no error when .ciloignore doesn't exist, got: %v", err)
	}

	defaults := DefaultExcludePatterns()
	if len(patterns) != len(defaults) {
		t.Errorf("Expected %d default patterns, got %d", len(defaults), len(patterns))
	}

	for i, expected := range defaults {
		if patterns[i] != expected {
			t.Errorf("Pattern at index %d: expected %q, got %q", i, expected, patterns[i])
		}
	}
}

func TestLoadProjectIgnore_WithFile(t *testing.T) {
	tempDir := t.TempDir()
	ignoreFile := filepath.Join(tempDir, ".ciloignore")

	content := `# This is a comment
*.tmp
.cache/
*.swp
# Another comment
custom_dir

`
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create .ciloignore: %v", err)
	}

	patterns, err := LoadProjectIgnore(tempDir)
	if err != nil {
		t.Fatalf("Failed to load .ciloignore: %v", err)
	}

	defaults := DefaultExcludePatterns()
	expectedCustom := []string{"*.tmp", ".cache/", "*.swp", "custom_dir"}
	expectedLen := len(defaults) + len(expectedCustom)

	if len(patterns) != expectedLen {
		t.Errorf("Expected %d patterns (defaults + custom), got %d", expectedLen, len(patterns))
	}

	for i, expected := range defaults {
		if patterns[i] != expected {
			t.Errorf("Default pattern at index %d: expected %q, got %q", i, expected, patterns[i])
		}
	}

	for i, expected := range expectedCustom {
		idx := len(defaults) + i
		if idx >= len(patterns) {
			t.Errorf("Missing custom pattern at index %d: %s", idx, expected)
			continue
		}
		if patterns[idx] != expected {
			t.Errorf("Custom pattern at index %d: expected %q, got %q", idx, expected, patterns[idx])
		}
	}
}

func TestLoadProjectIgnore_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	ignoreFile := filepath.Join(tempDir, ".ciloignore")

	if err := os.WriteFile(ignoreFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty .ciloignore: %v", err)
	}

	patterns, err := LoadProjectIgnore(tempDir)
	if err != nil {
		t.Fatalf("Failed to load empty .ciloignore: %v", err)
	}

	defaults := DefaultExcludePatterns()
	if len(patterns) != len(defaults) {
		t.Errorf("Expected %d default patterns from empty file, got %d", len(defaults), len(patterns))
	}
}

func TestLoadProjectIgnore_OnlyComments(t *testing.T) {
	tempDir := t.TempDir()
	ignoreFile := filepath.Join(tempDir, ".ciloignore")

	content := `# Comment 1
# Comment 2

   
`
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create .ciloignore: %v", err)
	}

	patterns, err := LoadProjectIgnore(tempDir)
	if err != nil {
		t.Fatalf("Failed to load .ciloignore: %v", err)
	}

	defaults := DefaultExcludePatterns()
	if len(patterns) != len(defaults) {
		t.Errorf("Expected %d default patterns from comments-only file, got %d", len(defaults), len(patterns))
	}
}

func TestLoadProjectIgnore_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()
	ignoreFile := filepath.Join(tempDir, ".ciloignore")

	if err := os.WriteFile(ignoreFile, []byte("*.tmp"), 0644); err != nil {
		t.Fatalf("Failed to create .ciloignore: %v", err)
	}

	if err := os.Chmod(ignoreFile, 0000); err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}
	defer os.Chmod(ignoreFile, 0644)

	_, err := LoadProjectIgnore(tempDir)
	if err == nil {
		t.Error("Expected error when .ciloignore is not readable, got nil")
	}
}

func TestEstimateWorkspaceSize_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()

	size, err := EstimateWorkspaceSize(tempDir, []string{})
	if err != nil {
		t.Fatalf("Failed to estimate empty dir size: %v", err)
	}

	if size != 0 {
		t.Errorf("Expected size 0 for empty directory, got %d", size)
	}
}

func TestEstimateWorkspaceSize_WithFiles(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := map[string]int{
		"file1.txt":        100,
		"file2.txt":        200,
		"subdir/file3.txt": 50,
	}

	var expectedSize int64
	for name, size := range testFiles {
		path := filepath.Join(tempDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		content := make([]byte, size)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		expectedSize += int64(size)
	}

	size, err := EstimateWorkspaceSize(tempDir, []string{})
	if err != nil {
		t.Fatalf("Failed to estimate size: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestEstimateWorkspaceSize_WithExclusions(t *testing.T) {
	tempDir := t.TempDir()

	files := []struct {
		path string
		size int
	}{
		{"include.txt", 100},
		{"exclude.tmp", 200},
		{"subdir/include.txt", 50},
		{"subdir/exclude.tmp", 75},
	}

	var expectedSize int64
	for _, f := range files {
		path := filepath.Join(tempDir, f.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		content := make([]byte, f.size)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if !strings.Contains(f.path, ".tmp") {
			expectedSize += int64(f.size)
		}
	}

	size, err := EstimateWorkspaceSize(tempDir, []string{"*.tmp"})
	if err != nil {
		t.Fatalf("Failed to estimate size: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Expected size %d (excluding .tmp files), got %d", expectedSize, size)
	}
}

func TestEstimateWorkspaceSize_ExcludeDirectory(t *testing.T) {
	tempDir := t.TempDir()

	files := []struct {
		path string
		size int
	}{
		{"include/file1.txt", 100},
		{"include/file2.txt", 200},
		{"exclude/file3.txt", 500},
		{"exclude/nested/file4.txt", 600},
	}

	var expectedSize int64
	for _, f := range files {
		path := filepath.Join(tempDir, f.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		content := make([]byte, f.size)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if strings.HasPrefix(f.path, "include/") {
			expectedSize += int64(f.size)
		}
	}

	size, err := EstimateWorkspaceSize(tempDir, []string{"exclude"})
	if err != nil {
		t.Fatalf("Failed to estimate size: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Expected size %d (excluding 'exclude' dir), got %d", expectedSize, size)
	}
}

func TestEstimateWorkspaceSize_NonExistentPath(t *testing.T) {
	nonExistentPath := filepath.Join(t.TempDir(), "does_not_exist")

	// filepath.Walk doesn't error on non-existent paths, it just returns 0 size
	size, err := EstimateWorkspaceSize(nonExistentPath, []string{})
	if err != nil {
		t.Fatalf("Unexpected error for non-existent path: %v", err)
	}

	// Should return 0 size for non-existent path
	if size != 0 {
		t.Errorf("Expected size 0 for non-existent path, got %d", size)
	}
}

func TestBuildRsyncCommand_Basic(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:       tempDir,
		RemoteHost:      "test.example.com",
		RemoteUser:      "testuser",
		RemotePath:      "/remote/path",
		ExcludePatterns: []string{"*.tmp", "*.log"},
		SSHKey:          "/path/to/key",
		Verbose:         true,
	}

	cmd, args, remote, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	if cmd != "rsync" {
		t.Errorf("Expected command 'rsync', got %q", cmd)
	}

	if remote != "testuser@test.example.com:/remote/path" {
		t.Errorf("Expected remote 'testuser@test.example.com:/remote/path', got %q", remote)
	}

	if !contains(args, "-avz") {
		t.Error("Expected -avz flag in rsync command")
	}
	if !contains(args, "--delete") {
		t.Error("Expected --delete flag in rsync command")
	}
	if !contains(args, "--progress") {
		t.Error("Expected --progress flag in rsync command")
	}
	if !contains(args, "-v") {
		t.Error("Expected -v flag for verbose mode")
	}

	if !contains(args, "--exclude") {
		t.Error("Expected --exclude flag")
	}
	if !contains(args, "*.tmp") {
		t.Error("Expected *.tmp exclude pattern")
	}
	if !contains(args, "*.log") {
		t.Error("Expected *.log exclude pattern")
	}

	sshFound := false
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) {
			if strings.Contains(args[i+1], "ssh") && strings.Contains(args[i+1], "/path/to/key") {
				sshFound = true
				break
			}
		}
	}
	if !sshFound {
		t.Error("Expected SSH options with key path")
	}

	sourceFound := false
	for _, arg := range args {
		if strings.HasPrefix(arg, tempDir) && strings.HasSuffix(arg, "/") {
			sourceFound = true
			break
		}
	}
	if !sourceFound {
		t.Error("Expected source path to end with /")
	}

	destFound := false
	for _, arg := range args {
		if arg == "testuser@test.example.com:/remote/path" {
			destFound = true
			break
		}
	}
	if !destFound {
		t.Error("Expected destination in format user@host:path")
	}
}

func TestBuildRsyncCommand_DefaultExcludes(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:       tempDir,
		RemoteHost:      "test.example.com",
		RemoteUser:      "testuser",
		RemotePath:      "/remote/path",
		ExcludePatterns: []string{},
	}

	_, args, _, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	excludeCount := 0
	for _, arg := range args {
		if arg == "--exclude" {
			excludeCount++
		}
	}

	defaults := DefaultExcludePatterns()
	if excludeCount != len(defaults) {
		t.Errorf("Expected %d default exclude patterns, got %d", len(defaults), excludeCount)
	}
}

func TestBuildRsyncCommand_NoSSHKey(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:  tempDir,
		RemoteHost: "test.example.com",
		RemoteUser: "testuser",
		RemotePath: "/remote/path",
	}

	_, args, _, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	sshFound := false
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) {
			sshOpt := args[i+1]
			if strings.Contains(sshOpt, "ssh") &&
				strings.Contains(sshOpt, "StrictHostKeyChecking=no") &&
				!strings.Contains(sshOpt, "-i") {
				sshFound = true
				break
			}
		}
	}
	if !sshFound {
		t.Error("Expected SSH options without key when SSHKey is empty")
	}
}

func TestBuildRsyncCommand_NonExistentPath(t *testing.T) {
	cfg := SyncConfig{
		LocalPath:  "/path/that/does/not/exist",
		RemoteHost: "test.example.com",
		RemoteUser: "testuser",
		RemotePath: "/remote/path",
	}

	_, _, _, err := BuildRsyncCommand(cfg)
	if err == nil {
		t.Error("Expected error for non-existent local path, got nil")
	}

	if !strings.Contains(err.Error(), "local path does not exist") {
		t.Errorf("Expected 'local path does not exist' error, got: %v", err)
	}
}

func TestBuildRsyncCommand_PathWithTrailingSlash(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:       tempDir + "/",
		RemoteHost:      "test.example.com",
		RemoteUser:      "testuser",
		RemotePath:      "/remote/path",
		ExcludePatterns: []string{".git"},
	}

	_, args, _, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	doubleSlashFound := false
	for _, arg := range args {
		if strings.Contains(arg, "//") {
			doubleSlashFound = true
			break
		}
	}
	if doubleSlashFound {
		t.Error("Path should not contain double slashes")
	}
}

func TestBuildRsyncCommand_NonVerbose(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:       tempDir,
		RemoteHost:      "test.example.com",
		RemoteUser:      "testuser",
		RemotePath:      "/remote/path",
		ExcludePatterns: []string{".git"},
		Verbose:         false,
	}

	_, args, _, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	standaloneVFound := false
	for _, arg := range args {
		if arg == "-v" {
			standaloneVFound = true
			break
		}
	}
	if standaloneVFound {
		t.Error("Should not have standalone -v flag when Verbose is false")
	}
}

func TestSyncWorkspace_NonExistentPath(t *testing.T) {
	cfg := SyncConfig{
		LocalPath:  "/path/that/does/not/exist",
		RemoteHost: "test.example.com",
		RemoteUser: "testuser",
		RemotePath: "/remote/path",
	}

	err := SyncWorkspace(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error for non-existent local path, got nil")
	}

	if !strings.Contains(err.Error(), "local path does not exist") {
		t.Errorf("Expected 'local path does not exist' error, got: %v", err)
	}
}

func TestSyncWorkspaceViaWireGuard(t *testing.T) {
	tempDir := t.TempDir()

	cfg := SyncConfig{
		LocalPath:       tempDir,
		RemoteHost:      "10.0.0.1",
		RemoteUser:      "root",
		RemotePath:      "/remote/workspace",
		ExcludePatterns: DefaultExcludePatterns(),
	}

	_, args, remote, err := BuildRsyncCommand(cfg)
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	if remote != "root@10.0.0.1:/remote/workspace" {
		t.Errorf("Expected destination to use WireGuard IP and root user, got %q", remote)
	}

	if !contains(args, "root@10.0.0.1:/remote/workspace") {
		t.Error("Expected destination in args")
	}
}

func TestEstimateWorkspaceSize_WithSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tempDir := t.TempDir()

	regularFile := filepath.Join(tempDir, "regular.txt")
	if err := os.WriteFile(regularFile, make([]byte, 100), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	symlink := filepath.Join(tempDir, "link.txt")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	size, err := EstimateWorkspaceSize(tempDir, []string{})
	if err != nil {
		t.Fatalf("Failed to estimate size: %v", err)
	}

	if size < 100 {
		t.Errorf("Expected size at least 100, got %d", size)
	}
}

func TestLoadProjectIgnore_WhitespaceHandling(t *testing.T) {
	tempDir := t.TempDir()
	ignoreFile := filepath.Join(tempDir, ".ciloignore")

	content := `  pattern1  
	pattern2	
   pattern3
`
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create .ciloignore: %v", err)
	}

	patterns, err := LoadProjectIgnore(tempDir)
	if err != nil {
		t.Fatalf("Failed to load .ciloignore: %v", err)
	}

	defaults := DefaultExcludePatterns()
	expected := []string{"pattern1", "pattern2", "pattern3"}

	for i, exp := range expected {
		idx := len(defaults) + i
		if idx >= len(patterns) {
			t.Errorf("Missing pattern at index %d: %s", idx, exp)
			continue
		}
		if patterns[idx] != exp {
			t.Errorf("Pattern at index %d: expected %q, got %q", idx, exp, patterns[idx])
		}
	}
}

func TestSyncWorkspace_MockedExecution(t *testing.T) {
	tempDir := t.TempDir()

	originalExec := defaultExecCommand
	executed := false
	var capturedArgs []string

	defaultExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		executed = true
		capturedArgs = arg
		return exec.CommandContext(ctx, "echo", "success")
	}
	defer func() { defaultExecCommand = originalExec }()

	cfg := SyncConfig{
		LocalPath:       tempDir,
		RemoteHost:      "test.example.com",
		RemoteUser:      "testuser",
		RemotePath:      "/remote/path",
		ExcludePatterns: []string{".git"},
	}

	err := SyncWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatalf("SyncWorkspace failed: %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	if !contains(capturedArgs, "-avz") {
		t.Error("Expected -avz in executed command args")
	}
}

func TestSyncWorkspace_MockedExecutionFailure(t *testing.T) {
	tempDir := t.TempDir()

	originalExec := defaultExecCommand
	defaultExecCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}
	defer func() { defaultExecCommand = originalExec }()

	cfg := SyncConfig{
		LocalPath:  tempDir,
		RemoteHost: "test.example.com",
		RemoteUser: "testuser",
		RemotePath: "/remote/path",
	}

	err := SyncWorkspace(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error when command fails, got nil")
	}

	if !strings.Contains(err.Error(), "rsync failed") {
		t.Errorf("Expected 'rsync failed' error, got: %v", err)
	}
}

func BenchmarkDefaultExcludePatterns(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultExcludePatterns()
	}
}

func BenchmarkEstimateWorkspaceSize(b *testing.B) {
	tempDir := b.TempDir()

	for i := 0; i < 100; i++ {
		path := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content := make([]byte, 1024)
		os.WriteFile(path, content, 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EstimateWorkspaceSize(tempDir, []string{})
	}
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
