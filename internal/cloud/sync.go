// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cloud

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execCommandFunc is a function type for executing commands - can be mocked in tests
type execCommandFunc func(ctx context.Context, name string, arg ...string) *exec.Cmd

// defaultExecCommand is the default implementation using exec.CommandContext
var defaultExecCommand execCommandFunc = exec.CommandContext

// SyncConfig contains configuration for workspace sync
type SyncConfig struct {
	LocalPath       string
	RemoteHost      string
	RemoteUser      string
	RemotePath      string
	ExcludePatterns []string
	SSHKey          string
	Verbose         bool
}

// DefaultExcludePatterns returns the default rsync exclude patterns
func DefaultExcludePatterns() []string {
	return []string{
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
}

// BuildRsyncCommand builds the rsync command arguments without executing
// Returns the command name, arguments, and the formatted remote destination
func BuildRsyncCommand(cfg SyncConfig) (string, []string, string, error) {
	// Validate local path exists
	if _, err := os.Stat(cfg.LocalPath); err != nil {
		return "", nil, "", fmt.Errorf("local path does not exist: %s", cfg.LocalPath)
	}

	// Ensure local path ends with /
	localPath := cfg.LocalPath
	if !strings.HasSuffix(localPath, "/") {
		localPath += "/"
	}

	// Build rsync command
	args := []string{
		"-avz",       // archive, verbose, compress
		"--delete",   // delete files not in source
		"--progress", // show progress
	}

	// Add exclude patterns
	excludes := cfg.ExcludePatterns
	if len(excludes) == 0 {
		excludes = DefaultExcludePatterns()
	}
	for _, pattern := range excludes {
		args = append(args, "--exclude", pattern)
	}

	// Add SSH options if key provided
	if cfg.SSHKey != "" {
		args = append(args, "-e", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", cfg.SSHKey))
	} else {
		args = append(args, "-e", "ssh -o StrictHostKeyChecking=no")
	}

	// Add verbose flag
	if cfg.Verbose {
		args = append(args, "-v")
	}

	// Add source and destination
	remote := fmt.Sprintf("%s@%s:%s", cfg.RemoteUser, cfg.RemoteHost, cfg.RemotePath)
	args = append(args, localPath, remote)

	return "rsync", args, remote, nil
}

// SyncWorkspace syncs the local workspace to a remote machine
func SyncWorkspace(ctx context.Context, cfg SyncConfig) error {
	_, args, _, err := BuildRsyncCommand(cfg)
	if err != nil {
		return err
	}

	cmd := defaultExecCommand(ctx, "rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	return nil
}

// SyncWorkspaceViaWireGuard syncs through the WireGuard tunnel
func SyncWorkspaceViaWireGuard(ctx context.Context, localPath string, wgIP string, remotePath string) error {
	return SyncWorkspace(ctx, SyncConfig{
		LocalPath:       localPath,
		RemoteHost:      wgIP,
		RemoteUser:      "root",
		RemotePath:      remotePath,
		ExcludePatterns: DefaultExcludePatterns(),
	})
}

// LoadProjectIgnore loads exclude patterns from .ciloignore file
func LoadProjectIgnore(projectPath string) ([]string, error) {
	ignorePath := filepath.Join(projectPath, ".ciloignore")

	data, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultExcludePatterns(), nil
		}
		return nil, fmt.Errorf("read .ciloignore: %w", err)
	}

	var patterns []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	// Merge with defaults
	return append(DefaultExcludePatterns(), patterns...), nil
}

// EstimateWorkspaceSize estimates the size of the workspace to sync
func EstimateWorkspaceSize(path string, excludePatterns []string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check if should exclude
		relPath, _ := filepath.Rel(path, filePath)
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.Contains(relPath, pattern) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}
