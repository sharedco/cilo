// Package engine provides the core abstraction layer for Cilo.
// It defines universal types and interfaces for parsing project configurations
// and executing environments across different container runtimes.
package engine

// EnvironmentSpec is the universal environment description that all parsers
// produce and all runtimes consume. It represents a complete, runtime-agnostic
// specification of an isolated environment.
type EnvironmentSpec struct {
	// Name is the environment identifier (e.g., "agent-1", "feature-auth")
	Name string

	// Project is the project identifier this environment belongs to
	Project string

	// Services defines all services in this environment
	Services []ServiceSpec

	// Networks defines custom networks for this environment
	Networks []NetworkSpec

	// Volumes defines named volumes for this environment
	Volumes []VolumeSpec

	// Source indicates which parser produced this spec ("compose", "devcontainer", "procfile")
	Source string

	// SourcePath is the absolute path to the source configuration file
	SourcePath string
}

// ServiceSpec defines a single service within an environment.
type ServiceSpec struct {
	// Name is the service identifier (e.g., "api", "db", "redis")
	Name string

	// Image is the container image to use (e.g., "postgres:15", "nginx:alpine")
	Image string

	// Build specifies how to build the image from source
	Build *BuildSpec

	// Command overrides the default container command
	Command []string

	// Entrypoint overrides the default container entrypoint
	Entrypoint []string

	// Ports defines port mappings for this service
	Ports []PortSpec

	// Env defines environment variables as key-value pairs
	Env map[string]string

	// EnvFile lists paths to environment files to load
	EnvFile []string

	// Volumes defines volume mounts for this service
	Volumes []VolumeMountSpec

	// DependsOn lists services that must start before this one
	DependsOn []string

	// Labels are metadata key-value pairs attached to the container
	Labels map[string]string

	// HealthCheck defines how to check if the service is healthy
	HealthCheck *HealthCheckSpec

	// Restart policy for the container ("no", "always", "on-failure", "unless-stopped")
	Restart string

	// WorkingDir sets the working directory inside the container
	WorkingDir string

	// User sets the user (UID:GID) to run the container as
	User string

	// Privileged grants extended privileges to the container
	Privileged bool

	// CapAdd lists Linux capabilities to add
	CapAdd []string

	// CapDrop lists Linux capabilities to drop
	CapDrop []string

	// Networks lists networks this service should connect to
	Networks []string

	// NetworkMode sets the network mode ("bridge", "host", "none", or container reference)
	NetworkMode string

	// Hostname sets the container hostname
	Hostname string

	// ExtraHosts adds entries to /etc/hosts
	ExtraHosts []string

	// DNS sets custom DNS servers
	DNS []string

	// DNSSearch sets custom DNS search domains
	DNSSearch []string

	// Tmpfs mounts tmpfs filesystems
	Tmpfs []string

	// ShmSize sets the size of /dev/shm
	ShmSize string

	// StopSignal sets the signal to stop the container
	StopSignal string

	// StopGracePeriod sets the timeout for graceful stop
	StopGracePeriod string

	// SecurityOpt sets security options
	SecurityOpt []string

	// Ulimits sets resource limits
	Ulimits map[string]UlimitSpec

	// Sysctls sets kernel parameters
	Sysctls map[string]string
}

// BuildSpec defines how to build a container image from source.
type BuildSpec struct {
	// Context is the build context path
	Context string

	// Dockerfile is the path to the Dockerfile (relative to context)
	Dockerfile string

	// Args are build-time variables
	Args map[string]string

	// Target specifies a build stage to target in a multi-stage build
	Target string

	// CacheFrom lists images to use as cache sources
	CacheFrom []string

	// Labels are metadata to add to the built image
	Labels map[string]string

	// Network sets the network mode during build
	Network string

	// ShmSize sets the size of /dev/shm during build
	ShmSize string
}

// PortSpec defines a port mapping between host and container.
type PortSpec struct {
	// Target is the container port
	Target int

	// Published is the host port (0 for auto-assign)
	Published int

	// Protocol is "tcp" or "udp"
	Protocol string

	// HostIP binds to a specific host interface
	HostIP string
}

// VolumeMountSpec defines a volume mount for a service.
type VolumeMountSpec struct {
	// Type is "volume", "bind", or "tmpfs"
	Type string

	// Source is the volume name or host path
	Source string

	// Target is the container path
	Target string

	// ReadOnly makes the mount read-only
	ReadOnly bool

	// Bind contains bind-specific options
	Bind *BindOptions

	// Volume contains volume-specific options
	Volume *VolumeOptions

	// Tmpfs contains tmpfs-specific options
	Tmpfs *TmpfsOptions
}

// BindOptions contains options for bind mounts.
type BindOptions struct {
	// Propagation sets mount propagation ("rprivate", "private", "rshared", "shared", "rslave", "slave")
	Propagation string

	// CreateHostPath creates the source path if it doesn't exist
	CreateHostPath bool
}

// VolumeOptions contains options for named volumes.
type VolumeOptions struct {
	// NoCopy disables copying data from container to volume on first mount
	NoCopy bool
}

// TmpfsOptions contains options for tmpfs mounts.
type TmpfsOptions struct {
	// Size sets the size of the tmpfs mount in bytes
	Size int64

	// Mode sets the file mode
	Mode int
}

// NetworkSpec defines a custom network.
type NetworkSpec struct {
	// Name is the network identifier
	Name string

	// Driver is the network driver ("bridge", "overlay", "host", "none")
	Driver string

	// DriverOpts are driver-specific options
	DriverOpts map[string]string

	// IPAM configures IP address management
	IPAM *IPAMSpec

	// Internal makes the network internal (no external connectivity)
	Internal bool

	// Attachable allows manual container attachment
	Attachable bool

	// Labels are metadata key-value pairs
	Labels map[string]string

	// EnableIPv6 enables IPv6 networking
	EnableIPv6 bool
}

// IPAMSpec configures IP address management for a network.
type IPAMSpec struct {
	// Driver is the IPAM driver
	Driver string

	// Config contains IPAM configuration blocks
	Config []IPAMConfig

	// Options are driver-specific options
	Options map[string]string
}

// IPAMConfig defines a single IPAM configuration block.
type IPAMConfig struct {
	// Subnet in CIDR format
	Subnet string

	// IPRange restricts IP allocation to a subset of the subnet
	IPRange string

	// Gateway sets the gateway IP
	Gateway string

	// AuxAddresses reserves IP addresses for special use
	AuxAddresses map[string]string
}

// VolumeSpec defines a named volume.
type VolumeSpec struct {
	// Name is the volume identifier
	Name string

	// Driver is the volume driver
	Driver string

	// DriverOpts are driver-specific options
	DriverOpts map[string]string

	// External indicates the volume is managed outside this environment
	External bool

	// Labels are metadata key-value pairs
	Labels map[string]string
}

// HealthCheckSpec defines how to check if a service is healthy.
type HealthCheckSpec struct {
	// Test is the command to run (e.g., ["CMD", "curl", "-f", "http://localhost"])
	Test []string

	// Interval is the time between checks
	Interval string

	// Timeout is the maximum time for a check to complete
	Timeout string

	// Retries is the number of consecutive failures needed to mark unhealthy
	Retries int

	// StartPeriod is the initialization time before health checks count
	StartPeriod string

	// Disable turns off health checking
	Disable bool
}

// UlimitSpec defines a resource limit.
type UlimitSpec struct {
	// Soft is the soft limit
	Soft int64

	// Hard is the hard limit
	Hard int64
}
