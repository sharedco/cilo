// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package share

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/models"
	sharestore "github.com/sharedco/cilo/internal/share/store"
	"gopkg.in/yaml.v3"
)

type Provider interface {
	ConnectContainerToNetwork(ctx context.Context, containerName, networkName, alias string) error
	DisconnectContainerFromNetwork(ctx context.Context, containerName, networkName string) error
	GetContainerIPForNetwork(ctx context.Context, containerName, networkName string) (string, error)
	ContainerExists(ctx context.Context, containerName string) (bool, error)
	GetContainerStatus(ctx context.Context, containerName string) (string, error)
	StopContainer(ctx context.Context, containerName string) error
	RemoveContainer(ctx context.Context, containerName string) error
}

type Manager struct {
	provider Provider
	store    sharestore.SharedServiceStore
	ctx      context.Context
}

func NewManager(provider Provider, ctx context.Context) *Manager {
	return &Manager{
		provider: provider,
		store:    sharestore.NewLocalStateStore(),
		ctx:      ctx,
	}
}

func NewManagerWithStore(provider Provider, sharedStore sharestore.SharedServiceStore, ctx context.Context) *Manager {
	if sharedStore == nil {
		sharedStore = sharestore.NewLocalStateStore()
	}

	return &Manager{
		provider: provider,
		store:    sharedStore,
		ctx:      ctx,
	}
}

// EnsureSharedService creates or returns existing shared container
// Returns: container name, IP address, error
func (m *Manager) EnsureSharedService(serviceName, project string, composeFiles []string) (containerName, ip string, err error) {
	containerName = fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)

	// Check if container already exists
	exists, err := m.provider.ContainerExists(m.ctx, containerName)
	if err != nil {
		return "", "", fmt.Errorf("failed to check if container exists: %w", err)
	}

	if exists {
		// Check if it's running
		status, err := m.provider.GetContainerStatus(m.ctx, containerName)
		if err != nil {
			return "", "", fmt.Errorf("failed to get container status: %w", err)
		}

		// If stopped, start it
		if status != "running" {
			if err := m.startContainer(containerName); err != nil {
				return "", "", fmt.Errorf("failed to start existing container: %w", err)
			}
		}

		// Get the primary IP (from the first network)
		ip, err = m.getContainerPrimaryIP(containerName)
		if err != nil {
			return "", "", fmt.Errorf("failed to get container IP: %w", err)
		}

		return containerName, ip, nil
	}

	// Container doesn't exist, create it
	return m.createSharedService(serviceName, project, composeFiles)
}

// createSharedService creates a new shared service container
func (m *Manager) createSharedService(serviceName, project string, composeFiles []string) (containerName, ip string, err error) {
	// Load the service definition from compose files
	serviceConfig, err := m.loadServiceConfig(serviceName, composeFiles)
	if err != nil {
		return "", "", fmt.Errorf("failed to load service config: %w", err)
	}

	// Load volume definitions
	volumeDefinitions, err := m.loadVolumeDefinitions(composeFiles)
	if err != nil {
		return "", "", fmt.Errorf("failed to load volume definitions: %w", err)
	}

	// Create a temporary compose file for just this service
	tempDir, err := os.MkdirTemp("", "cilo-shared-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	containerName = fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)

	// Create isolated compose file
	sharedComposeFile := map[string]interface{}{
		"services": map[string]interface{}{
			serviceName: map[string]interface{}{
				"image":          serviceConfig.Image,
				"container_name": containerName,
				"network_mode":   "bridge", // Use default bridge, we'll attach to env networks later
				"labels": map[string]string{
					"cilo":         "true",
					"cilo.shared":  "true",
					"cilo.project": project,
					"cilo.service": serviceName,
				},
			},
		},
	}

	// Copy over relevant fields from original service
	service := sharedComposeFile["services"].(map[string]interface{})[serviceName].(map[string]interface{})

	if serviceConfig.Environment != nil {
		service["environment"] = serviceConfig.Environment
	}
	if serviceConfig.Volumes != nil && len(serviceConfig.Volumes) > 0 {
		service["volumes"] = serviceConfig.Volumes

		// Add volume definitions for named volumes used by this service
		namedVolumes := extractNamedVolumes(serviceConfig.Volumes)
		if len(namedVolumes) > 0 {
			volumes := make(map[string]interface{})
			for _, volName := range namedVolumes {
				if volDef, ok := volumeDefinitions[volName]; ok {
					volumes[volName] = volDef
				} else {
					// Create empty volume definition if not found
					volumes[volName] = map[string]interface{}{}
				}
			}
			if len(volumes) > 0 {
				sharedComposeFile["volumes"] = volumes
			}
		}
	}
	if serviceConfig.Command != nil {
		service["command"] = serviceConfig.Command
	}
	if serviceConfig.WorkingDir != "" {
		service["working_dir"] = serviceConfig.WorkingDir
	}

	// Write to temp file
	composeData, err := yaml.Marshal(sharedComposeFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal compose file: %w", err)
	}

	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write compose file: %w", err)
	}

	// Start the container using docker compose
	cmd := exec.CommandContext(m.ctx, "docker", "compose", "-f", composePath, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to start shared service: %w", err)
	}

	// Wait a moment for container to initialize
	time.Sleep(2 * time.Second)

	// Get the container IP
	ip, err = m.getContainerPrimaryIP(containerName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get container IP: %w", err)
	}

	return containerName, ip, nil
}

// loadServiceConfig loads a service definition from compose files
func (m *Manager) loadServiceConfig(serviceName string, composeFiles []string) (*models.ComposeService, error) {
	for i := len(composeFiles) - 1; i >= 0; i-- {
		data, err := os.ReadFile(composeFiles[i])
		if err != nil {
			continue
		}

		var composeFile models.ComposeFile
		if err := yaml.Unmarshal(data, &composeFile); err != nil {
			continue
		}

		if service, ok := composeFile.Services[serviceName]; ok {
			return service, nil
		}
	}

	return nil, fmt.Errorf("service %s not found in compose files", serviceName)
}

// loadVolumeDefinitions loads all volume definitions from compose files
func (m *Manager) loadVolumeDefinitions(composeFiles []string) (map[string]interface{}, error) {
	volumes := make(map[string]interface{})

	for i := len(composeFiles) - 1; i >= 0; i-- {
		data, err := os.ReadFile(composeFiles[i])
		if err != nil {
			continue
		}

		var composeFile struct {
			Volumes map[string]interface{} `yaml:"volumes"`
		}
		if err := yaml.Unmarshal(data, &composeFile); err != nil {
			continue
		}

		// Merge volumes (later files override earlier ones)
		for name, def := range composeFile.Volumes {
			volumes[name] = def
		}
	}

	return volumes, nil
}

// extractNamedVolumes extracts named volume names from volume mount specifications
// e.g., "es_data:/usr/share/elasticsearch/data" -> "es_data"
func extractNamedVolumes(volumeMounts []string) []string {
	var namedVolumes []string
	for _, mount := range volumeMounts {
		// Split on ':' to get source
		parts := strings.Split(mount, ":")
		if len(parts) >= 2 {
			source := parts[0]
			// Named volumes don't start with '/' or '.' (those are bind mounts)
			if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "~") {
				namedVolumes = append(namedVolumes, source)
			}
		}
	}
	return namedVolumes
}

// ConnectSharedServiceToEnvironment attaches shared container to env network with alias
func (m *Manager) ConnectSharedServiceToEnvironment(serviceName, project, envName string) error {
	containerName := fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)
	for _, networkName := range environmentNetworkCandidates(envName) {
		if err := m.provider.ConnectContainerToNetwork(m.ctx, containerName, networkName, serviceName); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to connect container %s to any environment network", containerName)
}

// DisconnectSharedServiceFromEnvironment removes network attachment
func (m *Manager) DisconnectSharedServiceFromEnvironment(serviceName, project, envName string) error {
	containerName := fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)
	var lastErr error
	for _, networkName := range environmentNetworkCandidates(envName) {
		if err := m.provider.DisconnectContainerFromNetwork(m.ctx, containerName, networkName); err != nil {
			if strings.Contains(err.Error(), "is not connected to") || strings.Contains(err.Error(), "No such network") {
				continue
			}
			lastErr = err
			continue
		}
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("failed to disconnect from environment networks: %w", lastErr)
	}

	return nil
}

// GetSharedServiceIP returns IP of shared container for a specific environment network
func (m *Manager) GetSharedServiceIP(serviceName, project, envName string) (string, error) {
	containerName := fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)
	for _, networkName := range environmentNetworkCandidates(envName) {
		ip, err := m.provider.GetContainerIPForNetwork(m.ctx, containerName, networkName)
		if err == nil {
			return ip, nil
		}
	}

	return "", fmt.Errorf("failed to get shared service IP on environment networks")
}

// RegisterSharedService adds or updates a shared service in state
func (m *Manager) RegisterSharedService(serviceName, project, containerName, ip string, composeFiles []string) error {
	serviceConfig, err := m.loadServiceConfig(serviceName, composeFiles)
	if err != nil {
		return fmt.Errorf("failed to load service config: %w", err)
	}

	configHash := computeConfigHash(serviceConfig)
	key := sharestore.Key(project, serviceName)

	return m.store.Update(m.ctx, key, func(existing *models.SharedService, exists bool) (*models.SharedService, bool, error) {
		if !exists || existing == nil {
			return &models.SharedService{
				Name:       serviceName,
				Container:  containerName,
				IP:         ip,
				Project:    project,
				Image:      serviceConfig.Image,
				ConfigHash: configHash,
				CreatedAt:  time.Now(),
				UsedBy:     []string{},
			}, true, nil
		}

		existing.Container = containerName
		existing.IP = ip
		existing.Project = project
		existing.Image = serviceConfig.Image
		existing.ConfigHash = configHash
		existing.DisconnectTimeout = time.Time{}

		return existing, true, nil
	})
}

func (m *Manager) AddEnvironmentReference(serviceName, project, envProject, envName string) error {
	key := sharestore.Key(project, serviceName)
	envKey := fmt.Sprintf("%s/%s", envProject, envName)

	return m.store.Update(m.ctx, key, func(existing *models.SharedService, exists bool) (*models.SharedService, bool, error) {
		if !exists || existing == nil {
			return nil, false, fmt.Errorf("shared service %s not found", key)
		}

		for _, used := range existing.UsedBy {
			if used == envKey {
				existing.DisconnectTimeout = time.Time{}
				return existing, true, nil
			}
		}

		existing.UsedBy = append(existing.UsedBy, envKey)
		existing.DisconnectTimeout = time.Time{}
		return existing, true, nil
	})
}

func (m *Manager) RemoveEnvironmentReference(serviceName, project, envProject, envName string) error {
	key := sharestore.Key(project, serviceName)
	envKey := fmt.Sprintf("%s/%s", envProject, envName)

	return m.store.Update(m.ctx, key, func(existing *models.SharedService, exists bool) (*models.SharedService, bool, error) {
		if !exists || existing == nil {
			return nil, false, nil
		}

		newUsedBy := make([]string, 0, len(existing.UsedBy))
		for _, used := range existing.UsedBy {
			if used != envKey {
				newUsedBy = append(newUsedBy, used)
			}
		}
		existing.UsedBy = newUsedBy

		if len(existing.UsedBy) == 0 {
			existing.DisconnectTimeout = time.Now().Add(60 * time.Second)
		}

		return existing, true, nil
	})
}

func (m *Manager) StopSharedServiceIfUnused(serviceName, project string) error {
	key := sharestore.Key(project, serviceName)
	sharedService, exists, err := m.store.Get(m.ctx, key)
	if err != nil {
		return err
	}
	if !exists || sharedService == nil {
		return nil
	}

	if len(sharedService.UsedBy) == 0 && !sharedService.DisconnectTimeout.IsZero() && time.Now().After(sharedService.DisconnectTimeout) {
		if err := m.provider.StopContainer(m.ctx, sharedService.Container); err != nil {
			fmt.Printf("Warning: failed to stop shared service container: %v\n", err)
		}
		if err := m.provider.RemoveContainer(m.ctx, sharedService.Container); err != nil {
			fmt.Printf("Warning: failed to remove shared service container: %v\n", err)
		}

		return m.store.Delete(m.ctx, key)
	}

	return nil
}

// computeConfigHash generates a hash of the service configuration for conflict detection
func computeConfigHash(service *models.ComposeService) string {
	// Include: image, volumes, ports, command, entrypoint
	// Exclude: environment variables (shared service runs with one config)
	data := map[string]interface{}{
		"image":   service.Image,
		"volumes": service.Volumes,
		"ports":   service.Ports,
		"command": service.Command,
	}

	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes
}

// getContainerPrimaryIP gets the first IP address of a container
func (m *Manager) getContainerPrimaryIP(containerName string) (string, error) {
	cmd := exec.CommandContext(m.ctx, "docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("container has no IP address")
	}

	return ip, nil
}

// startContainer starts a stopped container
func (m *Manager) startContainer(containerName string) error {
	cmd := exec.CommandContext(m.ctx, "docker", "start", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GetSharedServiceKey(project, serviceName string) string {
	return sharestore.Key(project, serviceName)
}

func environmentNetworkCandidates(envName string) []string {
	projectName := fmt.Sprintf("cilo_%s", envName)
	return []string{projectName, fmt.Sprintf("%s_default", projectName), fmt.Sprintf("%s_default", envName)}
}
