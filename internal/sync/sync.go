// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package sync

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SyncOptions configures workspace sync behavior
type SyncOptions struct {
	RemoteHost     string
	RemotePath     string
	UseRsync       bool
	ProgressWriter io.Writer
	SSHKeyPath     string
	SSHPort        int
}

// SyncWorkspace copies local files to a remote machine using rsync over SSH
// Falls back to tar+ssh if rsync is unavailable
// Uses WireGuard tunnel IP for SSH connectivity
func SyncWorkspace(localPath, remoteHost, remotePath string, opts SyncOptions) error {
	if err := validateSyncInputs(localPath, remoteHost, remotePath, &opts); err != nil {
		return err
	}

	// Read .ciloignore if present
	ciloignorePath := filepath.Join(localPath, ".ciloignore")
	var excludes []string
	if data, err := os.ReadFile(ciloignorePath); err == nil {
		excludes = parseCiloignoreContent(string(data))
	}

	// Add default excludes
	excludes = append(getDefaultExcludes(), excludes...)

	// Determine sync method
	if opts.UseRsync && isRsyncAvailable() {
		return syncWithRsync(localPath, remoteHost, remotePath, excludes, &opts)
	}

	return syncWithTarSSH(localPath, remoteHost, remotePath, excludes, &opts)
}

// validateSyncInputs validates sync inputs
func validateSyncInputs(localPath, remoteHost, remotePath string, opts *SyncOptions) error {
	if localPath == "" {
		return fmt.Errorf("local path cannot be empty")
	}

	if remoteHost == "" {
		return fmt.Errorf("remote host cannot be empty")
	}

	if remotePath == "" {
		return fmt.Errorf("remote path cannot be empty")
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local path must be a directory")
	}

	return nil
}

// validateSyncOptions validates sync options
func validateSyncOptions(opts *SyncOptions) error {
	if opts.RemoteHost == "" {
		return fmt.Errorf("remote host cannot be empty")
	}
	if opts.RemotePath == "" {
		return fmt.Errorf("remote path cannot be empty")
	}
	return nil
}

// isRsyncAvailable checks if rsync is installed and available
func isRsyncAvailable() bool {
	_, err := exec.LookPath("rsync")
	return err == nil
}

// getDefaultExcludes returns the default exclude patterns
func getDefaultExcludes() []string {
	return []string{
		".git/",
		"node_modules/",
		".cilo/",
		"__pycache__/",
		".venv/",
		".env.local",
		"*.log",
		".DS_Store",
		"Thumbs.db",
		".ciloignore",
	}
}

// parseCiloignoreContent parses .ciloignore file content (gitignore format)
func parseCiloignoreContent(content string) []string {
	var patterns []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	return patterns
}

// buildRsyncArgs builds rsync command arguments
func buildRsyncArgs(srcDir, dstDir string, excludes []string, remoteHost string) []string {
	args := []string{
		"-avz",       // archive, verbose, compress
		"--delete",   // delete files not in source
		"--checksum", // use checksum for comparison (better for incremental)
	}

	// Add excludes
	for _, exclude := range excludes {
		args = append(args, "--exclude="+exclude)
	}

	// Source directory (trailing slash for contents only)
	args = append(args, srcDir+"/")

	// Destination
	if remoteHost == "localhost" || remoteHost == "127.0.0.1" {
		// Local sync (for testing)
		args = append(args, dstDir)
	} else {
		// Remote sync via SSH
		args = append(args, remoteHost+":"+dstDir)
	}

	return args
}

// syncWithRsync performs sync using rsync over SSH
func syncWithRsync(localPath, remoteHost, remotePath string, excludes []string, opts *SyncOptions) error {
	args := buildRsyncArgs(localPath, remotePath, excludes, remoteHost)

	// Add SSH options if specified
	if opts.SSHKeyPath != "" {
		sshOpts := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
			opts.SSHKeyPath)
		args = append([]string{"-e", sshOpts}, args...)
	}

	cmd := exec.Command("rsync", args...)

	if opts.ProgressWriter != nil {
		cmd.Stdout = opts.ProgressWriter
		cmd.Stderr = opts.ProgressWriter
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	return nil
}

// syncWithTarSSH performs sync using tar over SSH (fallback method)
func syncWithTarSSH(localPath, remoteHost, remotePath string, excludes []string, opts *SyncOptions) error {
	// Build tar command with excludes
	tarArgs := []string{"-czf", "-"}

	// Add excludes
	for _, exclude := range excludes {
		tarArgs = append(tarArgs, "--exclude="+exclude)
	}

	// Add source directory contents
	tarArgs = append(tarArgs, "-C", localPath, ".")

	tarCmd := exec.Command("tar", tarArgs...)
	tarOutput, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create tar pipe: %w", err)
	}

	// Build remote command
	remoteCmd := fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s",
		remotePath, remotePath)

	var sshArgs []string
	if opts.SSHKeyPath != "" {
		sshArgs = append(sshArgs, "-i", opts.SSHKeyPath)
	}
	sshArgs = append(sshArgs,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		remoteHost,
		remoteCmd,
	)

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = tarOutput

	if opts.ProgressWriter != nil {
		sshCmd.Stdout = opts.ProgressWriter
		sshCmd.Stderr = opts.ProgressWriter
	}

	// Start tar command
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar: %w", err)
	}

	// Run SSH command
	if err := sshCmd.Run(); err != nil {
		tarCmd.Wait()
		return fmt.Errorf("ssh/tar sync failed: %w", err)
	}

	// Wait for tar to complete
	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("tar command failed: %w", err)
	}

	return nil
}

// SyncWorkspaceToMachine syncs workspace to a connected machine
// This is a convenience function that uses the machine's WireGuard IP
func SyncWorkspaceToMachine(localPath string, machineHost string, remotePath string) error {
	// For now, use the machine host directly (which should be the WG IP)
	opts := SyncOptions{
		RemoteHost: machineHost,
		RemotePath: remotePath,
		UseRsync:   true,
	}

	return SyncWorkspace(localPath, machineHost, remotePath, opts)
}
