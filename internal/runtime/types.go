// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package runtime

import (
	"io"
	"time"
)

// EnvironmentState represents the state of an environment
type EnvironmentState string

const (
	StateCreated EnvironmentState = "created"
	StateRunning EnvironmentState = "running"
	StateStopped EnvironmentState = "stopped"
	StateError   EnvironmentState = "error"
)

// CreateOptions for environment creation
type CreateOptions struct {
	CopyFiles bool
}

// UpOptions for starting environments
type UpOptions struct {
	Build    bool
	Recreate bool
}

// ExecOptions for executing commands
type ExecOptions struct {
	Interactive bool
	TTY         bool
	Env         map[string]string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// LogOptions for retrieving logs
type LogOptions struct {
	Follow bool
	Tail   int
	Since  time.Time
	Stdout io.Writer
	Stderr io.Writer
}

// ComposeOptions for running arbitrary compose commands
type ComposeOptions struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Status represents environment status
type Status struct {
	State       EnvironmentState
	Services    map[string]ServiceStatus
	LastUpdated time.Time
}

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Name      string
	State     string
	IP        string
	Container string
}

// Network represents a container network
type Network struct {
	ID     string
	Name   string
	Subnet string
	Driver string
}
