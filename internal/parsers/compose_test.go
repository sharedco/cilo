// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package parsers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComposeParser_Name(t *testing.T) {
	parser := &ComposeParser{}
	if parser.Name() != "compose" {
		t.Errorf("expected name 'compose', got '%s'", parser.Name())
	}
}

func TestComposeParser_Detect(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"compose.yml", "compose.yml", true},
		{"compose.yaml", "compose.yaml", true},
		{"docker-compose.yml", "docker-compose.yml", true},
		{"docker-compose.yaml", "docker-compose.yaml", true},
		{"no compose file", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.filename != "" {
				// Create an empty compose file
				path := filepath.Join(tmpDir, tt.filename)
				if err := os.WriteFile(path, []byte("version: '3'\nservices: {}\n"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			parser := &ComposeParser{}
			if got := parser.Detect(tmpDir); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComposeParser_Parse(t *testing.T) {
	tests := []struct {
		name        string
		composeYaml string
		wantErr     bool
		wantService string
		wantImage   string
	}{
		{
			name: "simple service",
			composeYaml: `version: '3'
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    environment:
      FOO: bar
    labels:
      cilo.ingress: "true"
`,
			wantErr:     false,
			wantService: "web",
			wantImage:   "nginx:alpine",
		},
		{
			name: "service with build",
			composeYaml: `version: '3'
services:
  api:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
`,
			wantErr:     false,
			wantService: "api",
			wantImage:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")

			if err := os.WriteFile(composePath, []byte(tt.composeYaml), 0644); err != nil {
				t.Fatalf("failed to create compose file: %v", err)
			}

			parser := &ComposeParser{}
			spec, err := parser.Parse(tmpDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if spec.Source != "compose" {
				t.Errorf("expected source 'compose', got '%s'", spec.Source)
			}

			if len(spec.Services) == 0 {
				t.Fatal("expected at least one service")
			}

			service := spec.Services[0]
			if service.Name != tt.wantService {
				t.Errorf("expected service name '%s', got '%s'", tt.wantService, service.Name)
			}

			if tt.wantImage != "" && service.Image != tt.wantImage {
				t.Errorf("expected image '%s', got '%s'", tt.wantImage, service.Image)
			}

			if tt.name == "simple service" {
				// Check environment
				if service.Env["FOO"] != "bar" {
					t.Errorf("expected env FOO=bar, got %v", service.Env)
				}

				// Check labels
				if service.Labels["cilo.ingress"] != "true" {
					t.Errorf("expected label cilo.ingress=true, got %v", service.Labels)
				}

				// Check ports
				if len(service.Ports) != 1 {
					t.Fatalf("expected 1 port, got %d", len(service.Ports))
				}
				port := service.Ports[0]
				if port.Target != 80 || port.Published != 8080 {
					t.Errorf("expected port 8080:80, got %d:%d", port.Published, port.Target)
				}
			}

			if tt.name == "service with build" {
				// Check build
				if service.Build == nil {
					t.Fatal("expected build spec")
				}
				if service.Build.Context != "." {
					t.Errorf("expected build context '.', got '%s'", service.Build.Context)
				}
				if service.Build.Dockerfile != "Dockerfile" {
					t.Errorf("expected dockerfile 'Dockerfile', got '%s'", service.Build.Dockerfile)
				}
			}
		})
	}
}

func TestConvertPorts(t *testing.T) {
	tests := []struct {
		name  string
		input []interface{}
		want  int
	}{
		{
			name:  "short syntax",
			input: []interface{}{"8080:80"},
			want:  1,
		},
		{
			name:  "with protocol",
			input: []interface{}{"8080:80/tcp"},
			want:  1,
		},
		{
			name:  "with host IP",
			input: []interface{}{"127.0.0.1:8080:80"},
			want:  1,
		},
		{
			name: "long syntax",
			input: []interface{}{
				map[string]interface{}{
					"target":    80,
					"published": 8080,
					"protocol":  "tcp",
				},
			},
			want: 1,
		},
		{
			name:  "mixed formats",
			input: []interface{}{"8080:80", "9090:90/udp"},
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPorts(tt.input)
			if len(result) != tt.want {
				t.Errorf("convertPorts() returned %d ports, want %d", len(result), tt.want)
			}

			// Verify first port
			if len(result) > 0 {
				if result[0].Target == 0 {
					t.Error("expected non-zero target port")
				}
			}
		})
	}
}

func TestConvertVolumes(t *testing.T) {
	tests := []struct {
		name  string
		input []interface{}
		want  int
	}{
		{
			name:  "short bind syntax",
			input: []interface{}{"./data:/app/data"},
			want:  1,
		},
		{
			name:  "short bind with readonly",
			input: []interface{}{"./data:/app/data:ro"},
			want:  1,
		},
		{
			name:  "named volume",
			input: []interface{}{"myvolume:/app/data"},
			want:  1,
		},
		{
			name: "long syntax",
			input: []interface{}{
				map[string]interface{}{
					"type":   "bind",
					"source": "./data",
					"target": "/app/data",
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertVolumes(tt.input)
			if len(result) != tt.want {
				t.Errorf("convertVolumes() returned %d volumes, want %d", len(result), tt.want)
			}

			// Verify first volume has target
			if len(result) > 0 {
				if result[0].Target == "" {
					t.Error("expected non-empty target")
				}
			}
		})
	}
}

func TestConvertEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  map[string]string
	}{
		{
			name:  "map format",
			input: map[string]interface{}{"FOO": "bar", "BAZ": "qux"},
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "list format",
			input: []interface{}{"FOO=bar", "BAZ=qux"},
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "list with empty values",
			input: []interface{}{"FOO=", "BAZ"},
			want:  map[string]string{"FOO": "", "BAZ": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertEnvironment(tt.input)
			if len(result) != len(tt.want) {
				t.Errorf("convertEnvironment() returned %d items, want %d", len(result), len(tt.want))
			}

			for k, v := range tt.want {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

func TestConvertDependsOn(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{
			name:  "list format",
			input: []interface{}{"db", "redis"},
			want:  2,
		},
		{
			name: "map format",
			input: map[string]interface{}{
				"db":    map[string]interface{}{"condition": "service_healthy"},
				"redis": map[string]interface{}{"condition": "service_started"},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDependsOn(tt.input)
			if len(result) != tt.want {
				t.Errorf("convertDependsOn() returned %d items, want %d", len(result), tt.want)
			}
		})
	}
}
