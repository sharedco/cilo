// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package dns

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sharedco/cilo/internal/config"
	"github.com/sharedco/cilo/internal/models"
)

const (
	defaultDNSPort = 5354
	dnsConfFile    = "dnsmasq.conf"
	dnsPidFile     = "dnsmasq.pid"
	resolverDir    = "/etc/resolver"
)

func SetupDNS(state *models.State) error {
	dnsDir := getDNSDir()

	// Render base config with current state
	config, err := RenderConfig(state)
	if err != nil {
		return fmt.Errorf("failed to render base DNS config: %w", err)
	}

	configPath := filepath.Join(dnsDir, dnsConfFile)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	return startDNS()
}

func SetupSystemResolver(state *models.State) error {
	return setupResolver(state)
}

// UpdateDNSFromState regenerates DNS config from state and reloads dnsmasq
func UpdateDNSFromState(state *models.State) error {
	config, err := RenderConfig(state)
	if err != nil {
		return fmt.Errorf("failed to render DNS config: %w", err)
	}

	// Atomic write
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)
	tmpPath := configPath + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write temp DNS config: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename DNS config: %w", err)
	}

	// Graceful reload with SIGHUP
	return reloadDNSGraceful()
}

// UpdateDNS updates DNS entries for an environment (deprecated - use UpdateDNSFromState)
func UpdateDNS(env *models.Environment) error {
	// For backward compat, we need to load full state and render
	// This is less efficient but maintains API compatibility
	state, err := loadStateForDNS()
	if err != nil {
		return err
	}
	return UpdateDNSFromState(state)
}

func loadStateForDNS() (*models.State, error) {
	// Load state from state.json
	statePath := config.GetStatePath()

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty state if file doesn't exist
			return &models.State{
				Version: 2,
				Hosts:   make(map[string]*models.Host),
			}, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state models.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

// RemoveDNS removes DNS entries for an environment
func RemoveDNS(envName string) error {
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil // Config doesn't exist, nothing to do
	}

	config := string(data)

	// Remove entries for this environment
	startMarker := fmt.Sprintf("\n# Environment: %s\n", envName)
	endMarker := fmt.Sprintf("\n# End environment: %s\n", envName)

	for {
		start := strings.Index(config, startMarker)
		if start == -1 {
			break
		}
		end := strings.Index(config[start:], endMarker)
		if end == -1 {
			config = config[:start]
			break
		}
		config = config[:start] + config[start+end+len(endMarker):]
	}

	// Write updated config
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	// Reload dnsmasq
	if err := reloadDNSRestart(); err != nil {
		return err
	}

	return nil
}

func startDNS() error {
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)
	pidPath := filepath.Join(dnsDir, dnsPidFile)

	// Check if dnsmasq is already running
	if data, err := os.ReadFile(pidPath); err == nil {
		pid := strings.TrimSpace(string(data))
		if _, err := os.Stat(fmt.Sprintf("/proc/%s", pid)); err == nil {
			// Already running, reload instead
			return reloadDNSRestart()
		}
	}

	// Start dnsmasq
	cmd := exec.Command("dnsmasq", "--conf-file="+configPath, fmt.Sprintf("--pid-file=%s", pidPath))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %w", err)
	}

	return nil
}

func reloadDNSGraceful() error {
	dnsDir := getDNSDir()
	pidPath := filepath.Join(dnsDir, dnsPidFile)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		// Not running, start it
		return startDNS()
	}

	pid := strings.TrimSpace(string(data))
	pidInt := 0
	fmt.Sscanf(pid, "%d", &pidInt)

	if pidInt <= 0 {
		return startDNS()
	}

	// Check if process exists
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", pidInt)); err != nil {
		return startDNS()
	}

	// Send SIGHUP for graceful reload (dnsmasq re-reads config on SIGHUP)
	if err := syscall.Kill(pidInt, syscall.SIGHUP); err != nil {
		// Process gone, restart
		return startDNS()
	}

	return nil
}

// reloadDNSRestart stops and restarts dnsmasq (use reloadDNSGraceful for config updates)
func reloadDNSRestart() error {
	dnsDir := getDNSDir()
	pidPath := filepath.Join(dnsDir, dnsPidFile)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return startDNS()
	}

	pid := strings.TrimSpace(string(data))
	pidInt := 0
	fmt.Sscanf(pid, "%d", &pidInt)

	if pidInt > 0 {
		syscall.Kill(pidInt, syscall.SIGTERM)
		exec.Command("sleep", "0.5").Run()
	}

	return startDNS()
}

func setupResolver(state *models.State) error {
	// Check OS
	if _, err := os.Stat("/etc/systemd"); err == nil {
		// Linux with systemd
		return setupSystemdResolved(state)
	}

	// macOS or other Unix
	return setupMacOSResolver(state)
}

func setupSystemdResolved(state *models.State) error {
	confDir := "/etc/systemd/resolved.conf.d"
	confFile := filepath.Join(confDir, "cilo.conf")

	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("failed to create resolved.conf.d: %w", err)
	}

	port := defaultDNSPort
	if state != nil && state.DNSPort != 0 {
		port = state.DNSPort
	}

	config := fmt.Sprintf(`[Resolve]
DNS=127.0.0.1:%d
Domains=~test
`, port)

	if err := os.WriteFile(confFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write resolved config: %w", err)
	}

	cmd := exec.Command("systemctl", "restart", "systemd-resolved")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	return nil
}

func setupMacOSResolver(state *models.State) error {
	// macOS uses /etc/resolver/
	if _, err := os.Stat(resolverDir); os.IsNotExist(err) {
		fmt.Printf("Please create %s directory with:\n", resolverDir)
		fmt.Printf("  sudo mkdir -p %s\n", resolverDir)
		fmt.Println("Then add the test resolver:")
		port := defaultDNSPort
		if state != nil && state.DNSPort != 0 {
			port = state.DNSPort
		}
		fmt.Printf("  echo 'nameserver 127.0.0.1\nport %d' | sudo tee %s/test\n", port, resolverDir)
		return fmt.Errorf("resolver directory not found")
	}

	resolverFile := filepath.Join(resolverDir, "test")
	port := defaultDNSPort
	if state != nil && state.DNSPort != 0 {
		port = state.DNSPort
	}
	content := fmt.Sprintf("nameserver 127.0.0.1\nport %d\n", port)

	existing, _ := os.ReadFile(resolverFile)
	if string(existing) == content {
		return nil // Already configured
	}

	fmt.Println("Please run the following command to configure DNS:")
	fmt.Printf("  echo 'nameserver 127.0.0.1\nport %d' | sudo tee %s/test\n", port, resolverDir)

	return nil
}

func getDNSDir() string {
	return config.GetDNSDir()
}

// GetDNSPort returns the configured DNS port
func GetDNSPort(state *models.State) int {
	if state != nil && state.DNSPort != 0 {
		return state.DNSPort
	}
	return defaultDNSPort
}

// Cleanup stops dnsmasq and removes DNS configuration
func Cleanup() error {
	dnsDir := getDNSDir()
	pidPath := filepath.Join(dnsDir, dnsPidFile)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return nil // PID file doesn't exist
	}

	pid := strings.TrimSpace(string(data))
	pidInt := 0
	fmt.Sscanf(pid, "%d", &pidInt)

	if pidInt > 0 {
		// Send SIGTERM to stop
		syscall.Kill(pidInt, syscall.SIGTERM)
	}

	os.Remove(pidPath)

	return nil
}
