// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package runtime

import (
	"os/exec"
)

type ProviderType string

const (
	Docker ProviderType = "docker"
	Podman ProviderType = "podman"
)

func DetectProvider() ProviderType {
	if _, err := exec.LookPath("docker"); err == nil {
		return Docker
	}

	if _, err := exec.LookPath("podman"); err == nil {
		return Podman
	}

	return Docker
}
