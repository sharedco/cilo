package models

import (
	"time"
)

// State represents the global cilo state
type State struct {
	Version       int                     `json:"version"`
	SubnetCounter int                     `json:"subnet_counter"`
	Environments  map[string]*Environment `json:"environments"`
}

// Environment represents a single isolated workspace
type Environment struct {
	Name      string              `json:"name"`
	Project   string              `json:"project,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	Subnet    string              `json:"subnet"`
	DNSSuffix string              `json:"dns_suffix,omitempty"`
	Status    string              `json:"status"`
	Source    string              `json:"source,omitempty"`
	Services  map[string]*Service `json:"services"`
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
	Driver string       `yaml:"driver"`
	IPAM   *ComposeIPAM `yaml:"ipam,omitempty"`
}

// ComposeIPAM represents IP address management configuration
type ComposeIPAM struct {
	Config []ComposeIPAMConfig `yaml:"config"`
}

// ComposeIPAMConfig represents IPAM configuration
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
