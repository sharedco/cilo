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
	"github.com/sharedco/cilo/internal/share"
	sharestore "github.com/sharedco/cilo/internal/share/store"
	"gopkg.in/yaml.v3"
)

// EnvironmentManager handles Docker Compose operations
type EnvironmentManager struct {
	workspaceRoot string    // e.g., /var/cilo/workspaces
	proxy         *EnvProxy // reverse proxy for routing HTTP traffic
	sharedStore   sharestore.SharedServiceStore
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager(workspaceRoot string, proxy *EnvProxy, sharedStore sharestore.SharedServiceStore) *EnvironmentManager {
	if sharedStore == nil {
		sharedStore = sharestore.NewLocalStateStore()
	}

	return &EnvironmentManager{
		workspaceRoot: workspaceRoot,
		proxy:         proxy,
		sharedStore:   sharedStore,
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
	projectNameForShared, err := m.getProjectName(workspacePath)
	if err != nil {
		return nil, err
	}

	composeFiles, err := m.resolveComposeFiles(workspacePath)
	if err != nil {
		return nil, err
	}

	sharedServices, err := compose.GetServicesWithLabel(composeFiles, "cilo.share", "true")
	if err != nil {
		return nil, fmt.Errorf("failed to get shared services: %w", err)
	}
	for _, svc := range req.Shared {
		svc = strings.TrimSpace(svc)
		if svc != "" && !containsService(sharedServices, svc) {
			sharedServices = append(sharedServices, svc)
		}
	}
	sharedServices = filterServices(sharedServices, req.Isolate)

	// Create Docker network if subnet is provided
	if req.Subnet != "" {
		networkName := fmt.Sprintf("cilo_%s", req.EnvName)
		if err := m.createNetwork(ctx, networkName, req.Subnet); err != nil {
			log.Printf("Warning: failed to create network (may already exist): %v", err)
		}

		// Generate docker-compose override to attach containers to Cilo network
		if err := m.generateOverride(workspacePath, req.EnvName, req.Subnet, composeFiles, sharedServices); err != nil {
			log.Printf("Warning: failed to generate override.yml: %v", err)
		}
	}

	if err := m.writeSelectedSharedServices(workspacePath, sharedServices); err != nil {
		return nil, err
	}

	sharedIPs := make(map[string]string)
	var shareMgr *share.Manager
	if len(sharedServices) > 0 {
		shareMgr = share.NewManagerWithStore(m, m.sharedStore, ctx)
		for _, svc := range sharedServices {
			containerName, ip, err := shareMgr.EnsureSharedService(svc, projectNameForShared, composeFiles)
			if err != nil {
				return nil, fmt.Errorf("failed to ensure shared service %s: %w", svc, err)
			}

			if err := shareMgr.RegisterSharedService(svc, projectNameForShared, containerName, ip, composeFiles); err != nil {
				return nil, fmt.Errorf("failed to register shared service %s: %w", svc, err)
			}
		}
	}

	// Build docker compose command
	composeArgFiles := []string{
		"-f", "docker-compose.yml",
	}

	// Check if override file exists
	overridePath := filepath.Join(workspacePath, ".cilo", "override.yml")
	if _, err := os.Stat(overridePath); err == nil {
		composeArgFiles = append(composeArgFiles, "-f", ".cilo/override.yml")
	}

	projectName := fmt.Sprintf("cilo_%s", req.EnvName)
	args := append([]string{"-p", projectName}, composeArgFiles...)
	args = append(args, "up", "-d")

	if req.Build {
		args = append(args, "--build")
	}
	if req.Recreate {
		args = append(args, "--force-recreate")
	}

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

	if shareMgr != nil {
		for _, svc := range sharedServices {
			if err := shareMgr.ConnectSharedServiceToEnvironment(svc, projectNameForShared, req.EnvName); err != nil {
				return nil, fmt.Errorf("failed to connect shared service %s: %w", svc, err)
			}

			netIP, err := shareMgr.GetSharedServiceIP(svc, projectNameForShared, req.EnvName)
			if err != nil {
				return nil, fmt.Errorf("failed to get shared service IP %s: %w", svc, err)
			}

			if err := shareMgr.AddEnvironmentReference(svc, projectNameForShared, projectNameForShared, req.EnvName); err != nil {
				return nil, fmt.Errorf("failed to add shared service reference %s: %w", svc, err)
			}

			sharedIPs[svc] = netIP
		}
	}

	// Get service IPs
	services, err := m.getServiceIPs(ctx, workspacePath, req.EnvName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service IPs: %w", err)
	}
	for svc, ip := range sharedIPs {
		services[svc] = ip
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

	projectNameForShared, err := m.getProjectName(workspacePath)
	if err != nil {
		return err
	}

	selectedSharedServices, err := m.readSelectedSharedServices(workspacePath)
	if err != nil {
		return err
	}

	if len(selectedSharedServices) > 0 {
		shareMgr := share.NewManagerWithStore(m, m.sharedStore, ctx)
		for _, svc := range selectedSharedServices {
			if err := shareMgr.DisconnectSharedServiceFromEnvironment(svc, projectNameForShared, envName); err != nil {
				log.Printf("Warning: failed to disconnect shared service %s: %v", svc, err)
			}

			if err := shareMgr.RemoveEnvironmentReference(svc, projectNameForShared, projectNameForShared, envName); err != nil {
				log.Printf("Warning: failed to remove shared service reference %s: %v", svc, err)
			}

			if err := shareMgr.StopSharedServiceIfUnused(svc, projectNameForShared); err != nil {
				log.Printf("Warning: failed to stop shared service %s: %v", svc, err)
			}
		}
	}

	log.Printf("Stopping environment %s", envName)

	projectName := fmt.Sprintf("cilo_%s", envName)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "down")
	cmd.Dir = workspacePath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w\nstderr: %s", err, stderr.String())
	}

	if m.proxy != nil {
		m.proxy.RemoveRoutesForEnv(envName)
	}

	if err := m.clearSelectedSharedServices(workspacePath); err != nil {
		log.Printf("Warning: failed to clear shared service selections: %v", err)
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

	projectName := fmt.Sprintf("cilo_%s", envName)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "ps", "--format", "json")
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

	projectName := fmt.Sprintf("cilo_%s", envName)
	args := []string{"compose", "-p", projectName, "logs"}
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
		projectName := fmt.Sprintf("cilo_%s", envName)
		cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "down", "-v")
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

func (m *EnvironmentManager) generateOverride(workspacePath, envName, subnet string, composeFiles []string, sharedServices []string) error {
	ciloDir := filepath.Join(workspacePath, ".cilo")
	if err := os.MkdirAll(ciloDir, 0755); err != nil {
		return fmt.Errorf("failed to create .cilo directory: %w", err)
	}

	overridePath := filepath.Join(ciloDir, "override.yml")

	env := &models.Environment{
		Name:   envName,
		Subnet: subnet,
	}

	if err := compose.TransformWithShared(env, composeFiles, overridePath, ".test", sharedServices); err != nil {
		return fmt.Errorf("failed to transform compose: %w", err)
	}

	log.Printf("Generated override.yml for %s with network cilo_%s", envName, envName)
	return nil
}

func (m *EnvironmentManager) resolveComposeFiles(workspacePath string) ([]string, error) {
	projectConfig, err := models.LoadProjectConfigFromPath(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	composeFiles, _, err := compose.ResolveComposeFiles(workspacePath, nil)
	if err == nil && projectConfig != nil {
		composeFiles, _, err = compose.ResolveComposeFiles(workspacePath, projectConfig.ComposeFiles)
	}
	if err != nil {
		return nil, err
	}

	return composeFiles, nil
}

func (m *EnvironmentManager) getProjectName(workspacePath string) (string, error) {
	projectConfig, err := models.LoadProjectConfigFromPath(workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to load project config: %w", err)
	}
	if projectConfig != nil && projectConfig.Project != "" {
		return projectConfig.Project, nil
	}

	return filepath.Base(workspacePath), nil
}

func (m *EnvironmentManager) selectedSharedServicesPath(workspacePath string) string {
	return filepath.Join(workspacePath, ".cilo", "selected-shared-services.json")
}

func (m *EnvironmentManager) writeSelectedSharedServices(workspacePath string, sharedServices []string) error {
	if err := os.MkdirAll(filepath.Join(workspacePath, ".cilo"), 0755); err != nil {
		return fmt.Errorf("failed to create .cilo directory: %w", err)
	}

	path := m.selectedSharedServicesPath(workspacePath)
	data, err := json.MarshalIndent(map[string][]string{"services": sharedServices}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal shared services selection: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write shared services selection: %w", err)
	}

	return nil
}

func (m *EnvironmentManager) readSelectedSharedServices(workspacePath string) ([]string, error) {
	path := m.selectedSharedServicesPath(workspacePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			composeFiles, resolveErr := m.resolveComposeFiles(workspacePath)
			if resolveErr != nil {
				return []string{}, nil
			}

			services, listErr := compose.GetServicesWithLabel(composeFiles, "cilo.share", "true")
			if listErr != nil {
				return []string{}, nil
			}
			return services, nil
		}
		return nil, fmt.Errorf("failed to read shared services selection: %w", err)
	}

	var payload struct {
		Services []string `json:"services"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse shared services selection: %w", err)
	}

	return payload.Services, nil
}

func (m *EnvironmentManager) clearSelectedSharedServices(workspacePath string) error {
	path := m.selectedSharedServicesPath(workspacePath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func containsService(services []string, name string) bool {
	for _, svc := range services {
		if svc == name {
			return true
		}
	}
	return false
}

func filterServices(services []string, filtered []string) []string {
	if len(filtered) == 0 {
		return services
	}

	blocked := make(map[string]struct{}, len(filtered))
	for _, svc := range filtered {
		svc = strings.TrimSpace(svc)
		if svc != "" {
			blocked[svc] = struct{}{}
		}
	}

	result := make([]string, 0, len(services))
	for _, svc := range services {
		if _, found := blocked[svc]; !found {
			result = append(result, svc)
		}
	}

	return result
}

func (m *EnvironmentManager) ConnectContainerToNetwork(ctx context.Context, containerName, networkName, alias string) error {
	args := []string{"network", "connect"}
	if alias != "" {
		args = append(args, "--alias", alias)
	}
	args = append(args, networkName, containerName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "already exists") || strings.Contains(stderr.String(), "already connected") {
			return nil
		}
		return fmt.Errorf("failed to connect container %s to network %s: %w", containerName, networkName, err)
	}

	return nil
}

func (m *EnvironmentManager) DisconnectContainerFromNetwork(ctx context.Context, containerName, networkName string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "disconnect", networkName, containerName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "is not connected") || strings.Contains(stderr.String(), "No such container") {
			return nil
		}
		return fmt.Errorf("failed to disconnect container %s from network %s: %w", containerName, networkName, err)
	}

	return nil
}

func (m *EnvironmentManager) GetContainerIPForNetwork(ctx context.Context, containerName, networkName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", fmt.Sprintf("{{(index .NetworkSettings.Networks %q).IPAddress}}", networkName), containerName)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get container IP for network %s: %w", networkName, err)
	}

	ip := strings.TrimSpace(stdout.String())
	if ip == "" {
		return "", fmt.Errorf("container %s has no IP on network %s", containerName, networkName)
	}

	return ip, nil
}

func (m *EnvironmentManager) ContainerExists(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerName)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (m *EnvironmentManager) GetContainerStatus(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", containerName)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get container status for %s: %w", containerName, err)
	}

	status := strings.TrimSpace(stdout.String())
	if status == "" {
		return "", fmt.Errorf("empty container status for %s", containerName)
	}

	return status, nil
}

func (m *EnvironmentManager) StopContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return nil
		}
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}

	return nil
}

func (m *EnvironmentManager) RemoveContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", containerName)
	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return nil
		}
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}

	return nil
}

// getServiceIPs retrieves IP addresses for all services in the environment
func (m *EnvironmentManager) getServiceIPs(ctx context.Context, workspacePath, envName string) (map[string]string, error) {
	// Get list of services from docker compose
	projectName := fmt.Sprintf("cilo_%s", envName)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "ps", "--format", "json")
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

func (m *EnvironmentManager) getContainerIP(ctx context.Context, containerName, envName string) (string, error) {
	projectName := fmt.Sprintf("cilo_%s", envName)
	networks := []string{
		projectName,
		fmt.Sprintf("%s_default", projectName),
		fmt.Sprintf("%s_default", envName),
	}

	for _, network := range networks {
		cmd := exec.CommandContext(ctx, "docker", "inspect",
			"--format", fmt.Sprintf("{{(index .NetworkSettings.Networks %q).IPAddress}}", network),
			containerName)

		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		if err := cmd.Run(); err == nil {
			if ip := strings.TrimSpace(stdout.String()); ip != "" {
				return ip, nil
			}
		}
	}

	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerName)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker inspect failed: %w", err)
	}

	ip := strings.TrimSpace(stdout.String())
	if ip == "" {
		return "", fmt.Errorf("no IP address found for %s", containerName)
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
