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

	"github.com/sharedco/cilo/pkg/models"
	"github.com/sharedco/cilo/pkg/runtime"
	"github.com/sharedco/cilo/pkg/state"
	"gopkg.in/yaml.v3"
)

// Manager handles shared service lifecycle
type Manager struct {
	provider runtime.Provider
	ctx      context.Context
}

// NewManager creates a new shared service manager
func NewManager(provider runtime.Provider, ctx context.Context) *Manager {
	return &Manager{
		provider: provider,
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
	networkName := fmt.Sprintf("cilo_%s", envName)

	// Connect with alias so containers in the environment can resolve by service name
	if err := m.provider.ConnectContainerToNetwork(m.ctx, containerName, networkName, serviceName); err != nil {
		return fmt.Errorf("failed to connect to network: %w", err)
	}

	return nil
}

// DisconnectSharedServiceFromEnvironment removes network attachment
func (m *Manager) DisconnectSharedServiceFromEnvironment(serviceName, project, envName string) error {
	containerName := fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)
	networkName := fmt.Sprintf("cilo_%s", envName)

	if err := m.provider.DisconnectContainerFromNetwork(m.ctx, containerName, networkName); err != nil {
		// Don't fail if already disconnected
		if strings.Contains(err.Error(), "is not connected to") {
			return nil
		}
		return fmt.Errorf("failed to disconnect from network: %w", err)
	}

	return nil
}

// GetSharedServiceIP returns IP of shared container for a specific environment network
func (m *Manager) GetSharedServiceIP(serviceName, project, envName string) (string, error) {
	containerName := fmt.Sprintf("cilo_shared_%s_%s", project, serviceName)
	networkName := fmt.Sprintf("cilo_%s", envName)

	ip, err := m.provider.GetContainerIPForNetwork(m.ctx, containerName, networkName)
	if err != nil {
		return "", fmt.Errorf("failed to get IP: %w", err)
	}

	return ip, nil
}

// RegisterSharedService adds or updates a shared service in state
func (m *Manager) RegisterSharedService(serviceName, project, containerName, ip string, composeFiles []string) error {
	// Load service config to compute hash
	serviceConfig, err := m.loadServiceConfig(serviceName, composeFiles)
	if err != nil {
		return fmt.Errorf("failed to load service config: %w", err)
	}

	configHash := computeConfigHash(serviceConfig)

	return state.WithLock(func(st *models.State) error {
		// Initialize SharedServices map if nil
		if st.SharedServices == nil {
			st.SharedServices = make(map[string]*models.SharedService)
		}

		key := fmt.Sprintf("%s/%s", project, serviceName)

		sharedService := st.SharedServices[key]
		if sharedService == nil {
			sharedService = &models.SharedService{
				Name:       serviceName,
				Container:  containerName,
				IP:         ip,
				Project:    project,
				Image:      serviceConfig.Image,
				ConfigHash: configHash,
				CreatedAt:  time.Now(),
				UsedBy:     []string{},
			}
			st.SharedServices[key] = sharedService
		} else {
			// Update IP in case it changed
			sharedService.IP = ip
			sharedService.DisconnectTimeout = time.Time{} // Clear any grace period
		}

		return nil
	})
}

// AddEnvironmentReference adds an environment to a shared service's UsedBy list
func (m *Manager) AddEnvironmentReference(serviceName, project, envProject, envName string) error {
	return state.WithLock(func(st *models.State) error {
		key := fmt.Sprintf("%s/%s", project, serviceName)
		envKey := fmt.Sprintf("%s/%s", envProject, envName)

		sharedService := st.SharedServices[key]
		if sharedService == nil {
			return fmt.Errorf("shared service %s not found", key)
		}

		// Add if not already present
		for _, used := range sharedService.UsedBy {
			if used == envKey {
				return nil // Already present
			}
		}

		sharedService.UsedBy = append(sharedService.UsedBy, envKey)
		sharedService.DisconnectTimeout = time.Time{} // Clear grace period if reconnecting

		return nil
	})
}

// RemoveEnvironmentReference removes an environment from a shared service's UsedBy list
func (m *Manager) RemoveEnvironmentReference(serviceName, project, envProject, envName string) error {
	return state.WithLock(func(st *models.State) error {
		key := fmt.Sprintf("%s/%s", project, serviceName)
		envKey := fmt.Sprintf("%s/%s", envProject, envName)

		sharedService := st.SharedServices[key]
		if sharedService == nil {
			return nil // Already removed
		}

		// Remove from UsedBy
		newUsedBy := []string{}
		for _, used := range sharedService.UsedBy {
			if used != envKey {
				newUsedBy = append(newUsedBy, used)
			}
		}
		sharedService.UsedBy = newUsedBy

		// If no longer used, set grace period
		if len(sharedService.UsedBy) == 0 {
			sharedService.DisconnectTimeout = time.Now().Add(60 * time.Second)
			// Note: Grace period cleanup handled by doctor or background process
		}

		return nil
	})
}

// StopSharedServiceIfUnused stops container if reference count is zero and grace period expired
func (m *Manager) StopSharedServiceIfUnused(serviceName, project string) error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%s", project, serviceName)
	sharedService := st.SharedServices[key]
	if sharedService == nil {
		return nil // Already removed
	}

	// Check if unused and grace period expired
	if len(sharedService.UsedBy) == 0 && !sharedService.DisconnectTimeout.IsZero() && time.Now().After(sharedService.DisconnectTimeout) {
		// Stop and remove the container
		if err := m.provider.StopContainer(m.ctx, sharedService.Container); err != nil {
			fmt.Printf("Warning: failed to stop shared service container: %v\n", err)
		}
		if err := m.provider.RemoveContainer(m.ctx, sharedService.Container); err != nil {
			fmt.Printf("Warning: failed to remove shared service container: %v\n", err)
		}

		// Remove from state
		return state.WithLock(func(st *models.State) error {
			delete(st.SharedServices, key)
			return nil
		})
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

// GetSharedServiceKey creates the state key for a shared service
func GetSharedServiceKey(project, serviceName string) string {
	return fmt.Sprintf("%s/%s", project, serviceName)
}
