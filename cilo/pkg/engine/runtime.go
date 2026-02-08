package engine

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Runtime executes environments on a container runtime (Docker, Podman, etc.).
// It consumes EnvironmentSpec and manages the lifecycle of containers, networks, and volumes.
type Runtime interface {
	// Name returns the runtime identifier (e.g., "docker", "podman")
	Name() string

	// Up starts all services in the environment.
	// Creates networks, volumes, and containers as needed.
	Up(ctx context.Context, spec *EnvironmentSpec, opts UpOptions) error

	// Down stops all services in the environment.
	// Containers are stopped but not removed.
	Down(ctx context.Context, spec *EnvironmentSpec) error

	// Destroy removes all resources for the environment.
	// Stops and removes containers, networks, and volumes.
	Destroy(ctx context.Context, spec *EnvironmentSpec) error

	// Status returns the current status of all services.
	Status(ctx context.Context, spec *EnvironmentSpec) ([]ServiceStatus, error)

	// Logs retrieves logs from a specific service.
	// Returns a ReadCloser that streams logs. Caller must close it.
	Logs(ctx context.Context, spec *EnvironmentSpec, service string, opts LogOptions) (io.ReadCloser, error)

	// Exec executes a command in a running service container.
	Exec(ctx context.Context, spec *EnvironmentSpec, service string, cmd []string, opts ExecOptions) error

	// CreateNetwork creates an isolated network for the environment.
	// The subnet parameter specifies the CIDR block (e.g., "10.224.1.0/24").
	CreateNetwork(ctx context.Context, spec *EnvironmentSpec, subnet string) error

	// RemoveNetwork removes the environment's network.
	RemoveNetwork(ctx context.Context, spec *EnvironmentSpec) error

	// GetServiceIPs returns the IP addresses of all services.
	// Returns a map of service name to IP address.
	GetServiceIPs(ctx context.Context, spec *EnvironmentSpec) (map[string]string, error)
}

// UpOptions configures how services are started.
type UpOptions struct {
	// Build forces rebuilding images before starting
	Build bool

	// Recreate forces recreating containers even if config hasn't changed
	Recreate bool

	// ForceRecreate forces recreating containers even if they're running
	ForceRecreate bool

	// NoDeps doesn't start linked services
	NoDeps bool

	// Timeout for container shutdown during restart (seconds)
	Timeout int

	// RemoveOrphans removes containers for services not in the spec
	RemoveOrphans bool

	// Detach runs containers in the background
	Detach bool

	// QuietPull suppresses pull output
	QuietPull bool
}

// LogOptions configures log retrieval.
type LogOptions struct {
	// Follow streams logs continuously
	Follow bool

	// Tail limits the number of lines from the end of logs (0 = all)
	Tail int

	// Since shows logs since timestamp
	Since time.Time

	// Until shows logs before timestamp
	Until time.Time

	// Timestamps includes timestamps in output
	Timestamps bool

	// Stdout writes stdout logs to this writer
	Stdout io.Writer

	// Stderr writes stderr logs to this writer
	Stderr io.Writer
}

// ExecOptions configures command execution.
type ExecOptions struct {
	// Interactive keeps stdin open
	Interactive bool

	// TTY allocates a pseudo-TTY
	TTY bool

	// Detach runs command in background
	Detach bool

	// User runs command as specific user (UID:GID)
	User string

	// WorkingDir sets the working directory
	WorkingDir string

	// Env sets environment variables
	Env map[string]string

	// Privileged gives extended privileges
	Privileged bool

	// Stdin provides input to the command
	Stdin io.Reader

	// Stdout captures command output
	Stdout io.Writer

	// Stderr captures command errors
	Stderr io.Writer
}

// ServiceStatus represents the current state of a service.
type ServiceStatus struct {
	// Name is the service identifier
	Name string

	// State is the container state ("running", "stopped", "exited", "paused", etc.)
	State string

	// Status is a human-readable status message
	Status string

	// Health is the health check status ("healthy", "unhealthy", "starting", "none")
	Health string

	// IP is the service's IP address in the environment network
	IP string

	// Container is the container name or ID
	Container string

	// Ports lists exposed port mappings
	Ports []PortMapping

	// CreatedAt is when the container was created
	CreatedAt time.Time

	// StartedAt is when the container started
	StartedAt time.Time

	// FinishedAt is when the container stopped
	FinishedAt time.Time

	// ExitCode is the container exit code (if stopped)
	ExitCode int

	// Error contains error message if state is "error"
	Error string
}

// PortMapping represents a published port.
type PortMapping struct {
	// ContainerPort is the port inside the container
	ContainerPort int

	// HostPort is the port on the host
	HostPort int

	// HostIP is the host interface the port is bound to
	HostIP string

	// Protocol is "tcp" or "udp"
	Protocol string
}

// RuntimeRegistry manages available container runtimes.
type RuntimeRegistry struct {
	mu       sync.RWMutex
	runtimes map[string]Runtime
	default_ string
}

// NewRuntimeRegistry creates a new runtime registry.
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		runtimes: make(map[string]Runtime),
	}
}

// Register adds a runtime to the registry.
func (r *RuntimeRegistry) Register(runtime Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := runtime.Name()
	r.runtimes[name] = runtime

	// First registered runtime becomes default
	if r.default_ == "" {
		r.default_ = name
	}
}

// Get retrieves a runtime by name.
// Returns nil if the runtime is not registered.
func (r *RuntimeRegistry) Get(name string) Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runtimes[name]
}

// Default returns the default runtime.
// Returns nil if no runtimes are registered.
func (r *RuntimeRegistry) Default() Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.default_ == "" {
		return nil
	}
	return r.runtimes[r.default_]
}

// SetDefault sets the default runtime by name.
// Returns an error if the runtime is not registered.
func (r *RuntimeRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runtimes[name]; !exists {
		return fmt.Errorf("runtime %s is not registered", name)
	}

	r.default_ = name
	return nil
}

// List returns the names of all registered runtimes.
func (r *RuntimeRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.runtimes))
	for name := range r.runtimes {
		names = append(names, name)
	}
	return names
}

// DefaultRuntimeRegistry is the global runtime registry.
var DefaultRuntimeRegistry = NewRuntimeRegistry()

// RegisterRuntime adds a runtime to the default registry.
func RegisterRuntime(runtime Runtime) {
	DefaultRuntimeRegistry.Register(runtime)
}

// GetRuntime retrieves a runtime by name from the default registry.
func GetRuntime(name string) Runtime {
	return DefaultRuntimeRegistry.Get(name)
}

// DefaultRuntime returns the default runtime from the default registry.
func DefaultRuntime() Runtime {
	return DefaultRuntimeRegistry.Default()
}

// SetDefaultRuntime sets the default runtime in the default registry.
func SetDefaultRuntime(name string) error {
	return DefaultRuntimeRegistry.SetDefault(name)
}

// ListRuntimes returns all runtime names from the default registry.
func ListRuntimes() []string {
	return DefaultRuntimeRegistry.List()
}
