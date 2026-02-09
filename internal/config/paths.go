// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package config

import (
	"os"
	"path/filepath"
)

func GetCiloHome() string {
	if home := os.Getenv("CILO_USER_HOME"); home != "" {
		return filepath.Join(home, ".cilo")
	}
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return filepath.Join("/home", sudoUser, ".cilo")
	}
	return filepath.Join(os.Getenv("HOME"), ".cilo")
}

func GetStatePath() string {
	return filepath.Join(GetCiloHome(), "state.json")
}

func GetEnvsDir() string {
	return filepath.Join(GetCiloHome(), "envs")
}

func GetDNSDir() string {
	return filepath.Join(GetCiloHome(), "dns")
}

func GetEnvPath(project, name string) string {
	return filepath.Join(GetEnvsDir(), project, name)
}
