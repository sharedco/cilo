// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

// Package version provides version information for all Cilo components.
// The version is set at build time via ldflags.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of Cilo (set by ldflags)
	Version = "dev"

	// Commit is the git commit hash (set by ldflags)
	Commit = "unknown"

	// BuildTime is the build timestamp (set by ldflags)
	BuildTime = "unknown"
)

// Info returns a formatted version string
func Info() string {
	return fmt.Sprintf("Cilo %s (commit: %s, built: %s, go: %s)",
		Version, Commit, BuildTime, runtime.Version())
}

// Short returns just the version number
func Short() string {
	return Version
}
