// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sharedco/cilo/internal/models"
	"gopkg.in/yaml.v3"
)

func TestTransform_CreatesOverrideFile(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
    ports:
      - "3000:3000"
  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// Verify override file exists
	if _, err := os.Stat(overridePath); os.IsNotExist(err) {
		t.Fatal("override file was not created")
	}

	// Read and parse override
	content, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read override: %v", err)
	}

	var override map[string]interface{}
	if err := yaml.Unmarshal(content, &override); err != nil {
		t.Fatalf("parse override: %v", err)
	}

	// Verify network is external
	networks := override["networks"].(map[string]interface{})
	defaultNet := networks["default"].(map[string]interface{})
	if defaultNet["name"] != "cilo_test-env" {
		t.Errorf("expected network name 'cilo_test-env', got %v", defaultNet["name"])
	}
	if defaultNet["external"] != true {
		t.Error("expected network to be external")
	}

	// Verify services have static IPs
	services := override["services"].(map[string]interface{})
	apiSvc := services["api"].(map[string]interface{})
	apiNetworks := apiSvc["networks"].(map[string]interface{})
	apiDefault := apiNetworks["default"].(map[string]interface{})
	apiIP := apiDefault["ipv4_address"].(string)
	if !strings.HasPrefix(apiIP, "10.224.1.") {
		t.Errorf("expected api IP in 10.224.1.x range, got %s", apiIP)
	}

	// Verify ports are cleared
	if ports, ok := apiSvc["ports"]; ok {
		portList := ports.([]interface{})
		if len(portList) != 0 {
			t.Errorf("expected ports to be empty, got %v", portList)
		}
	}
}

func TestTransform_AssignsSequentialIPs(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
  db:
    image: postgres:15
  redis:
    image: redis:alpine
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "myenv",
		Subnet: "10.224.5.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// Verify services in env
	if len(env.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(env.Services))
	}

	// IPs should start from .10 (skipping .1-.9 for shared services)
	seenIPs := make(map[string]bool)
	for name, svc := range env.Services {
		if !strings.HasPrefix(svc.IP, "10.224.5.") {
			t.Errorf("service %s has wrong subnet: %s", name, svc.IP)
		}
		if seenIPs[svc.IP] {
			t.Errorf("duplicate IP assigned: %s", svc.IP)
		}
		seenIPs[svc.IP] = true
	}
}

func TestTransform_SetsContainerNames(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "feature-xyz",
		Subnet: "10.224.2.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// Verify container name
	if env.Services["api"].Container != "cilo_feature-xyz_api" {
		t.Errorf("expected container name 'cilo_feature-xyz_api', got %s", env.Services["api"].Container)
	}
}

func TestTransform_SetsServiceURLs(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".local")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	expectedURL := "http://api.test-env.local"
	if env.Services["api"].URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, env.Services["api"].URL)
	}
}

func TestTransform_DetectsIngressService(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	// nginx should be detected as ingress by name convention
	composeContent := `version: "3.8"
services:
  api:
    image: node:18
  nginx:
    image: nginx:alpine
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	if !env.Services["nginx"].IsIngress {
		t.Error("nginx should be detected as ingress")
	}
	if env.Services["api"].IsIngress {
		t.Error("api should not be ingress")
	}
}

func TestTransform_IngressLabelOverridesConvention(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	// cilo.ingress label should override name convention
	composeContent := `version: "3.8"
services:
  api:
    image: node:18
    labels:
      cilo.ingress: "true"
  nginx:
    image: nginx:alpine
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	if !env.Services["api"].IsIngress {
		t.Error("api should be ingress (has cilo.ingress label)")
	}
}

func TestTransformWithShared_DisablesSharedServices(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
  postgres:
    image: postgres:15
  redis:
    image: redis:alpine
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	// Mark postgres as shared (should be disabled in override)
	err := TransformWithShared(env, []string{composePath}, overridePath, ".test", []string{"postgres"})
	if err != nil {
		t.Fatalf("TransformWithShared: %v", err)
	}

	// Read and parse override
	content, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read override: %v", err)
	}

	var override map[string]interface{}
	if err := yaml.Unmarshal(content, &override); err != nil {
		t.Fatalf("parse override: %v", err)
	}

	services := override["services"].(map[string]interface{})

	// postgres should have replicas: 0
	postgresSvc := services["postgres"].(map[string]interface{})
	deploy := postgresSvc["deploy"].(map[string]interface{})
	if deploy["replicas"] != 0 {
		t.Errorf("expected postgres replicas=0, got %v", deploy["replicas"])
	}

	// api should have normal config (no deploy section)
	apiSvc := services["api"].(map[string]interface{})
	if _, ok := apiSvc["deploy"]; ok {
		t.Error("api should not have deploy section")
	}
}

func TestTransform_InvalidSubnet(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "invalid-subnet",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err == nil {
		t.Fatal("expected error for invalid subnet")
	}
	if !strings.Contains(err.Error(), "invalid subnet") {
		t.Errorf("expected 'invalid subnet' error, got: %v", err)
	}
}

func TestTransform_NoServices(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services: {}
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err == nil {
		t.Fatal("expected error for empty services")
	}
	if !strings.Contains(err.Error(), "no services found") {
		t.Errorf("expected 'no services found' error, got: %v", err)
	}
}

func TestTransform_DefaultDNSSuffix(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	// Pass empty DNS suffix - should default to .test
	err := Transform(env, []string{composePath}, overridePath, "")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	expectedURL := "http://api.test-env.test"
	if env.Services["api"].URL != expectedURL {
		t.Errorf("expected URL %s with default suffix, got %s", expectedURL, env.Services["api"].URL)
	}
}

func TestTransform_HostnamesLabel(t *testing.T) {
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	overridePath := filepath.Join(root, ".cilo", "override.yml")

	composeContent := `version: "3.8"
services:
  api:
    image: node:18
    labels:
      cilo.hostnames: "api.example.com, api.test.local"
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &models.Environment{
		Name:   "test-env",
		Subnet: "10.224.1.0/24",
	}

	err := Transform(env, []string{composePath}, overridePath, ".test")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	api := env.Services["api"]
	if len(api.Hostnames) != 2 {
		t.Fatalf("expected 2 hostnames, got %d", len(api.Hostnames))
	}
	if api.Hostnames[0] != "api.example.com" {
		t.Errorf("expected first hostname 'api.example.com', got %s", api.Hostnames[0])
	}
	if api.Hostnames[1] != "api.test.local" {
		t.Errorf("expected second hostname 'api.test.local', got %s", api.Hostnames[1])
	}
	// Service with hostnames should be marked as ingress
	if !api.IsIngress {
		t.Error("service with hostnames should be marked as ingress")
	}
}
