// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServices_MergesLabels(t *testing.T) {
	root := t.TempDir()
	fileA := filepath.Join(root, "compose-a.yml")
	fileB := filepath.Join(root, "compose-b.yml")

	contentA := `version: "3.8"
services:
  api:
    image: nginx:alpine
    labels:
      - "cilo.ingress=true"
      - "cilo.hostnames=api.primary"
`
	contentB := `version: "3.8"
services:
  api:
    labels:
      cilo.hostnames: api.override
  web:
    labels:
      cilo.ingress: "true"
`

	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	services, err := LoadServices([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("LoadServices: %v", err)
	}

	api := services["api"]
	if api == nil {
		t.Fatalf("expected api service")
	}
	if api.Labels["cilo.hostnames"] != "api.override" {
		t.Fatalf("expected override label, got %q", api.Labels["cilo.hostnames"])
	}

	web := services["web"]
	if web == nil {
		t.Fatalf("expected web service")
	}
	if web.Labels["cilo.ingress"] != "true" {
		t.Fatalf("expected web ingress label, got %q", web.Labels["cilo.ingress"])
	}
}

func TestResolveComposeFiles_ProjectDir(t *testing.T) {
	root := t.TempDir()
	composeDir := filepath.Join(root, "docker")
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	composePath := filepath.Join(composeDir, "compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	files, projectDir, err := ResolveComposeFiles(root, []string{"docker/compose.yml"})
	if err != nil {
		t.Fatalf("ResolveComposeFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if projectDir != composeDir {
		t.Fatalf("expected project dir %s, got %s", composeDir, projectDir)
	}
}
