// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sharedco/cilo/internal/cilod"
)

// RemoteMachine represents a connected remote machine for DNS purposes
// This is a minimal struct to avoid import cycles with internal/cli
type RemoteMachine struct {
	Host         string
	WGAssignedIP string
}

const (
	remoteMachineStartMarker = "# Remote machine: %s\n"
	remoteMachineEndMarker   = "# End remote machine: %s\n"
)

// AddRemoteMachine adds DNS entries for all services on a remote machine
// DNS entries point to the WireGuard peer IP (cilod's proxy IP)
// Format: {service}.{env}.test → {wg-proxy-ip}
func AddRemoteMachine(machine *RemoteMachine, envs []cilod.Environment) error {
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	// Read existing config or create new one
	var config string
	data, err := os.ReadFile(configPath)
	if err == nil {
		config = string(data)
	}

	// Remove any existing entries for this machine (idempotent)
	config = removeRemoteMachineEntries(config, machine.Host)

	// Generate new entries
	var entries strings.Builder
	entries.WriteString(fmt.Sprintf(remoteMachineStartMarker, machine.Host))

	for _, env := range envs {
		for _, svc := range env.Services {
			hostname := fmt.Sprintf("%s.%s.test", svc, env.Name)
			entries.WriteString(fmt.Sprintf("address=/%s/%s\n", hostname, machine.WGAssignedIP))
		}
	}

	entries.WriteString(fmt.Sprintf(remoteMachineEndMarker, machine.Host))

	// Append to config
	config += entries.String()

	// Write config
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write DNS config: %w", err)
	}

	// Reload dnsmasq
	return reloadDNSGraceful()
}

// RemoveRemoteMachine removes all DNS entries for a remote machine
func RemoveRemoteMachine(host string) error {
	dnsDir := getDNSDir()
	configPath := filepath.Join(dnsDir, dnsConfFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read DNS config: %w", err)
	}

	config := string(data)
	config = removeRemoteMachineEntries(config, host)

	// Write updated config
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write DNS config: %w", err)
	}

	// Reload dnsmasq
	return reloadDNSGraceful()
}

// UpdateRemoteDNSEntries updates DNS entries when remote environments change
func UpdateRemoteDNSEntries(machine *RemoteMachine, envs []cilod.Environment) error {
	// Simply remove and re-add all entries
	if err := RemoveRemoteMachine(machine.Host); err != nil {
		return err
	}
	return AddRemoteMachine(machine, envs)
}

// removeRemoteMachineEntries removes all entries for a specific remote machine from config
func removeRemoteMachineEntries(config string, host string) string {
	startMarker := fmt.Sprintf(remoteMachineStartMarker, host)
	endMarker := fmt.Sprintf(remoteMachineEndMarker, host)

	for {
		start := strings.Index(config, startMarker)
		if start == -1 {
			break
		}

		end := strings.Index(config[start:], endMarker)
		if end == -1 {
			// Malformed - remove from start to end of string
			config = config[:start]
			break
		}

		// Remove the section including markers
		config = config[:start] + config[start+end+len(endMarker):]
	}

	return config
}
