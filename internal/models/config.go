// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package models

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadProjectConfig loads the .cilo/config.yml from the current directory
// Returns nil, nil if no config exists
func LoadProjectConfig() (*ProjectConfig, error) {
	return LoadProjectConfigFromPath(".")
}

// LoadProjectConfigFromPath loads .cilo/config.yml from a specific directory
// Returns nil, nil if no config exists
func LoadProjectConfigFromPath(path string) (*ProjectConfig, error) {
	configPath := filepath.Join(path, ".cilo", "config.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var config ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ProjectConfigured returns true if the current directory has a .cilo/config.yml
func ProjectConfigured() bool {
	_, err := os.Stat(filepath.Join(".cilo", "config.yml"))
	return err == nil
}
