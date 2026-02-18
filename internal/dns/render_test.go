// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package dns

import (
	"strings"
	"testing"

	"github.com/sharedco/cilo/internal/models"
)

func TestRenderConfig_CustomSuffix(t *testing.T) {
	state := &models.State{
		Hosts: map[string]*models.Host{
			"local": {
				Environments: map[string]*models.Environment{
					"dev": {
						Name:      "dev",
						Project:   "myproj",
						DNSSuffix: ".localhost",
						Services: map[string]*models.Service{
							"web": {
								Name:      "web",
								IP:        "10.224.1.2",
								IsIngress: true,
							},
						},
					},
				},
			},
		},
	}

	config, err := RenderConfig(state)
	if err != nil {
		t.Fatalf("RenderConfig failed: %v", err)
	}

	// Check service-specific hostname
	if !strings.Contains(config, "address=/web.dev.localhost/10.224.1.2") {
		t.Errorf("Expected web.dev.localhost entry, got:\n%s", config)
	}

	// Check wildcard entry for ingress
	if !strings.Contains(config, "address=/.myproj.dev.localhost/10.224.1.2") {
		t.Errorf("Expected .myproj.dev.localhost entry, got:\n%s", config)
	}

	// Check apex entry for ingress
	if !strings.Contains(config, "address=/myproj.dev.localhost/10.224.1.2") {
		t.Errorf("Expected myproj.dev.localhost entry, got:\n%s", config)
	}
}

func TestRenderConfig_DefaultSuffix(t *testing.T) {
	state := &models.State{
		Hosts: map[string]*models.Host{
			"local": {
				Environments: map[string]*models.Environment{
					"dev": {
						Name:    "dev",
						Project: "myproj",
						Services: map[string]*models.Service{
							"web": {
								Name: "web",
								IP:   "10.224.1.2",
							},
						},
					},
				},
			},
		},
	}

	config, err := RenderConfig(state)
	if err != nil {
		t.Fatalf("RenderConfig failed: %v", err)
	}

	if !strings.Contains(config, "address=/web.dev.test/10.224.1.2") {
		t.Errorf("Expected web.dev.test entry, got:\n%s", config)
	}
}
