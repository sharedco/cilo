// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the agent configuration loaded from environment variables.
type Config struct {
	// Server settings
	ListenAddr   string // e.g., "10.225.0.100:8080" (WireGuard interface only)
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Workspace settings
	WorkspaceDir string // Where environment workspaces are stored

	// WireGuard settings
	WGInterface  string // WireGuard interface name
	WGListenPort int    // WireGuard listen port
	WGPrivateKey string // WireGuard private key
	WGAddress    string // WireGuard interface address (CIDR)

	// Server connection (for reporting)
	ServerURL string
	MachineID string
}

// Load creates a Config from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ListenAddr:   getEnv("CILO_AGENT_LISTEN", "0.0.0.0:8080"),
		ReadTimeout:  getDuration("CILO_AGENT_READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getDuration("CILO_AGENT_WRITE_TIMEOUT", 30*time.Second),
		WorkspaceDir: getEnv("CILO_WORKSPACE_DIR", "/var/cilo/envs"),
		WGInterface:  getEnv("CILO_WG_INTERFACE", "wg0"),
		WGListenPort: getInt("CILO_WG_PORT", 51820),
		WGPrivateKey: getEnv("CILO_WG_PRIVATE_KEY", ""),
		WGAddress:    getEnv("CILO_WG_ADDRESS", "10.225.0.100/16"),
		ServerURL:    getEnv("CILO_SERVER_URL", ""),
		MachineID:    getEnv("CILO_MACHINE_ID", ""),
	}
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getInt retrieves an integer environment variable or returns a default value.
func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getDuration retrieves a duration environment variable or returns a default value.
// The environment variable should be in a format parseable by time.ParseDuration.
func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getBool retrieves a boolean environment variable or returns a default value.
func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
