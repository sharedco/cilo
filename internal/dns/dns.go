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
	"runtime"
	"strings"

	"github.com/sharedco/cilo/internal/config"
	"github.com/sharedco/cilo/internal/models"
)

const (
	defaultDNSPort = 5354
	dnsConfFile    = "dnsmasq.conf"
	resolverDir    = "/etc/resolver"
)

func SetupDNS(state *models.State) error {
	if runtime.GOOS == "darwin" {
		if err := ensureDnsmasqBaseConfig(); err != nil {
			return err
		}

		entries, err := RenderEntries(state)
		if err != nil {
			return fmt.Errorf("failed to render DNS entries: %w", err)
		}

		ciloConf := getCiloConfPath()
		if err := os.WriteFile(ciloConf, []byte(entries), 0644); err != nil {
			return fmt.Errorf("failed to write cilo DNS config: %w", err)
		}

		return reloadDNSService()
	}

	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/etc/systemd"); err == nil {
			return setupSystemdResolved(state)
		}

		if isDnsmasqManaged() {
			entries, err := RenderEntries(state)
			if err != nil {
				return fmt.Errorf("failed to render DNS entries: %w", err)
			}

			ciloConf := getCiloConfPath()
			if err := os.WriteFile(ciloConf, []byte(entries), 0644); err != nil {
				return fmt.Errorf("failed to write cilo DNS config: %w", err)
			}

			return reloadDNSService()
		}

		return fallbackSetupDNS(state)
	}

	return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
}

func fallbackSetupDNS(state *models.State) error {
	dnsDir := getDNSDir()

	config, err := RenderConfig(state)
	if err != nil {
		return fmt.Errorf("failed to render base DNS config: %w", err)
	}

	configPath := filepath.Join(dnsDir, dnsConfFile)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	return fallbackStartDNS()
}

func SetupSystemResolver(state *models.State) error {
	return setupResolver(state)
}

func UpdateDNSFromState(state *models.State) error {
	if runtime.GOOS == "darwin" {
		entries, err := RenderEntries(state)
		if err != nil {
			return fmt.Errorf("failed to render DNS entries: %w", err)
		}

		ciloConf := getCiloConfPath()
		tmpPath := ciloConf + ".tmp"

		if err := os.WriteFile(tmpPath, []byte(entries), 0644); err != nil {
			return fmt.Errorf("failed to write temp DNS config: %w", err)
		}

		if err := os.Rename(tmpPath, ciloConf); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to rename DNS config: %w", err)
		}

		return reloadDNSService()
	}

	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/etc/systemd"); err == nil {
			return setupSystemdResolved(state)
		}

		if isDnsmasqManaged() {
			entries, err := RenderEntries(state)
			if err != nil {
				return fmt.Errorf("failed to render DNS entries: %w", err)
			}

			ciloConf := getCiloConfPath()
			tmpPath := ciloConf + ".tmp"

			if err := os.WriteFile(tmpPath, []byte(entries), 0644); err != nil {
				return fmt.Errorf("failed to write temp DNS config: %w", err)
			}

			if err := os.Rename(tmpPath, ciloConf); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("failed to rename DNS config: %w", err)
			}

			return reloadDNSService()
		}

		return fallbackUpdateDNSFromState(state)
	}

	return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
}

func fallbackUpdateDNSFromState(state *models.State) error {
	config, err := RenderConfig(state)
	if err != nil {
		return fmt.Errorf("failed to render DNS config: %w", err)
	}

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

	return fallbackReloadDNS()
}

func UpdateDNS(env *models.Environment) error {
	state, err := loadStateForDNS()
	if err != nil {
		return err
	}
	return UpdateDNSFromState(state)
}

func loadStateForDNS() (*models.State, error) {
	statePath := config.GetStatePath()

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
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

func RemoveDNS(envName string) error {
	if runtime.GOOS == "darwin" || (runtime.GOOS == "linux" && isDnsmasqManaged()) {
		ciloConf := getCiloConfPath()

		data, err := os.ReadFile(ciloConf)
		if err != nil {
			return nil
		}

		config := string(data)

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

		if err := os.WriteFile(ciloConf, []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to write dnsmasq config: %w", err)
		}

		return reloadDNSService()
	}

	return fallbackRemoveDNS(envName)
}

func fallbackRemoveDNS(envName string) error {
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	config := string(data)

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

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	return fallbackReloadDNS()
}

func reloadDNS() error {
	if runtime.GOOS == "darwin" || (runtime.GOOS == "linux" && isDnsmasqManaged()) {
		return reloadDNSService()
	}
	return fallbackReloadDNS()
}

func fallbackStartDNS() error {
	if _, err := exec.LookPath("dnsmasq"); err != nil {
		return fmt.Errorf("dnsmasq is not installed. Please install it:\n\n  macOS: brew install dnsmasq\n  Ubuntu/Debian: sudo apt install dnsmasq\n  Fedora: sudo dnf install dnsmasq\n  Arch: sudo pacman -S dnsmasq")
	}

	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	cmd := exec.Command("dnsmasq", "--conf-file="+configPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %w", err)
	}

	return nil
}

func fallbackReloadDNS() error {
	if _, err := exec.LookPath("dnsmasq"); err != nil {
		return nil
	}

	exec.Command("killall", "dnsmasq").Run()

	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	cmd := exec.Command("dnsmasq", "--conf-file="+configPath)
	cmd.Start()

	return nil
}

func setupResolver(state *models.State) error {
	if _, err := os.Stat("/etc/systemd"); err == nil {
		return setupSystemdResolved(state)
	}

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
	if _, err := os.Stat(resolverDir); os.IsNotExist(err) {
		if err := os.MkdirAll(resolverDir, 0755); err != nil {
			return fmt.Errorf("failed to create resolver directory: %w", err)
		}
	}

	resolverFile := filepath.Join(resolverDir, "test")
	port := defaultDNSPort
	if state != nil && state.DNSPort != 0 {
		port = state.DNSPort
	}
	content := fmt.Sprintf("nameserver 127.0.0.1\nport %d\n", port)

	existing, _ := os.ReadFile(resolverFile)
	if string(existing) == content {
		return nil
	}

	if err := os.WriteFile(resolverFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write resolver file: %w", err)
	}

	return nil
}

var getDNSDir = func() string {
	return config.GetDNSDir()
}

func GetDNSPort(state *models.State) int {
	if state != nil && state.DNSPort != 0 {
		return state.DNSPort
	}
	return defaultDNSPort
}

func Cleanup() error {
	if runtime.GOOS == "darwin" {
		ciloConf := getCiloConfPath()
		os.Remove(ciloConf)
		return reloadDNSService()
	}

	if runtime.GOOS == "linux" {
		if isDnsmasqManaged() {
			ciloConf := getCiloConfPath()
			os.Remove(ciloConf)
			return reloadDNSService()
		}
	}

	return nil
}
