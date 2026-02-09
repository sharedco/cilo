// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectConfigFromPath(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".cilo")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.yml")
	content := "project: demo\ncompose_files:\n  - docker-compose.yml\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config, err := LoadProjectConfigFromPath(root)
	if err != nil {
		t.Fatalf("LoadProjectConfigFromPath: %v", err)
	}
	if config == nil {
		t.Fatalf("expected config")
	}
	if config.Project != "demo" {
		t.Fatalf("expected project demo, got %q", config.Project)
	}
	if len(config.ComposeFiles) != 1 || config.ComposeFiles[0] != "docker-compose.yml" {
		t.Fatalf("unexpected compose files: %v", config.ComposeFiles)
	}
}
