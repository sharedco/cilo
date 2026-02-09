// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// EnvironmentManager handles Docker Compose operations
type EnvironmentManager struct {
	workspaceRoot string // e.g., /var/cilo/workspaces
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager(workspaceRoot string) *EnvironmentManager {
	return &EnvironmentManager{
		workspaceRoot: workspaceRoot,
	}
}

// Up starts the environment using docker compose
func (m *EnvironmentManager) Up(ctx context.Context, req UpRequest) (*UpResponse, error) {
	// Validate inputs
	if err := validateEnvName(req.EnvName); err != nil {
		return nil, fmt.Errorf("invalid env_name: %w", err)
	}

	// Build workspace path
	workspacePath := filepath.Join(m.workspaceRoot, req.EnvName)
	if req.WorkspacePath != "" {
		workspacePath = req.WorkspacePath
	}

	// Verify workspace exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace does not exist: %s", workspacePath)
	}

	log.Printf("Starting environment %s in workspace %s", req.EnvName, workspacePath)

	// Create Docker network if subnet is provided
	if req.Subnet != "" {
		networkName := fmt.Sprintf("cilo_%s", req.EnvName)
		if err := m.createNetwork(ctx, networkName, req.Subnet); err != nil {
			log.Printf("Warning: failed to create network (may already exist): %v", err)
		}
	}

	// Build docker compose command
	composeFiles := []string{
		"-f", "docker-compose.yml",
	}

	// Check if override file exists
	overridePath := filepath.Join(workspacePath, ".cilo", "override.yml")
	if _, err := os.Stat(overridePath); err == nil {
		composeFiles = append(composeFiles, "-f", ".cilo/override.yml")
	}

	args := append(composeFiles, "up", "-d")

	// Add optional flags
	if req.Build {
		args = append(args, "--build")
	}
	if req.Recreate {
		args = append(args, "--force-recreate")
	}

	// Execute docker compose up
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = workspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Running: docker compose %s", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker compose up failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	log.Printf("Docker compose up completed for %s", req.EnvName)

	// Get service IPs
	services, err := m.getServiceIPs(ctx, workspacePath, req.EnvName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service IPs: %w", err)
	}

	return &UpResponse{
		Status:   "running",
		Services: services,
	}, nil
}

// Down stops the environment
func (m *EnvironmentManager) Down(ctx context.Context, envName string) error {
	if err := validateEnvName(envName); err != nil {
		return fmt.Errorf("invalid env_name: %w", err)
	}

	workspacePath := filepath.Join(m.workspaceRoot, envName)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace does not exist: %s", workspacePath)
	}

	log.Printf("Stopping environment %s", envName)

	cmd := exec.CommandContext(ctx, "docker", "compose", "down")
	cmd.Dir = workspacePath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Printf("Environment %s stopped successfully", envName)
	return nil
}

// Status returns container status for all services
func (m *EnvironmentManager) Status(ctx context.Context, envName string) (map[string]ServiceStatus, error) {
	if err := validateEnvName(envName); err != nil {
		return nil, fmt.Errorf("invalid env_name: %w", err)
	}

	workspacePath := filepath.Join(m.workspaceRoot, envName)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace does not exist: %s", workspacePath)
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "json")
	cmd.Dir = workspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker compose ps failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output (one JSON object per line)
	statuses := make(map[string]ServiceStatus)
	scanner := bufio.NewScanner(&stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var container struct {
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Health  string `json:"Health"`
		}

		if err := json.Unmarshal([]byte(line), &container); err != nil {
			log.Printf("Warning: failed to parse container status: %v", err)
			continue
		}

		statuses[container.Service] = ServiceStatus{
			Service: container.Service,
			State:   container.State,
			Status:  container.Status,
			Health:  container.Health,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read docker compose ps output: %w", err)
	}

	return statuses, nil
}

// Logs returns a reader for service logs
func (m *EnvironmentManager) Logs(ctx context.Context, envName, service string, follow bool) (io.ReadCloser, error) {
	if err := validateEnvName(envName); err != nil {
		return nil, fmt.Errorf("invalid env_name: %w", err)
	}

	workspacePath := filepath.Join(m.workspaceRoot, envName)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace does not exist: %s", workspacePath)
	}

	args := []string{"compose", "logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, service)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspacePath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker compose logs: %w", err)
	}

	// Return a ReadCloser that also cleans up the command
	return &logReader{
		reader: stdout,
		cmd:    cmd,
	}, nil
}

// Destroy removes the environment completely
func (m *EnvironmentManager) Destroy(ctx context.Context, envName string) error {
	if err := validateEnvName(envName); err != nil {
		return fmt.Errorf("invalid env_name: %w", err)
	}

	workspacePath := filepath.Join(m.workspaceRoot, envName)

	log.Printf("Destroying environment %s", envName)

	// Stop containers first
	if _, err := os.Stat(workspacePath); err == nil {
		cmd := exec.CommandContext(ctx, "docker", "compose", "down", "-v")
		cmd.Dir = workspacePath
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: docker compose down failed: %v", err)
		}
	}

	// Remove network
	networkName := fmt.Sprintf("cilo_%s", envName)
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", networkName)
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: failed to remove network %s: %v", networkName, err)
	}

	// Remove workspace
	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to remove workspace: %w", err)
	}

	log.Printf("Environment %s destroyed successfully", envName)
	return nil
}

// createNetwork creates a Docker network with the specified subnet
func (m *EnvironmentManager) createNetwork(ctx context.Context, name, subnet string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "create",
		"--driver", "bridge",
		"--subnet", subnet,
		name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Network might already exist, which is fine
		if strings.Contains(stderr.String(), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create network: %w\nstderr: %s", err, stderr.String())
	}

	log.Printf("Created Docker network %s with subnet %s", name, subnet)
	return nil
}

// getServiceIPs retrieves IP addresses for all services in the environment
func (m *EnvironmentManager) getServiceIPs(ctx context.Context, workspacePath, envName string) (map[string]string, error) {
	// Get list of services from docker compose
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "json")
	cmd.Dir = workspacePath

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make(map[string]string)
	scanner := bufio.NewScanner(&stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var container struct {
			Service string `json:"Service"`
			Name    string `json:"Name"`
		}

		if err := json.Unmarshal([]byte(line), &container); err != nil {
			log.Printf("Warning: failed to parse container info: %v", err)
			continue
		}

		// Get container IP using docker inspect
		ip, err := m.getContainerIP(ctx, container.Name, envName)
		if err != nil {
			log.Printf("Warning: failed to get IP for %s: %v", container.Name, err)
			continue
		}

		services[container.Service] = ip
	}

	return services, nil
}

// getContainerIP retrieves the IP address of a container from the Cilo network
func (m *EnvironmentManager) getContainerIP(ctx context.Context, containerName, envName string) (string, error) {
	ciloNetworkName := fmt.Sprintf("cilo_%s", envName)

	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", fmt.Sprintf("{{range .NetworkSettings.Networks}}{{if eq .Name %q}}{{.IPAddress}}{{end}}{{end}}", ciloNetworkName),
		containerName)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker inspect failed: %w", err)
	}

	ip := strings.TrimSpace(stdout.String())
	if ip == "" {
		return "", fmt.Errorf("no IP address found on network %s", ciloNetworkName)
	}

	return ip, nil
}

// validateEnvName ensures environment name contains only safe characters
func validateEnvName(name string) error {
	if name == "" {
		return fmt.Errorf("environment name cannot be empty")
	}

	// Allow alphanumeric, hyphens, and underscores
	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", name)
	if !match {
		return fmt.Errorf("environment name must contain only alphanumeric characters, hyphens, and underscores")
	}

	return nil
}

// logReader wraps an io.ReadCloser and ensures the command is cleaned up
type logReader struct {
	reader io.ReadCloser
	cmd    *exec.Cmd
}

func (l *logReader) Read(p []byte) (int, error) {
	return l.reader.Read(p)
}

func (l *logReader) Close() error {
	l.reader.Close()
	// Kill the process if still running
	if l.cmd.Process != nil {
		l.cmd.Process.Kill()
	}
	return nil
}
