package models

import (
	"time"
)

// State represents global cilo state
type State struct {
	Version        int                       `json:"version"`
	BaseSubnet     string                    `json:"base_subnet,omitempty"`
	DNSPort        int                       `json:"dns_port,omitempty"`
	SubnetCounter  int                       `json:"subnet_counter"`
	Hosts          map[string]*Host          `json:"hosts"`
	SharedNetworks map[string]*SharedNetwork `json:"shared_networks,omitempty"`
	SharedServices map[string]*SharedService `json:"shared_services,omitempty"`
}

// Host represents a machine or server where environments run
type Host struct {
	ID           string                  `json:"id"`
	Provider     string                  `json:"provider,omitempty"`
	MeshProvider string                  `json:"mesh_provider,omitempty"`
	MeshID       string                  `json:"mesh_id,omitempty"`
	Environments map[string]*Environment `json:"environments"`
}

// SharedNetwork represents a network shared across multiple environments
type SharedNetwork struct {
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`    // Environment key that created it
	ReferencedBy []string  `json:"referenced_by"` // Environment keys using it
}

// SharedService represents a service shared across multiple environments
type SharedService struct {
	Name              string    `json:"name"`        // Service name (e.g., "elasticsearch")
	Container         string    `json:"container"`   // Docker container name (e.g., "cilo_shared_elasticsearch")
	IP                string    `json:"ip"`          // Primary IP address
	Project           string    `json:"project"`     // Project that owns this shared service
	Image             string    `json:"image"`       // Image with tag for conflict detection
	ConfigHash        string    `json:"config_hash"` // Hash of service definition for conflict detection
	CreatedAt         time.Time `json:"created_at"`
	UsedBy            []string  `json:"used_by"`            // Environment keys: "project/env"
	DisconnectTimeout time.Time `json:"disconnect_timeout"` // Grace period timestamp
}

// Environment represents a single isolated workspace
type Environment struct {
	Name               string              `json:"name"`
	Project            string              `json:"project,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	Subnet             string              `json:"subnet"`
	DNSSuffix          string              `json:"dns_suffix,omitempty"`
	Status             string              `json:"status"`
	Source             string              `json:"source,omitempty"`
	Services           map[string]*Service `json:"services"`
	SharedNetworks     []string            `json:"shared_networks,omitempty"`      // Names of shared networks
	UsesSharedServices []string            `json:"uses_shared_services,omitempty"` // Names of shared services this env consumes
}

// Service represents a service within an environment
type Service struct {
	Name      string   `json:"name"`
	IP        string   `json:"ip"`
	Container string   `json:"container"`
	URL       string   `json:"url,omitempty"`
	IsIngress bool     `json:"is_ingress,omitempty"`
	Hostnames []string `json:"hostnames,omitempty"`
}

// ComposeService represents a service in a docker-compose file
type ComposeService struct {
	Image         string            `yaml:"image,omitempty"`
	Build         interface{}       `yaml:"build,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Environment   interface{}       `yaml:"environment,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Networks      interface{}       `yaml:"networks,omitempty"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Command       interface{}       `yaml:"command,omitempty"`
	WorkingDir    string            `yaml:"working_dir,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

// ComposeFile represents a docker-compose.yml structure
type ComposeFile struct {
	Version  string                     `yaml:"version,omitempty"`
	Services map[string]*ComposeService `yaml:"services"`
	Networks map[string]*ComposeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}     `yaml:"volumes,omitempty"`
}

// ComposeNetwork represents a network configuration
type ComposeNetwork struct {
	Driver string             `yaml:"driver"`
	IPAM   *ComposeIPAMConfig `yaml:"ipam,omitempty"`
}

// ComposeIPAMConfig represents IP address management configuration
type ComposeIPAMConfig struct {
	Subnet string `yaml:"subnet"`
}

// ProjectConfig represents a .cilo/config.yml file
// This configures how cilo works for a specific project
type ProjectConfig struct {
	Project               string     `yaml:"project"`
	BuildTool             string     `yaml:"build_tool,omitempty"`
	ComposeFiles          []string   `yaml:"compose_files"`
	EnvFiles              []string   `yaml:"env_files,omitempty"`
	DNSSuffix             string     `yaml:"dns_suffix,omitempty"`
	DefaultEnvironment    string     `yaml:"default_environment,omitempty"`
	DefaultIngressService string     `yaml:"default_ingress_service,omitempty"`
	Hostnames             []string   `yaml:"hostnames,omitempty"`
	Environments          []string   `yaml:"environments,omitempty"`
	CopyDotDirs           []string   `yaml:"copy_dot_dirs,omitempty"`
	IgnoreDotDirs         []string   `yaml:"ignore_dot_dirs,omitempty"`
	Env                   *EnvConfig `yaml:"env,omitempty"`
}

// EnvConfig controls env file handling for a project
// This is a generic, config-driven mechanism to copy, initialize, and rewrite env files per environment.
type EnvConfig struct {
	CopyMode string          `yaml:"copy_mode,omitempty"` // "all" (default), "none", "allowlist"
	Copy     []string        `yaml:"copy,omitempty"`
	Ignore   []string        `yaml:"ignore,omitempty"`
	InitHook string          `yaml:"init_hook,omitempty"`
	Render   []EnvRenderRule `yaml:"render,omitempty"`
}

// EnvRenderRule describes how to rewrite an env file.
type EnvRenderRule struct {
	File    string       `yaml:"file"`
	Tokens  bool         `yaml:"tokens,omitempty"`
	Replace []EnvReplace `yaml:"replace,omitempty"`
}

// EnvReplace defines a simple string replacement.
type EnvReplace struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}
