// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sharedco/cilo/internal/models"
	"gopkg.in/yaml.v3"
)

// CiloConfig represents a .cilo/config.yml file
type CiloConfig struct {
	Project   string            `yaml:"project,omitempty"`
	Ingress   string            `yaml:"ingress,omitempty"`
	Hostnames []string          `yaml:"hostnames,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// LoadCiloConfig loads a .cilo/config.yml file if it exists
func LoadCiloConfig(projectPath string) (*CiloConfig, error) {
	configPath := filepath.Join(projectPath, ".cilo", "config.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config CiloConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveCiloConfig saves a .cilo/config.yml file
func SaveCiloConfig(workspacePath string, config *CiloConfig) error {
	ciloDir := filepath.Join(workspacePath, ".cilo")
	if err := os.MkdirAll(ciloDir, 0755); err != nil {
		return fmt.Errorf("failed to create .cilo directory: %w", err)
	}

	configPath := filepath.Join(ciloDir, "config.yml")

	output, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	header := "# Cilo configuration file\n# This file is managed by cilo. Edit with care.\n\n"

	if err := os.WriteFile(configPath, []byte(header+string(output)), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ApplyCiloConfig applies config settings to an environment
func ApplyCiloConfig(env *models.Environment, config *CiloConfig) {
	if config == nil {
		return
	}

	if config.Project != "" && env.Project == "" {
		env.Project = config.Project
	}

	if config.Ingress != "" {
		for _, svc := range env.Services {
			if svc.Name == config.Ingress {
				svc.IsIngress = true
			}
		}
	}

	if len(config.Hostnames) > 0 {
		for _, svc := range env.Services {
			if svc.IsIngress {
				svc.Hostnames = append(svc.Hostnames, config.Hostnames...)
			}
		}
	}
}

// CreateDefaultCiloConfig creates a default config file for an environment
func CreateDefaultCiloConfig(env *models.Environment, workspacePath string) error {
	config := &CiloConfig{
		Project: env.Project,
	}

	for _, svc := range env.Services {
		if svc.IsIngress {
			config.Ingress = svc.Name
			config.Hostnames = svc.Hostnames
			break
		}
	}

	return SaveCiloConfig(workspacePath, config)
}
