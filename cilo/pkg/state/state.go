package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/models"
	"github.com/cilo/cilo/pkg/network"
)

const baseSubnet = "10.224."

func getStatePath() string {
	return config.GetStatePath()
}

func GetEnvStoragePath(project, name string) string {
	return config.GetEnvPath(project, name)
}

// makeEnvKey creates a unique key for an environment
// Format: "project/name"
func makeEnvKey(project, name string) string {
	return fmt.Sprintf("%s/%s", project, name)
}

// parseEnvKey splits an environment key into project and name
func parseEnvKey(key string) (project, name string, err error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid environment key: %s", key)
	}
	return parts[0], parts[1], nil
}

// InitializeState creates the initial state file if it doesn't exist
func InitializeState() error {
	path := getStatePath()

	if _, err := os.Stat(path); err == nil {
		_, err := LoadState()
		return err
	}

	state := &models.State{
		Version:       1,
		SubnetCounter: 0,
		Environments:  make(map[string]*models.Environment),
	}
	return SaveState(state)
}

// LoadState loads the state from disk
func LoadState() (*models.State, error) {
	path := getStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cilo not initialized (run 'cilo init')")
		}
		return nil, err
	}

	var state models.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	// Initialize map if nil (for backwards compatibility)
	if state.Environments == nil {
		state.Environments = make(map[string]*models.Environment)
	}

	return &state, nil
}

// SaveState saves the state to disk
func SaveState(state *models.State) error {
	path := getStatePath()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

// GetEnvironment retrieves an environment by project and name
func GetEnvironment(project, name string) (*models.Environment, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	key := makeEnvKey(project, name)
	env, exists := state.Environments[key]
	if !exists {
		return nil, fmt.Errorf("environment %q does not exist in project %q", name, project)
	}

	return env, nil
}

// GetEnvironmentByKey retrieves an environment by its full key (project/name)
func GetEnvironmentByKey(key string) (*models.Environment, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	env, exists := state.Environments[key]
	if !exists {
		return nil, fmt.Errorf("environment %q does not exist", key)
	}

	return env, nil
}

// EnvironmentExists checks if an environment exists in a project
func EnvironmentExists(project, name string) (bool, error) {
	state, err := LoadState()
	if err != nil {
		return false, err
	}

	key := makeEnvKey(project, name)
	_, exists := state.Environments[key]
	return exists, nil
}

// CreateEnvironment creates a new environment and allocates resources
func CreateEnvironment(name string, source string, project string) (*models.Environment, error) {
	var env *models.Environment

	err := WithLock(func(state *models.State) error {
		key := makeEnvKey(project, name)
		if _, exists := state.Environments[key]; exists {
			return fmt.Errorf("environment %q already exists in project %q", name, project)
		}

		if err := validateName(name); err != nil {
			return err
		}

		state.SubnetCounter++
		subnet := fmt.Sprintf("%s%d.0/24", baseSubnet, state.SubnetCounter)

		// Check for collisions with existing Docker networks
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		collision, collidingNet, err := network.CheckSubnetCollision(ctx, subnet)
		if err != nil {
			// Log warning but continue - don't fail on detection errors
			fmt.Printf("Warning: could not check subnet collision: %v\n", err)
		} else if collision {
			// Try next subnet
			fmt.Printf("Subnet %s conflicts with Docker network %s, trying next...\n", subnet, collidingNet)
			state.SubnetCounter++
			subnet = fmt.Sprintf("%s%d.0/24", baseSubnet, state.SubnetCounter)

			// Check once more
			collision2, collidingNet2, _ := network.CheckSubnetCollision(ctx, subnet)
			if collision2 {
				return fmt.Errorf("failed to allocate non-conflicting subnet (conflicts with %s)", collidingNet2)
			}
		}

		env = &models.Environment{
			Name:      name,
			Project:   project,
			CreatedAt: time.Now(),
			Subnet:    subnet,
			Status:    "created",
			Source:    source,
			Services:  make(map[string]*models.Service),
		}

		state.Environments[key] = env
		return nil
	})

	return env, err
}

// UpdateEnvironment updates an environment in state
func UpdateEnvironment(env *models.Environment) error {
	return WithLock(func(state *models.State) error {
		key := makeEnvKey(env.Project, env.Name)
		state.Environments[key] = env
		return nil
	})
}

// DeleteEnvironment removes an environment from state
func DeleteEnvironment(project, name string) error {
	return WithLock(func(state *models.State) error {
		key := makeEnvKey(project, name)
		delete(state.Environments, key)
		return nil
	})
}

// DeleteEnvironmentByKey removes an environment by its full key
func DeleteEnvironmentByKey(key string) error {
	return WithLock(func(state *models.State) error {
		delete(state.Environments, key)
		return nil
	})
}

// ListEnvironments returns all environments
func ListEnvironments() ([]*models.Environment, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	envs := make([]*models.Environment, 0, len(state.Environments))
	for _, env := range state.Environments {
		envs = append(envs, env)
	}

	return envs, nil
}

// ListEnvironmentsByProject returns all environments for a specific project
func ListEnvironmentsByProject(project string) ([]*models.Environment, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	envs := make([]*models.Environment, 0)
	for key, env := range state.Environments {
		// Parse the key to check project
		if strings.HasPrefix(key, project+"/") {
			envs = append(envs, env)
		}
	}

	return envs, nil
}

func NormalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.Trim(name, "-")
	return name
}

func validateName(name string) error {
	if len(name) == 0 || len(name) > 63 {
		return fmt.Errorf("environment name must be between 1 and 63 characters")
	}

	for i, c := range name {
		if i == 0 || i == len(name)-1 {
			if c == '-' {
				return fmt.Errorf("environment name cannot start or end with '-'")
			}
		}

		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return fmt.Errorf("environment name can only contain lowercase letters, numbers, and hyphens")
	}

	return nil
}
