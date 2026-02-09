// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Auth contains cloud authentication credentials
type Auth struct {
	Server string `json:"server"`
	APIKey string `json:"api_key"`
	TeamID string `json:"team_id,omitempty"`
}

// authFilePath returns the path to the auth file
func authFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".cilo", "cloud-auth.json"), nil
}

// SaveAuth saves authentication credentials
func SaveAuth(auth *Auth) error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write auth file: %w", err)
	}

	return nil
}

// LoadAuth loads authentication credentials
func LoadAuth() (*Auth, error) {
	path, err := authFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in")
		}
		return nil, fmt.Errorf("read auth file: %w", err)
	}

	var auth Auth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("parse auth file: %w", err)
	}

	if auth.Server == "" || auth.APIKey == "" {
		return nil, fmt.Errorf("invalid auth file: missing server or api_key")
	}

	return &auth, nil
}

// ClearAuth removes the authentication file
func ClearAuth() error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove auth file: %w", err)
	}

	return nil
}

// IsLoggedIn checks if the user is logged in
func IsLoggedIn() bool {
	auth, err := LoadAuth()
	return err == nil && auth != nil
}

// GetServerURL returns the configured server URL
func GetServerURL() (string, error) {
	auth, err := LoadAuth()
	if err != nil {
		return "", err
	}
	return auth.Server, nil
}
