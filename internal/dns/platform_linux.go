// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build linux

package dns

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	networkManagerDnsmasqDir = "/etc/NetworkManager/dnsmasq.d"
)

func getConfDir() string {
	if isNetworkManagerWithDnsmasq() {
		return networkManagerDnsmasqDir
	}
	return ""
}

func isDnsmasqManaged() bool {
	return isNetworkManagerWithDnsmasq()
}

func ensureDnsmasqBaseConfig() error {
	// On Linux, dnsmasq base config is typically managed by the system
	// (NetworkManager or systemd-resolved). We don't need to modify
	// the main config file like we do on macOS with Homebrew.
	// This stub satisfies the cross-platform interface.
	return nil
}

func isNetworkManagerWithDnsmasq() bool {
	_, err := os.Stat("/etc/NetworkManager/NetworkManager.conf")
	if err != nil {
		return false
	}

	data, err := os.ReadFile("/etc/NetworkManager/NetworkManager.conf")
	if err != nil {
		return false
	}

	content := string(data)
	return contains(content, "dns=dnsmasq") || contains(content, "dnsmasq")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func reloadDNSService() error {
	if isNetworkManagerWithDnsmasq() {
		cmd := exec.Command("systemctl", "restart", "NetworkManager")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restart NetworkManager: %w", err)
		}
		return nil
	}

	if _, err := os.Stat("/etc/systemd"); err == nil {
		cmd := exec.Command("systemctl", "restart", "systemd-resolved")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restart systemd-resolved: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no supported DNS service found")
}

var getCiloConfPath = func() string {
	if dir := getConfDir(); dir != "" {
		return dir + "/cilo.conf"
	}
	return ""
}
