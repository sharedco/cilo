// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/sharedco/cilo/internal/config"
	"github.com/spf13/cobra"
)

// Target determines where a command should execute - locally or on a remote machine
type Target interface {
	IsRemote() bool
	GetMachine() string
	GetClient() *cilod.Client
}

// LocalTarget represents execution on the local machine
type LocalTarget struct{}

func (l LocalTarget) IsRemote() bool           { return false }
func (l LocalTarget) GetMachine() string       { return "" }
func (l LocalTarget) GetClient() *cilod.Client { return nil }

// RemoteTarget represents execution on a remote machine via cilod
type RemoteTarget struct {
	Machine string
	Client  *cilod.Client
}

func (r RemoteTarget) IsRemote() bool           { return true }
func (r RemoteTarget) GetMachine() string       { return r.Machine }
func (r RemoteTarget) GetClient() *cilod.Client { return r.Client }

// Machine represents a connected remote machine
// Machine state is persisted in ~/.cilo/machines/<host>/state.json
// WireGuard keys are stored in ~/.cilo/machines/<host>/wg-key
type Machine struct {
	Host              string    `json:"host"`
	Token             string    `json:"token"`
	WGPrivateKey      string    `json:"wg_private_key"`
	WGPublicKey       string    `json:"wg_public_key"`
	WGServerPublicKey string    `json:"wg_server_public_key"`
	WGAssignedIP      string    `json:"wg_assigned_ip"`
	WGEndpoint        string    `json:"wg_endpoint"`
	WGAllowedIPs      []string  `json:"wg_allowed_ips,omitempty"`
	WGInterface       string    `json:"wg_interface,omitempty"`
	EnvironmentSubnet string    `json:"environment_subnet,omitempty"`
	ConnectedAt       time.Time `json:"connected_at"`
	Status            string    `json:"status,omitempty"`
	Version           int       `json:"version"`
}

// resolveTarget determines whether to execute locally or remotely based on --on flag
// Returns LocalTarget{} if --on is not specified
// Returns RemoteTarget{machine, client} if --on is specified and machine is connected
// Returns error if --on is specified but machine is not connected
func resolveTarget(cmd *cobra.Command) (Target, error) {
	onFlag, err := cmd.Flags().GetString("on")
	if err != nil {
		return nil, fmt.Errorf("failed to get --on flag: %w", err)
	}

	if onFlag == "" {
		return LocalTarget{}, nil
	}

	machine, err := GetMachine(onFlag)
	if err != nil {
		return nil, fmt.Errorf("failed to check machine '%s': %w", onFlag, err)
	}
	if machine == nil {
		return nil, fmt.Errorf("machine '%s' is not connected. Run 'cilo connect %s' first", onFlag, onFlag)
	}

	client := cilod.NewClient(machine.WGAssignedIP, machine.Token)

	return RemoteTarget{
		Machine: onFlag,
		Client:  client,
	}, nil
}

// getMachinesDir returns the directory where machine state is stored
var getMachinesDir = func() string {
	return filepath.Join(config.GetCiloHome(), "machines")
}

// sanitizeHost converts a hostname to a safe directory name
func sanitizeHost(host string) string {
	sanitized := strings.ReplaceAll(host, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "[", "_")
	sanitized = strings.ReplaceAll(sanitized, "]", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	return sanitized
}

// GetMachine retrieves a connected machine by host
// Returns nil if machine is not connected
func GetMachine(host string) (*Machine, error) {
	machinesDir := getMachinesDir()
	statePath := filepath.Join(machinesDir, sanitizeHost(host), "state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read machine state: %w", err)
	}

	var machine Machine
	if err := json.Unmarshal(data, &machine); err != nil {
		return nil, nil
	}

	return &machine, nil
}

// IsConnected checks if a machine is currently connected
func IsConnected(host string) bool {
	machine, err := GetMachine(host)
	if err != nil {
		return false
	}
	return machine != nil
}

// ListConnectedMachines returns all connected machines
func ListConnectedMachines() ([]Machine, error) {
	machinesDir := getMachinesDir()

	entries, err := os.ReadDir(machinesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Machine{}, nil
		}
		return nil, fmt.Errorf("failed to read machines directory: %w", err)
	}

	var machines []Machine
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		statePath := filepath.Join(machinesDir, entry.Name(), "state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}

		var machine Machine
		if err := json.Unmarshal(data, &machine); err != nil {
			continue
		}

		machines = append(machines, machine)
	}

	return machines, nil
}

// SaveMachine persists machine connection state to disk
func SaveMachine(machine *Machine) error {
	if machine.Version == 0 {
		machine.Version = 1
	}

	machinesDir := getMachinesDir()
	machineDir := filepath.Join(machinesDir, sanitizeHost(machine.Host))

	if err := os.MkdirAll(machineDir, 0755); err != nil {
		return fmt.Errorf("failed to create machine directory: %w", err)
	}

	statePath := filepath.Join(machineDir, "state.json")
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal machine state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write machine state: %w", err)
	}

	wgKeyPath := filepath.Join(machineDir, "wg-key")
	wgKeyData := fmt.Sprintf("%s\n%s\n", machine.WGPrivateKey, machine.WGPublicKey)
	if err := os.WriteFile(wgKeyPath, []byte(wgKeyData), 0600); err != nil {
		return fmt.Errorf("failed to write wireguard keys: %w", err)
	}

	return nil
}

// RemoveMachine removes a machine's connection state
func RemoveMachine(host string) error {
	machinesDir := getMachinesDir()
	machineDir := filepath.Join(machinesDir, sanitizeHost(host))

	if err := os.RemoveAll(machineDir); err != nil {
		return fmt.Errorf("failed to remove machine directory: %w", err)
	}

	return nil
}
