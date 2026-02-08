package runtimes

import (
	"reflect"
	"testing"

	"github.com/sharedco/cilo/pkg/engine"
)

// TestExtractServiceName tests the extractServiceName function with various container naming formats.
func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		want          string
	}{
		{
			name:          "standard format with project and instance",
			containerName: "myapp_web_1",
			want:          "web",
		},
		{
			name:          "standard format with different service",
			containerName: "myapp_database_1",
			want:          "database",
		},
		{
			name:          "multi-part project name",
			containerName: "my-cool-app_api_1",
			want:          "api",
		},
		{
			name:          "service name with underscore - takes second to last",
			containerName: "project_db_primary_1",
			want:          "primary",
		},
		{
			name:          "higher instance number",
			containerName: "myapp_worker_5",
			want:          "worker",
		},
		{
			name:          "single underscore only",
			containerName: "project_service",
			want:          "project",
		},
		{
			name:          "no underscores",
			containerName: "web",
			want:          "web",
		},
		{
			name:          "empty string",
			containerName: "",
			want:          "",
		},
		{
			name:          "many parts - takes second to last",
			containerName: "a_b_c_d_e_f_1",
			want:          "f",
		},
		{
			name:          "podman default format",
			containerName: "cilo_myenv_redis_1",
			want:          "redis",
		},
		{
			name:          "service with hyphens",
			containerName: "myapp_my-service_1",
			want:          "my-service",
		},
		{
			name:          "numeric service name",
			containerName: "project_8080_1",
			want:          "8080",
		},
		{
			name:          "instance number only",
			containerName: "_1",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceName(tt.containerName)
			if got != tt.want {
				t.Errorf("extractServiceName(%q) = %q, want %q", tt.containerName, got, tt.want)
			}
		})
	}
}

// TestParsePortMappings tests the parsePortMappings function with various port formats.
func TestParsePortMappings(t *testing.T) {
	tests := []struct {
		name  string
		ports []string
		want  []engine.PortMapping
	}{
		{
			name:  "empty ports list",
			ports: []string{},
			want:  []engine.PortMapping{},
		},
		{
			name:  "nil ports list",
			ports: nil,
			want:  []engine.PortMapping{},
		},
		{
			name:  "simple unpublished port",
			ports: []string{"80/tcp"},
			want: []engine.PortMapping{
				{ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "unpublished port with udp",
			ports: []string{"53/udp"},
			want: []engine.PortMapping{
				{ContainerPort: 53, Protocol: "udp"},
			},
		},
		{
			name:  "published port with host IP",
			ports: []string{"0.0.0.0:8080->80/tcp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "published port without host IP",
			ports: []string{"8080->80/tcp"},
			want: []engine.PortMapping{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "published port with localhost",
			ports: []string{"127.0.0.1:3000->3000/tcp"},
			want: []engine.PortMapping{
				{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
			},
		},
		{
			name:  "published port with udp",
			ports: []string{"0.0.0.0:53->53/udp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 53, ContainerPort: 53, Protocol: "udp"},
			},
		},
		{
			name:  "multiple ports",
			ports: []string{"0.0.0.0:8080->80/tcp", "0.0.0.0:443->443/tcp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				{HostIP: "0.0.0.0", HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
			},
		},
		{
			name:  "mixed published and unpublished",
			ports: []string{"0.0.0.0:8080->80/tcp", "9000/tcp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				{ContainerPort: 9000, Protocol: "tcp"},
			},
		},
		{
			name:  "port without protocol defaults to tcp",
			ports: []string{"80"},
			want: []engine.PortMapping{
				{ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "published port without protocol",
			ports: []string{"8080->80"},
			want: []engine.PortMapping{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "high port numbers",
			ports: []string{"0.0.0.0:65535->65535/tcp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 65535, ContainerPort: 65535, Protocol: "tcp"},
			},
		},
		{
			name:  "IPv6 host IP - not fully supported by parser",
			ports: []string{"[::1]:8080->80/tcp"},
			want: []engine.PortMapping{
				{ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name:  "malformed port creates empty mapping",
			ports: []string{"invalid"},
			want: []engine.PortMapping{
				{Protocol: "tcp"},
			},
		},
		{
			name:  "malformed arrow format - parses host side only",
			ports: []string{"0.0.0.0:8080->"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 8080, Protocol: "tcp"},
			},
		},
		{
			name:  "empty string in ports list",
			ports: []string{""},
			want: []engine.PortMapping{
				{Protocol: "tcp"},
			},
		},
		{
			name:  "complex real-world example",
			ports: []string{"0.0.0.0:5432->5432/tcp", "0.0.0.0:6379->6379/tcp", "8080/tcp"},
			want: []engine.PortMapping{
				{HostIP: "0.0.0.0", HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"},
				{HostIP: "0.0.0.0", HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
				{ContainerPort: 8080, Protocol: "tcp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePortMappings(tt.ports)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePortMappings(%v) = %v, want %v", tt.ports, got, tt.want)
			}
		})
	}
}

// TestBuildComposeArgs tests the buildComposeArgs method.
func TestBuildComposeArgs(t *testing.T) {
	tests := []struct {
		name          string
		composeBinary string
		workdir       string
		project       string
		composeFile   string
		want          []string
	}{
		{
			name:          "podman-compose with all args",
			composeBinary: "podman-compose",
			workdir:       "/tmp/test",
			project:       "myapp",
			composeFile:   "/tmp/test/docker-compose.yml",
			want:          []string{"--project-name", "myapp", "-f", "/tmp/test/docker-compose.yml"},
		},
		{
			name:          "podman compose with all args",
			composeBinary: "podman compose",
			workdir:       "/tmp/test",
			project:       "myapp",
			composeFile:   "/tmp/test/docker-compose.yml",
			want:          []string{"compose", "--project-name", "myapp", "-f", "/tmp/test/docker-compose.yml"},
		},
		{
			name:          "podman-compose without compose file",
			composeBinary: "podman-compose",
			workdir:       "/tmp/test",
			project:       "myapp",
			composeFile:   "",
			want:          []string{"--project-name", "myapp"},
		},
		{
			name:          "podman compose without compose file",
			composeBinary: "podman compose",
			workdir:       "/tmp/test",
			project:       "myapp",
			composeFile:   "",
			want:          []string{"compose", "--project-name", "myapp"},
		},
		{
			name:          "project with hyphen",
			composeBinary: "podman-compose",
			workdir:       "/home/user/projects",
			project:       "my-cool-app",
			composeFile:   "/home/user/projects/docker-compose.yaml",
			want:          []string{"--project-name", "my-cool-app", "-f", "/home/user/projects/docker-compose.yaml"},
		},
		{
			name:          "project with underscore",
			composeBinary: "podman-compose",
			workdir:       "/home/user/projects",
			project:       "my_cool_app",
			composeFile:   "/home/user/projects/docker-compose.yaml",
			want:          []string{"--project-name", "my_cool_app", "-f", "/home/user/projects/docker-compose.yaml"},
		},
		{
			name:          "absolute path with spaces",
			composeBinary: "podman-compose",
			workdir:       "/tmp/my project",
			project:       "test",
			composeFile:   "/tmp/my project/docker-compose.yml",
			want:          []string{"--project-name", "test", "-f", "/tmp/my project/docker-compose.yml"},
		},
		{
			name:          "podman-compose with relative compose file",
			composeBinary: "podman-compose",
			workdir:       "/tmp/test",
			project:       "myapp",
			composeFile:   "docker-compose.yml",
			want:          []string{"--project-name", "myapp", "-f", "docker-compose.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PodmanRuntime{
				composeBinary: tt.composeBinary,
			}
			got := r.buildComposeArgs(tt.workdir, tt.project, tt.composeFile)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildComposeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPodmanRuntimeName tests the Name method of PodmanRuntime.
func TestPodmanRuntimeName(t *testing.T) {
	tests := []struct {
		name     string
		runtime  PodmanRuntime
		expected string
	}{
		{
			name: "basic runtime",
			runtime: PodmanRuntime{
				composeBinary: "podman-compose",
				podmanBinary:  "podman",
			},
			expected: "podman",
		},
		{
			name: "with podman compose",
			runtime: PodmanRuntime{
				composeBinary: "podman compose",
				podmanBinary:  "podman",
			},
			expected: "podman",
		},
		{
			name: "empty runtime",
			runtime: PodmanRuntime{
				composeBinary: "",
				podmanBinary:  "",
			},
			expected: "podman",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.runtime.Name()
			if got != tt.expected {
				t.Errorf("Name() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPodmanRuntimeStructFields tests that the PodmanRuntime struct can be properly initialized.
func TestPodmanRuntimeStructFields(t *testing.T) {
	tests := []struct {
		name          string
		composeBinary string
		podmanBinary  string
	}{
		{
			name:          "podman-compose setup",
			composeBinary: "podman-compose",
			podmanBinary:  "podman",
		},
		{
			name:          "podman compose setup",
			composeBinary: "podman compose",
			podmanBinary:  "podman",
		},
		{
			name:          "custom paths",
			composeBinary: "/usr/local/bin/podman-compose",
			podmanBinary:  "/usr/bin/podman",
		},
		{
			name:          "empty strings",
			composeBinary: "",
			podmanBinary:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PodmanRuntime{
				composeBinary: tt.composeBinary,
				podmanBinary:  tt.podmanBinary,
			}

			if r.composeBinary != tt.composeBinary {
				t.Errorf("composeBinary = %q, want %q", r.composeBinary, tt.composeBinary)
			}
			if r.podmanBinary != tt.podmanBinary {
				t.Errorf("podmanBinary = %q, want %q", r.podmanBinary, tt.podmanBinary)
			}
		})
	}
}
