// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package reconcile

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/runtime"
	"github.com/sharedco/cilo/internal/runtime/docker"
)

// Result contains reconciliation findings
type Result struct {
	EnvsReconciled int
	EnvsNotRunning []string
	OrphanedItems  []OrphanedResource
	Errors         []error
}

// OrphanedResource represents a Docker resource not tracked in state
type OrphanedResource struct {
	Type string // "network", "container"
	Name string
	ID   string
}

// Environment reconciles a single environment's state with runtime
func Environment(ctx context.Context, env *models.Environment, provider runtime.Provider) error {
	// Get actual service status from Docker
	status, err := provider.GetServiceStatus(ctx, env.Project, env.Name)
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	// Update environment status based on running containers
	hasRunning := false
	for svcName, svcStatus := range status {
		if svcStatus == "running" {
			hasRunning = true
		}

		// Update service in env
		if svc, exists := env.Services[svcName]; exists {
			// Get updated IP if container is running
			if svcStatus == "running" {
				ip, err := provider.GetContainerIP(ctx, env.Name, svcName)
				if err == nil {
					svc.IP = ip
				}
			}
		}
	}

	// Update environment status
	if hasRunning {
		env.Status = "running"
	} else if len(status) > 0 {
		env.Status = "stopped"
	}

	return nil
}

// All reconciles all environments in state
func All(ctx context.Context, state *models.State) *Result {
	result := &Result{}
	var provider runtime.Provider = docker.NewProvider()

	// Collect all environments from all hosts
	environments := make(map[string]*models.Environment)

	for _, host := range state.Hosts {
		for envKey, env := range host.Environments {
			environments[envKey] = env
		}
	}

	// Reconcile all environments
	for envKey, env := range environments {
		if err := Environment(ctx, env, provider); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", envKey, err))
		} else {
			result.EnvsReconciled++
			if env.Status != "running" {
				result.EnvsNotRunning = append(result.EnvsNotRunning, envKey)
			}
		}
	}

	return result
}

// FindOrphans finds Docker resources with cilo labels not tracked in state
func FindOrphans(ctx context.Context, state *models.State) ([]OrphanedResource, error) {
	var orphans []OrphanedResource

	// Find orphaned networks
	networks, err := findCiloNetworks(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all environments for checking
	environments := make(map[string]*models.Environment)

	for _, host := range state.Hosts {
		for envKey, env := range host.Environments {
			environments[envKey] = env
		}
	}

	for _, net := range networks {
		// Check if network belongs to a tracked environment
		found := false
		for _, env := range environments {
			expectedNetName := fmt.Sprintf("cilo_%s", env.Name)
			if net == expectedNetName {
				found = true
				break
			}
		}
		if !found {
			orphans = append(orphans, OrphanedResource{
				Type: "network",
				Name: net,
			})
		}
	}

	return orphans, nil
}

// findCiloNetworks finds all Docker networks with cilo label
func findCiloNetworks(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "network", "ls",
		"--filter", "label=cilo=true",
		"--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var networks []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			networks = append(networks, line)
		}
	}
	return networks, nil
}
