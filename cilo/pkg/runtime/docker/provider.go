package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cilo/cilo/pkg/compose"
	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/models"
	"github.com/cilo/cilo/pkg/runtime"
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
