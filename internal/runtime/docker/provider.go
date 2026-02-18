// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sharedco/cilo/internal/compose"
	"github.com/sharedco/cilo/internal/config"
	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/runtime"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) CreateNetwork(ctx context.Context, env *models.Environment) error {
	networkName := getNetworkName(env.Name)
	subnet := env.Subnet

	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkName)
	if err := cmd.Run(); err == nil {
		if err := p.RemoveNetwork(ctx, env.Name); err != nil {
			return fmt.Errorf("failed to remove existing network: %w", err)
		}
	}

	args := []string{
		"network", "create",
		"--driver", "bridge",
		"--subnet", subnet,
		"--label", "cilo=true",
		"--label", fmt.Sprintf("cilo.env=%s", env.Name),
		networkName,
	}

	cmd = exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	return nil
}

func (p *Provider) RemoveNetwork(ctx context.Context, envName string) error {
	networkName := getNetworkName(envName)
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", networkName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove network %s: %w", networkName, err)
	}
	return nil
}

func (p *Provider) Up(ctx context.Context, env *models.Environment, opts runtime.UpOptions) error {
	workspace, args, err := buildComposeArgs(env.Project, env.Name)
	if err != nil {
		return err
	}
	args = append(args, "up", "-d")

	if opts.Build {
		args = append(args, "--build")
	}

	if opts.Recreate {
		args = append(args, "--force-recreate")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start environment: %w", err)
	}

	env.Status = "running"

	return nil
}

func (p *Provider) Down(ctx context.Context, env *models.Environment) error {
	workspace, args, err := buildComposeArgs(env.Project, env.Name)
	if err != nil {
		return err
	}
	args = append(args, "down")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop environment: %w", err)
	}

	env.Status = "stopped"
	return nil
}

func (p *Provider) Destroy(ctx context.Context, env *models.Environment) error {
	workspace, args, err := buildComposeArgs(env.Project, env.Name)
	if err != nil {
		return err
	}
	overridePath := filepath.Join(workspace, ".cilo", "override.yml")
	if _, err := os.Stat(overridePath); err == nil {
		args = append(args, "down", "-v")
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Dir = workspace
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: could not stop containers: %v\n", err)
		}

		if err := p.RemoveNetwork(ctx, env.Name); err != nil {
			fmt.Printf("Warning: could not remove network: %v\n", err)
		}
	}

	env.Status = "destroyed"
	return nil
}

func (p *Provider) GetContainerIP(ctx context.Context, envName, serviceName string) (string, error) {
	containerName := fmt.Sprintf("cilo_%s_%s", envName, serviceName)

	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get IP for container %s: %w", containerName, err)
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("container %s has no IP address", containerName)
	}

	return ip, nil
}

func (p *Provider) GetContainerIPs(ctx context.Context, envName string, services []string) (map[string]string, error) {
	ips := make(map[string]string)

	for _, service := range services {
		ip, err := p.GetContainerIP(ctx, envName, service)
		if err != nil {
			continue
		}
		ips[service] = ip
	}

	return ips, nil
}

func (p *Provider) GetServiceStatus(ctx context.Context, project, envName string) (map[string]string, error) {
	workspace, args, err := buildComposeArgs(project, envName)
	if err != nil {
		return nil, err
	}
	args = append(args, "ps", "-q")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspace
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get service status: %w", err)
	}

	containers := strings.Fields(string(output))
	status := make(map[string]string)

	for _, container := range containers {
		infoCmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.Name}} {{.State.Status}}", container)
		info, err := infoCmd.Output()
		if err != nil {
			continue
		}

		parts := strings.Fields(string(info))
		if len(parts) >= 2 {
			name := strings.TrimPrefix(parts[0], "/")
			nameParts := strings.Split(name, "_")
			if len(nameParts) >= 3 {
				serviceName := nameParts[len(nameParts)-1]
				status[serviceName] = parts[1]
			}
		}
	}

	return status, nil
}

func (p *Provider) Logs(ctx context.Context, project, envName, serviceName string, opts runtime.LogOptions) error {
	workspace, args, err := buildComposeArgs(project, envName)
	if err != nil {
		return err
	}
	args = append(args, "logs")

	if opts.Follow {
		args = append(args, "-f")
	}

	if opts.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Tail))
	}

	if serviceName != "" {
		args = append(args, serviceName)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspace

	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (p *Provider) Exec(ctx context.Context, project, envName, serviceName string, command []string, opts runtime.ExecOptions) error {
	workspace, args, err := buildComposeArgs(project, envName)
	if err != nil {
		return err
	}
	args = append(args, "exec")

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.TTY {
		args = append(args, "-t")
	}

	args = append(args, serviceName)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspace

	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}

	return cmd.Run()
}

func (p *Provider) Compose(ctx context.Context, project, envName string, opts runtime.ComposeOptions) error {
	workspace, args, err := buildComposeArgs(project, envName)
	if err != nil {
		return err
	}
	fullArgs := append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	cmd.Dir = workspace

	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}

	return cmd.Run()
}

// ConnectContainerToNetwork attaches a container to a network with an alias
// The alias is critical for inter-container DNS resolution
func (p *Provider) ConnectContainerToNetwork(ctx context.Context, containerName, networkName, alias string) error {
	args := []string{"network", "connect"}
	if alias != "" {
		args = append(args, "--alias", alias)
	}
	args = append(args, networkName, containerName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect container %s to network %s: %w", containerName, networkName, err)
	}

	return nil
}

// DisconnectContainerFromNetwork removes a container from a network
func (p *Provider) DisconnectContainerFromNetwork(ctx context.Context, containerName, networkName string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "disconnect", networkName, containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disconnect container %s from network %s: %w", containerName, networkName, err)
	}

	return nil
}

// GetContainerIPForNetwork returns the IP address of a container on a specific network
func (p *Provider) GetContainerIPForNetwork(ctx context.Context, containerName, networkName string) (string, error) {
	// Get the network ID first
	networkIDCmd := exec.CommandContext(ctx, "docker", "network", "inspect", "-f", "{{.Id}}", networkName)
	networkIDOutput, err := networkIDCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get network ID for %s: %w", networkName, err)
	}
	networkID := strings.TrimSpace(string(networkIDOutput))

	// Get the IP for this specific network
	template := fmt.Sprintf("{{range .NetworkSettings.Networks}}{{if eq .NetworkID \"%s\"}}{{.IPAddress}}{{end}}{{end}}", networkID)
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", template, containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get IP for container %s on network %s: %w", containerName, networkName, err)
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("container %s has no IP address on network %s", containerName, networkName)
	}

	return ip, nil
}

// ListContainersWithLabel returns container names that have the specified label
func (p *Provider) ListContainersWithLabel(ctx context.Context, labelKey, labelValue string) ([]string, error) {
	label := fmt.Sprintf("%s=%s", labelKey, labelValue)
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("label=%s", label), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers with label %s: %w", label, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var containers []string
	for _, line := range lines {
		if line != "" {
			containers = append(containers, line)
		}
	}

	return containers, nil
}

// ContainerExists checks if a container with the given name exists
func (p *Provider) ContainerExists(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerName)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetContainerStatus returns the status of a container (running, stopped, etc.)
func (p *Provider) GetContainerStatus(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get status for container %s: %w", containerName, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// StopContainer stops a running container
func (p *Provider) StopContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}

	return nil
}

// RemoveContainer removes a container
func (p *Provider) RemoveContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}

	return nil
}

func getNetworkName(envName string) string {
	return fmt.Sprintf("cilo_%s", envName)
}

func getWorkspacePath(project, envName string) string {
	return config.GetEnvPath(project, envName)
}

func buildComposeArgs(project, envName string) (string, []string, error) {
	workspace := getWorkspacePath(project, envName)
	projectConfig, err := models.LoadProjectConfigFromPath(workspace)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load project config: %w", err)
	}

	composeFiles, projectDir, err := compose.ResolveComposeFiles(workspace, nil)
	if err == nil && projectConfig != nil {
		composeFiles, projectDir, err = compose.ResolveComposeFiles(workspace, projectConfig.ComposeFiles)
	}
	if err != nil {
		return "", nil, err
	}

	args := []string{"compose", "-p", fmt.Sprintf("cilo_%s", envName)}
	if projectDir != "" {
		args = append(args, "--project-directory", projectDir)
	}
	for _, file := range composeFiles {
		args = append(args, "-f", file)
	}
	args = append(args, "-f", filepath.Join(workspace, ".cilo", "override.yml"))

	if projectConfig != nil {
		for _, envFile := range projectConfig.EnvFiles {
			path := envFile
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspace, envFile)
			}
			args = append(args, "--env-file", path)
		}
	}

	return workspace, args, nil
}
