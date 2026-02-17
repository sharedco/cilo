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

type RemoteMachine struct {
	Host         string
	WGAssignedIP string
}

const (
	remoteMachineStartMarker = "# Remote machine: %s\n"
	remoteMachineEndMarker   = "# End remote machine: %s\n"
)

func AddRemoteMachine(machine *RemoteMachine, envs []cilod.Environment) error {
	configPath := getCiloConfPath()
	if configPath == "" {
		configPath = filepath.Join(getDNSDir(), dnsConfFile)
	}

	var config string
	data, err := os.ReadFile(configPath)
	if err == nil {
		config = string(data)
	}

	config = removeRemoteMachineEntries(config, machine.Host)

	var entries strings.Builder
	entries.WriteString(fmt.Sprintf(remoteMachineStartMarker, machine.Host))

	for _, env := range envs {
		for _, svc := range env.Services {
			hostname := fmt.Sprintf("%s.%s.test", svc, env.Name)
			entries.WriteString(fmt.Sprintf("address=/%s/%s\n", hostname, machine.WGAssignedIP))
		}
	}

	entries.WriteString(fmt.Sprintf(remoteMachineEndMarker, machine.Host))

	config += entries.String()

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write DNS config: %w", err)
	}

	return reloadDNS()
}

func RemoveRemoteMachine(host string) error {
	configPath := getCiloConfPath()
	if configPath == "" {
		configPath = filepath.Join(getDNSDir(), dnsConfFile)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read DNS config: %w", err)
	}

	config := string(data)
	config = removeRemoteMachineEntries(config, host)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write DNS config: %w", err)
	}

	return reloadDNS()
}

func UpdateRemoteDNSEntries(machine *RemoteMachine, envs []cilod.Environment) error {
	if err := RemoveRemoteMachine(machine.Host); err != nil {
		return err
	}
	return AddRemoteMachine(machine, envs)
}

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
			config = config[:start]
			break
		}

		config = config[:start] + config[start+end+len(endMarker):]
	}

	return config
}
