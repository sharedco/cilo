// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package runtimes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/engine"
)

// PodmanRuntime implements engine.Runtime using Podman CLI.
// It supports both podman-compose (Python) and "podman compose" (Go plugin).
type PodmanRuntime struct {
	composeBinary string // "podman-compose" or "podman compose"
	podmanBinary  string
}

// NewPodmanRuntime creates a new Podman runtime.
// Returns an error if Podman or a compose tool is not available.
func NewPodmanRuntime() (*PodmanRuntime, error) {
	r := &PodmanRuntime{
		podmanBinary: "podman",
	}

	// Check if podman is available
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, fmt.Errorf("podman not found in PATH")
	}

	// Try podman-compose first (Python implementation)
	if _, err := exec.LookPath("podman-compose"); err == nil {
		r.composeBinary = "podman-compose"
	} else {
		// Try "podman compose" subcommand (Go plugin)
		cmd := exec.Command("podman", "compose", "version")
		if err := cmd.Run(); err == nil {
			r.composeBinary = "podman compose"
		} else {
			return nil, fmt.Errorf("neither podman-compose nor 'podman compose' found")
		}
	}

	return r, nil
}

// Name returns the runtime identifier.
func (r *PodmanRuntime) Name() string {
	return "podman"
}

// Up starts all services in the environment.
func (r *PodmanRuntime) Up(ctx context.Context, spec *engine.EnvironmentSpec, opts engine.UpOptions) error {
	// Get the working directory (should contain the compose file)
	composeFile := spec.SourcePath
	if composeFile == "" {
		return fmt.Errorf("source path required")
	}

	workdir := filepath.Dir(composeFile)

	// Build compose command
	args := r.buildComposeArgs(workdir, spec.Name, composeFile)
	args = append(args, "up")

	if opts.Detach {
		args = append(args, "-d")
	}

	if opts.Build {
		args = append(args, "--build")
	}

	if opts.ForceRecreate {
		args = append(args, "--force-recreate")
	} else if opts.Recreate {
		args = append(args, "--force-recreate")
	}

	if opts.NoDeps {
		args = append(args, "--no-deps")
	}

	if opts.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}

	if opts.QuietPull {
		args = append(args, "--quiet-pull")
	}

	if opts.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", opts.Timeout))
	}

	return r.runCompose(ctx, workdir, args...)
}

// Down stops all services in the environment.
func (r *PodmanRuntime) Down(ctx context.Context, spec *engine.EnvironmentSpec) error {
	composeFile := spec.SourcePath
	if composeFile == "" {
		return fmt.Errorf("source path required")
	}

	workdir := filepath.Dir(composeFile)

	args := r.buildComposeArgs(workdir, spec.Name, composeFile)
	args = append(args, "down")

	return r.runCompose(ctx, workdir, args...)
}

// Destroy removes all resources for the environment.
func (r *PodmanRuntime) Destroy(ctx context.Context, spec *engine.EnvironmentSpec) error {
	composeFile := spec.SourcePath
	if composeFile == "" {
		return fmt.Errorf("source path required")
	}

	workdir := filepath.Dir(composeFile)

	args := r.buildComposeArgs(workdir, spec.Name, composeFile)
	args = append(args, "down", "-v", "--remove-orphans")

	if err := r.runCompose(ctx, workdir, args...); err != nil {
		return err
	}

	// Also try to remove the network
	networkName := fmt.Sprintf("cilo_%s", spec.Name)
	_ = r.removeNetwork(ctx, networkName) // Ignore error if network doesn't exist

	return nil
}

// Status returns the current status of all services.
func (r *PodmanRuntime) Status(ctx context.Context, spec *engine.EnvironmentSpec) ([]engine.ServiceStatus, error) {
	composeFile := spec.SourcePath
	if composeFile == "" {
		return nil, fmt.Errorf("source path required")
	}

	workdir := filepath.Dir(composeFile)

	// Use podman ps to get container status
	args := r.buildComposeArgs(workdir, spec.Name, composeFile)
	args = append(args, "ps", "--format", "json")

	cmd := r.buildCommand(ctx, workdir, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Parse JSON output
	var containers []struct {
		Name    string   `json:"Name"`
		State   string   `json:"State"`
		Status  string   `json:"Status"`
		Image   string   `json:"Image"`
		Ports   []string `json:"Ports"`
		Created int64    `json:"CreatedAt"`
	}

	if err := json.Unmarshal(output, &containers); err != nil {
		// Try parsing as array of objects
		if err := json.Unmarshal(output, &containers); err != nil {
			return nil, fmt.Errorf("failed to parse status: %w", err)
		}
	}

	statuses := make([]engine.ServiceStatus, 0, len(containers))
	for _, c := range containers {
		status := engine.ServiceStatus{
			Name:      extractServiceName(c.Name),
			State:     c.State,
			Status:    c.Status,
			Container: c.Name,
			CreatedAt: time.Unix(c.Created, 0),
			Ports:     parsePortMappings(c.Ports),
		}

		// Get container IP if available
		ip, _ := r.getContainerIP(ctx, c.Name)
		status.IP = ip

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Logs retrieves logs from a specific service.
func (r *PodmanRuntime) Logs(ctx context.Context, spec *engine.EnvironmentSpec, service string, opts engine.LogOptions) (io.ReadCloser, error) {
	composeFile := spec.SourcePath
	if composeFile == "" {
		return nil, fmt.Errorf("source path required")
	}

	workdir := filepath.Dir(composeFile)

	args := r.buildComposeArgs(workdir, spec.Name, composeFile)
	args = append(args, "logs")

	if opts.Follow {
		args = append(args, "-f")
	}

	if opts.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Tail))
	}

	if opts.Timestamps {
		args = append(args, "-t")
	}

	if !opts.Since.IsZero() {
		args = append(args, "--since", opts.Since.Format(time.RFC3339))
	}

	if !opts.Until.IsZero() {
		args = append(args, "--until", opts.Until.Format(time.RFC3339))
	}

	args = append(args, service)

	cmd := r.buildCommand(ctx, workdir, args...)

	// Create pipe for output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start logs: %w", err)
	}

	return stdout, nil
}

// Exec executes a command in a running service container.
func (r *PodmanRuntime) Exec(ctx context.Context, spec *engine.EnvironmentSpec, service string, cmd []string, opts engine.ExecOptions) error {
	// Get container name for service
	containerName := fmt.Sprintf("%s_%s_1", spec.Name, service)

	args := []string{"exec"}

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.TTY {
		args = append(args, "-t")
	}

	if opts.Detach {
		args = append(args, "-d")
	}

	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--workdir", opts.WorkingDir)
	}

	if opts.Privileged {
		args = append(args, "--privileged")
	}

	for key, val := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, val))
	}

	args = append(args, containerName)
	args = append(args, cmd...)

	execCmd := exec.CommandContext(ctx, r.podmanBinary, args...)
	execCmd.Stdin = opts.Stdin
	execCmd.Stdout = opts.Stdout
	execCmd.Stderr = opts.Stderr

	return execCmd.Run()
}

// CreateNetwork creates an isolated network for the environment.
func (r *PodmanRuntime) CreateNetwork(ctx context.Context, spec *engine.EnvironmentSpec, subnet string) error {
	networkName := fmt.Sprintf("cilo_%s", spec.Name)

	// Check if network already exists
	checkCmd := exec.CommandContext(ctx, r.podmanBinary, "network", "inspect", networkName)
	if err := checkCmd.Run(); err == nil {
		// Network exists, skip creation
		return nil
	}

	args := []string{"network", "create", "--driver", "bridge"}

	if subnet != "" {
		args = append(args, "--subnet", subnet)
	}

	args = append(args, networkName)

	cmd := exec.CommandContext(ctx, r.podmanBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network: %w: %s", err, string(output))
	}

	return nil
}

// RemoveNetwork removes the environment's network.
func (r *PodmanRuntime) RemoveNetwork(ctx context.Context, spec *engine.EnvironmentSpec) error {
	networkName := fmt.Sprintf("cilo_%s", spec.Name)
	return r.removeNetwork(ctx, networkName)
}

// GetServiceIPs returns the IP addresses of all services.
func (r *PodmanRuntime) GetServiceIPs(ctx context.Context, spec *engine.EnvironmentSpec) (map[string]string, error) {
	statuses, err := r.Status(ctx, spec)
	if err != nil {
		return nil, err
	}

	ips := make(map[string]string)
	for _, status := range statuses {
		if status.IP != "" {
			ips[status.Name] = status.IP
		}
	}

	return ips, nil
}

// Helper methods

// buildComposeArgs builds the base arguments for compose commands.
func (r *PodmanRuntime) buildComposeArgs(workdir, project, composeFile string) []string {
	var args []string

	// Handle both "podman-compose" and "podman compose"
	if strings.Contains(r.composeBinary, " ") {
		// "podman compose" - split into multiple args
		parts := strings.Fields(r.composeBinary)
		args = append(args, parts[1:]...)
	}
	// else: "podman-compose" is handled by exec.Command

	// Add project name
	args = append(args, "--project-name", project)

	// Add compose file
	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}

	return args
}

// buildCommand builds an exec.Command for compose operations.
func (r *PodmanRuntime) buildCommand(ctx context.Context, workdir string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd

	if strings.Contains(r.composeBinary, " ") {
		// "podman compose" - use podman as base command
		parts := strings.Fields(r.composeBinary)
		allArgs := append(parts[1:], args...)
		cmd = exec.CommandContext(ctx, parts[0], allArgs...)
	} else {
		// "podman-compose" - use as base command
		cmd = exec.CommandContext(ctx, r.composeBinary, args...)
	}

	cmd.Dir = workdir
	return cmd
}

// runCompose executes a compose command.
func (r *PodmanRuntime) runCompose(ctx context.Context, workdir string, args ...string) error {
	cmd := r.buildCommand(ctx, workdir, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compose command failed: %w", err)
	}

	return nil
}

// removeNetwork removes a network by name.
func (r *PodmanRuntime) removeNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, r.podmanBinary, "network", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore "network not found" errors
		if strings.Contains(string(output), "not found") || strings.Contains(string(output), "no such network") {
			return nil
		}
		return fmt.Errorf("failed to remove network: %w: %s", err, string(output))
	}
	return nil
}

// getContainerIP retrieves the IP address of a container.
func (r *PodmanRuntime) getContainerIP(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, r.podmanBinary, "inspect", "--format", "{{.NetworkSettings.IPAddress}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(output))
	return ip, nil
}

// extractServiceName extracts the service name from a container name.
// Container names are typically in the format "project_service_1"
func extractServiceName(containerName string) string {
	parts := strings.Split(containerName, "_")
	if len(parts) >= 2 {
		// Remove project prefix and instance suffix
		return parts[len(parts)-2]
	}
	return containerName
}

// parsePortMappings parses port mapping strings into PortMapping structs.
func parsePortMappings(ports []string) []engine.PortMapping {
	mappings := make([]engine.PortMapping, 0, len(ports))

	for _, port := range ports {
		// Parse formats like "0.0.0.0:8080->80/tcp" or "8080/tcp"
		var hostIP, hostPort, containerPort, protocol string

		if strings.Contains(port, "->") {
			// Published port format: "0.0.0.0:8080->80/tcp"
			parts := strings.Split(port, "->")
			if len(parts) != 2 {
				continue
			}

			// Parse host side
			hostParts := strings.Split(parts[0], ":")
			if len(hostParts) == 2 {
				hostIP = hostParts[0]
				hostPort = hostParts[1]
			} else {
				hostPort = hostParts[0]
			}

			// Parse container side
			containerParts := strings.Split(parts[1], "/")
			containerPort = containerParts[0]
			if len(containerParts) == 2 {
				protocol = containerParts[1]
			}
		} else {
			// Unpublished port format: "80/tcp"
			parts := strings.Split(port, "/")
			containerPort = parts[0]
			if len(parts) == 2 {
				protocol = parts[1]
			}
		}

		if protocol == "" {
			protocol = "tcp"
		}

		mapping := engine.PortMapping{
			Protocol: protocol,
			HostIP:   hostIP,
		}

		// Parse port numbers
		if containerPort != "" {
			fmt.Sscanf(containerPort, "%d", &mapping.ContainerPort)
		}
		if hostPort != "" {
			fmt.Sscanf(hostPort, "%d", &mapping.HostPort)
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}
