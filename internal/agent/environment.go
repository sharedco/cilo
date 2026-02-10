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

	"github.com/sharedco/cilo/internal/compose"
	"github.com/sharedco/cilo/internal/models"
	"gopkg.in/yaml.v3"
)

// EnvironmentManager handles Docker Compose operations
type EnvironmentManager struct {
	workspaceRoot string    // e.g., /var/cilo/workspaces
	proxy         *EnvProxy // reverse proxy for routing HTTP traffic
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager(workspaceRoot string, proxy *EnvProxy) *EnvironmentManager {
	return &EnvironmentManager{
		workspaceRoot: workspaceRoot,
		proxy:         proxy,
	}
}

// List returns all environments in the workspace root
func (m *EnvironmentManager) List(ctx context.Context) ([]EnvironmentInfo, error) {
	entries, err := os.ReadDir(m.workspaceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []EnvironmentInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var envs []EnvironmentInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := validateEnvName(name); err != nil {
			continue
		}

		info, err := m.getEnvironmentInfo(ctx, name)
		if err != nil {
			log.Printf("Warning: failed to get info for environment %s: %v", name, err)
			continue
		}
		envs = append(envs, info)
	}

	return envs, nil
}

// getEnvironmentInfo returns detailed info about a single environment
func (m *EnvironmentManager) getEnvironmentInfo(ctx context.Context, name string) (EnvironmentInfo, error) {
	workspacePath := filepath.Join(m.workspaceRoot, name)
	info := EnvironmentInfo{
		Name:   name,
		Status: "unknown",
	}

	// Get creation time from directory
	if stat, err := os.Stat(workspacePath); err == nil {
		info.CreatedAt = stat.ModTime()
	}

	// Get status from docker compose
	statuses, err := m.Status(ctx, name)
	if err != nil {
		info.Status = "error"
		return info, nil
	}

	// Determine overall status
	if len(statuses) == 0 {
		info.Status = "stopped"
	} else {
		running := 0
		for _, s := range statuses {
			info.Services = append(info.Services, s.Service)
			if s.State == "running" {
				running++
			}
		}
		if running == len(statuses) {
			info.Status = "running"
		} else if running > 0 {
			info.Status = "partial"
		} else {
			info.Status = "stopped"
		}
	}

	return info, nil
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

		// Generate docker-compose override to attach containers to Cilo network
		if err := m.generateOverride(workspacePath, req.EnvName, req.Subnet); err != nil {
			log.Printf("Warning: failed to generate override.yml: %v", err)
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

	// Register proxy routes for each service
	if m.proxy != nil {
		for name, ip := range services {
			port := m.detectServiceHTTPPort(ctx, workspacePath, name)
			hostname := fmt.Sprintf("%s.%s.test", name, req.EnvName)
			target := fmt.Sprintf("http://%s:%s", ip, port)
			if err := m.proxy.AddRoute(hostname, target); err != nil {
				log.Printf("Warning: failed to register proxy route for %s: %v", hostname, err)
			}
		}
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

	if m.proxy != nil {
		m.proxy.RemoveRoutesForEnv(envName)
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

	if m.proxy != nil {
		m.proxy.RemoveRoutesForEnv(envName)
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
		if strings.Contains(stderr.String(), "already exists") {
			if m.networkSubnetMatches(ctx, name, subnet) {
				return nil
			}
			log.Printf("Network %s exists with wrong subnet, recreating with %s", name, subnet)
			rmCmd := exec.CommandContext(ctx, "docker", "network", "rm", name)
			if rmErr := rmCmd.Run(); rmErr != nil {
				return fmt.Errorf("failed to remove stale network %s: %w", name, rmErr)
			}
			retryCmd := exec.CommandContext(ctx, "docker", "network", "create",
				"--driver", "bridge",
				"--subnet", subnet,
				name)
			var retryStderr bytes.Buffer
			retryCmd.Stderr = &retryStderr
			if retryErr := retryCmd.Run(); retryErr != nil {
				return fmt.Errorf("failed to recreate network: %w\nstderr: %s", retryErr, retryStderr.String())
			}
			log.Printf("Recreated Docker network %s with subnet %s", name, subnet)
			return nil
		}
		return fmt.Errorf("failed to create network: %w\nstderr: %s", err, stderr.String())
	}

	log.Printf("Created Docker network %s with subnet %s", name, subnet)
	return nil
}

func (m *EnvironmentManager) networkSubnetMatches(ctx context.Context, name, expectedSubnet string) bool {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", name, "--format", "{{range .IPAM.Config}}{{.Subnet}}{{end}}")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.TrimSpace(stdout.String()) == expectedSubnet
}

func (m *EnvironmentManager) generateOverride(workspacePath, envName, subnet string) error {
	ciloDir := filepath.Join(workspacePath, ".cilo")
	if err := os.MkdirAll(ciloDir, 0755); err != nil {
		return fmt.Errorf("failed to create .cilo directory: %w", err)
	}

	overridePath := filepath.Join(ciloDir, "override.yml")
	baseFiles := []string{filepath.Join(workspacePath, "docker-compose.yml")}

	env := &models.Environment{
		Name:   envName,
		Subnet: subnet,
	}

	if err := compose.Transform(env, baseFiles, overridePath, ".test"); err != nil {
		return fmt.Errorf("failed to transform compose: %w", err)
	}

	log.Printf("Generated override.yml for %s with network cilo_%s", envName, envName)
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
		"--format", fmt.Sprintf("{{(index .NetworkSettings.Networks %q).IPAddress}}", ciloNetworkName),
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

func (m *EnvironmentManager) detectServiceHTTPPort(ctx context.Context, workspacePath, serviceName string) string {
	composePath := filepath.Join(workspacePath, "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		return defaultHTTPPortForService(serviceName)
	}

	var root struct {
		Services map[string]struct {
			Ports []interface{} `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return defaultHTTPPortForService(serviceName)
	}

	svc, ok := root.Services[serviceName]
	if !ok || len(svc.Ports) == 0 {
		return defaultHTTPPortForService(serviceName)
	}

	portStr := fmt.Sprintf("%v", svc.Ports[0])
	parts := strings.Split(portStr, ":")
	if len(parts) >= 2 {
		return strings.Split(parts[len(parts)-1], "/")[0]
	}
	return strings.Split(parts[0], "/")[0]
}

func defaultHTTPPortForService(serviceName string) string {
	s := strings.ToLower(serviceName)
	if s == "api" || strings.Contains(s, "backend") {
		return "8080"
	}
	if s == "nginx" || s == "web" || strings.Contains(s, "front") {
		return "80"
	}
	return "80"
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
