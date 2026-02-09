// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the Cilo server
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Pool     PoolConfig
	Provider ProviderConfig
	Features FeaturesConfig
	Cleanup  CleanupConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	ListenAddr   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	URL string
}

// PoolConfig holds VM pool configuration
type PoolConfig struct {
	MinReady int
	MaxTotal int
	VMSize   string
	Region   string
	ImageID  string
}

// ProviderConfig holds cloud provider configuration
type ProviderConfig struct {
	Type         string // "hetzner", "digitalocean", etc.
	HetznerToken string
}

// FeaturesConfig holds feature flags
type FeaturesConfig struct {
	BillingEnabled bool
	MetricsEnabled bool
}

// CleanupConfig holds cleanup job configuration
type CleanupConfig struct {
	AutoDestroyHours int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			ListenAddr:   getEnv("LISTEN_ADDR", ":8080"),
			ReadTimeout:  getDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDuration("IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		Pool: PoolConfig{
			MinReady: getInt("POOL_MIN_READY", 2),
			MaxTotal: getInt("POOL_MAX_TOTAL", 10),
			VMSize:   getEnv("POOL_VM_SIZE", "cx11"),
			Region:   getEnv("POOL_REGION", "nbg1"),
			ImageID:  getEnv("POOL_IMAGE_ID", ""),
		},
		Provider: ProviderConfig{
			Type:         getEnv("PROVIDER_TYPE", "hetzner"),
			HetznerToken: getEnv("HETZNER_TOKEN", ""),
		},
		Features: FeaturesConfig{
			BillingEnabled: getBool("BILLING_ENABLED", false),
			MetricsEnabled: getBool("METRICS_ENABLED", true),
		},
		Cleanup: CleanupConfig{
			AutoDestroyHours: getInt("AUTO_DESTROY_HOURS", 24),
		},
	}

	// Validate required fields
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
