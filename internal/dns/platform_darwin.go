// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build darwin

package dns

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getHomebrewPrefix() string {
	if cmd := exec.Command("brew", "--prefix"); cmd != nil {
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}

	if _, err := os.Stat("/opt/homebrew/bin/brew"); err == nil {
		return "/opt/homebrew"
	}
	if _, err := os.Stat("/usr/local/bin/brew"); err == nil {
		return "/usr/local"
	}

	return "/opt/homebrew"
}

func getConfDir() string {
	prefix := getHomebrewPrefix()
	return filepath.Join(prefix, "etc", "dnsmasq.d")
}

func getMainConfPath() string {
	prefix := getHomebrewPrefix()
	return filepath.Join(prefix, "etc", "dnsmasq.conf")
}

func isDnsmasqManaged() bool {
	cmd := exec.Command("brew", "list", "dnsmasq")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func ensureDnsmasqBaseConfig() error {
	mainConf := getMainConfPath()

	var existing string
	if data, err := os.ReadFile(mainConf); err == nil {
		existing = string(data)
	}

	needsPort := !strings.Contains(existing, "port=5354")
	needsBind := !strings.Contains(existing, "bind-interfaces")
	needsListen := !strings.Contains(existing, "listen-address=127.0.0.1")
	needsConfDir := !strings.Contains(existing, "conf-dir=")

	if !needsPort && !needsBind && !needsListen && !needsConfDir {
		return nil
	}

	var additions strings.Builder
	if needsPort {
		additions.WriteString("port=5354\n")
	}
	if needsBind {
		additions.WriteString("bind-interfaces\n")
	}
	if needsListen {
		additions.WriteString("listen-address=127.0.0.1\n")
	}
	if needsConfDir {
		confDir := getConfDir()
		additions.WriteString(fmt.Sprintf("conf-dir=%s/\n", confDir))
	}

	confDir := getConfDir()
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("failed to create dnsmasq.d directory: %w", err)
	}

	newConfig := existing + "\n# cilo DNS settings\n" + additions.String()
	if err := os.WriteFile(mainConf, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	return nil
}

func reloadDNSService() error {
	cmd := exec.Command("brew", "services", "restart", "dnsmasq")
	if err := cmd.Run(); err != nil {
		exec.Command("killall", "dnsmasq").Run()
		cmd = exec.Command("brew", "services", "start", "dnsmasq")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restart dnsmasq via brew services: %w", err)
		}
	}
	return nil
}

var getCiloConfPath = func() string {
	return filepath.Join(getConfDir(), "cilo.conf")
}
