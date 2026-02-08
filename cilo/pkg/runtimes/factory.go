package runtimes

import (
	"fmt"

	"github.com/sharedco/cilo/pkg/engine"
)

// Detect returns the best available runtime.
// It tries Docker first (most common), then Podman.
// Returns an error if no container runtime is found.
func Detect() (engine.Runtime, error) {
	// Try Docker first (most common)
	if r, err := NewDockerRuntime(); err == nil {
		return r, nil
	}

	// Try Podman
	if r, err := NewPodmanRuntime(); err == nil {
		return r, nil
	}

	return nil, fmt.Errorf("no container runtime found (docker or podman required)")
}

// Get returns a specific runtime by name.
// Supported names: "docker", "podman"
// Returns an error if the runtime name is unknown or unavailable.
func Get(name string) (engine.Runtime, error) {
	switch name {
	case "docker":
		return NewDockerRuntime()
	case "podman":
		return NewPodmanRuntime()
	default:
		return nil, fmt.Errorf("unknown runtime: %s", name)
	}
}

// Available returns a list of available runtime names.
// Only returns runtimes that are actually available on the system.
func Available() []string {
	var runtimes []string

	if _, err := NewDockerRuntime(); err == nil {
		runtimes = append(runtimes, "docker")
	}

	if _, err := NewPodmanRuntime(); err == nil {
		runtimes = append(runtimes, "podman")
	}

	return runtimes
}

// IsAvailable checks if a specific runtime is available.
func IsAvailable(name string) bool {
	_, err := Get(name)
	return err == nil
}
