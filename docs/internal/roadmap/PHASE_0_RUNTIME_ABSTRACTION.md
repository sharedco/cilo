# Phase 0: Runtime Abstraction

**Duration:** ½ day  
**Dependencies:** None  
**Goal:** Extract runtime operations behind a clean provider interface

---

## Objectives

1. Create `pkg/runtime` package with provider interface
2. Move Docker-specific code to `pkg/runtime/docker`
3. Update core code to use provider interface
4. **No behavior changes** - pure refactor for future extensibility

---

## Success Criteria

- [ ] All Docker operations go through `runtime.Provider` interface
- [ ] Zero functional changes (all existing tests pass)
- [ ] Code is ready for Podman provider in Phase 2
- [ ] Clear separation: core logic vs runtime-specific logic

---

## Detailed Tasks

### 1. Create Provider Interface

**File:** `pkg/runtime/provider.go`

```go
package runtime

import (
    "context"
    "io"
    "time"
    
    "github.com/cilo/cilo/pkg/models"
)

// Provider abstracts container runtime operations
type Provider interface {
    // Lifecycle
    Create(ctx context.Context, env *models.Environment, opts CreateOptions) error
    Up(ctx context.Context, env *models.Environment, opts UpOptions) error
    Down(ctx context.Context, env *models.Environment) error
    Destroy(ctx context.Context, env *models.Environment) error
    
    // Introspection
    Status(ctx context.Context, env *models.Environment) (*Status, error)
    Services(ctx context.Context, env *models.Environment) ([]*models.Service, error)
    Logs(ctx context.Context, env *models.Environment, service string, opts LogOptions) (io.ReadCloser, error)
    
    // Execution
    Exec(ctx context.Context, env *models.Environment, service, command string, opts ExecOptions) error
    
    // Network management
    AllocateNetwork(ctx context.Context, subnet string) (*Network, error)
    ReleaseNetwork(ctx context.Context, network *Network) error
    
    // Validation
    Validate(ctx context.Context, composePath string) error
}

// CreateOptions for environment creation
type CreateOptions struct {
    CopyFiles bool
}

// UpOptions for starting environments
type UpOptions struct {
    Build    bool
    Recreate bool
}

// ExecOptions for executing commands
type ExecOptions struct {
    Interactive bool
    TTY         bool
    Env         map[string]string
}

// LogOptions for retrieving logs
type LogOptions struct {
    Follow bool
    Tail   int
    Since  time.Time
}

// Status represents environment status
type Status struct {
    State       EnvironmentState
    Services    []*models.Service
    LastUpdated time.Time
}

// EnvironmentState enum
type EnvironmentState string

const (
    StateCreated EnvironmentState = "created"
    StateRunning EnvironmentState = "running"
    StateStopped EnvironmentState = "stopped"
    StateError   EnvironmentState = "error"
)

// Network represents a container network
type Network struct {
    ID     string
    Name   string
    Subnet string
    Driver string
}
```

**Rationale:**
- Minimal interface - only what varies by runtime
- Context for cancellation/timeout
- Returns structured data, not raw output
- Enables testing via mock providers

---

### 2. Implement Docker Provider

**File:** `pkg/runtime/docker/provider.go`

```go
package docker

import (
    "context"
    "fmt"
    "os/exec"
    "path/filepath"
    
    "github.com/cilo/cilo/pkg/models"
    "github.com/cilo/cilo/pkg/runtime"
)

// Provider implements runtime.Provider for Docker
type Provider struct {
    // Future: support remote hosts
    host *models.Host
}

// NewProvider creates a Docker provider
func NewProvider() *Provider {
    return &Provider{}
}

// Up starts an environment
func (p *Provider) Up(ctx context.Context, env *models.Environment, opts runtime.UpOptions) error {
    workspace := getWorkspace(env)
    composePath := filepath.Join(workspace, "docker-compose.yml")
    
    args := []string{
        "compose",
        "-f", composePath,
        "-p", fmt.Sprintf("cilo_%s", env.Name),
        "up", "-d",
    }
    
    if opts.Build {
        args = append(args, "--build")
    }
    
    if opts.Recreate {
        args = append(args, "--force-recreate")
    }
    
    cmd := exec.CommandContext(ctx, "docker", args...)
    cmd.Dir = workspace
    
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("docker compose up failed: %w\nOutput: %s", err, output)
    }
    
    return nil
}

// Down stops an environment
func (p *Provider) Down(ctx context.Context, env *models.Environment) error {
    workspace := getWorkspace(env)
    
    cmd := exec.CommandContext(ctx, "docker", "compose",
        "-f", filepath.Join(workspace, "docker-compose.yml"),
        "-p", fmt.Sprintf("cilo_%s", env.Name),
        "down",
    )
    cmd.Dir = workspace
    
    return cmd.Run()
}

// Status gets current environment status
func (p *Provider) Status(ctx context.Context, env *models.Environment) (*runtime.Status, error) {
    workspace := getWorkspace(env)
    
    // Use docker compose ps --format json
    cmd := exec.CommandContext(ctx, "docker", "compose",
        "-f", filepath.Join(workspace, "docker-compose.yml"),
        "-p", fmt.Sprintf("cilo_%s", env.Name),
        "ps", "--format", "json",
    )
    cmd.Dir = workspace
    
    output, err := cmd.Output()
    if err != nil {
        return &runtime.Status{State: runtime.StateError}, nil
    }
    
    // Parse JSON to determine state
    // If any container running -> StateRunning
    // If all exited -> StateStopped
    // If none exist -> StateCreated
    
    return parseComposeStatus(output)
}

// Services returns list of services with current IPs
func (p *Provider) Services(ctx context.Context, env *models.Environment) ([]*models.Service, error) {
    // Use docker compose ps and docker inspect to get actual IPs
    workspace := getWorkspace(env)
    
    cmd := exec.CommandContext(ctx, "docker", "compose",
        "-f", filepath.Join(workspace, "docker-compose.yml"),
        "-p", fmt.Sprintf("cilo_%s", env.Name),
        "ps", "--format", "json",
    )
    cmd.Dir = workspace
    
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    return parseServices(output, env)
}

// Destroy removes all resources
func (p *Provider) Destroy(ctx context.Context, env *models.Environment) error {
    workspace := getWorkspace(env)
    
    // docker compose down -v (remove volumes too)
    cmd := exec.CommandContext(ctx, "docker", "compose",
        "-f", filepath.Join(workspace, "docker-compose.yml"),
        "-p", fmt.Sprintf("cilo_%s", env.Name),
        "down", "-v", "--remove-orphans",
    )
    cmd.Dir = workspace
    
    return cmd.Run()
}

// Validate checks if compose file is valid
func (p *Provider) Validate(ctx context.Context, composePath string) error {
    cmd := exec.CommandContext(ctx, "docker", "compose",
        "-f", composePath,
        "config", "--quiet",
    )
    
    return cmd.Run()
}

// Helper functions
func getWorkspace(env *models.Environment) string {
    // Move this from state package to here or make it configurable
    return filepath.Join(os.ExpandEnv("$HOME/.cilo/envs"), env.Project, env.Name)
}

func parseComposeStatus(jsonOutput []byte) (*runtime.Status, error) {
    // Parse docker compose ps JSON output
    // Determine overall state
    // Return Status struct
}

func parseServices(jsonOutput []byte, env *models.Environment) ([]*models.Service, error) {
    // Parse docker compose ps JSON
    // For each container, docker inspect to get IP
    // Build Service objects
}
```

---

### 3. Move Docker Code from `pkg/docker`

**Current:** `pkg/docker/docker.go` has functions like:
- `Up(env, build, recreate)`
- `Down(env)`
- `Destroy(env)`

**New:** These become methods on `DockerProvider`

**Migration:**
1. Copy function bodies to provider methods
2. Adjust signatures to match interface
3. Update error handling to be consistent
4. Delete old `pkg/docker` package

---

### 4. Update Core Code to Use Provider

**File:** `cmd/lifecycle.go`

**Before:**
```go
import "github.com/cilo/cilo/pkg/docker"

// In upCmd.RunE
if err := docker.Up(env, build, recreate); err != nil {
    return err
}
```

**After:**
```go
import "github.com/cilo/cilo/pkg/runtime/docker"

// Get provider for environment
provider := docker.NewProvider()

// Use provider interface
if err := provider.Up(ctx, env, runtime.UpOptions{
    Build: build,
    Recreate: recreate,
}); err != nil {
    return err
}
```

**Files to Update:**
- `cmd/lifecycle.go` - create, up, down, destroy
- `cmd/commands.go` - logs, exec
- `pkg/compose/compose.go` - validation

---

### 5. Add Provider Factory (Future-Proofing)

**File:** `pkg/runtime/factory.go`

```go
package runtime

import (
    "fmt"
    
    "github.com/cilo/cilo/pkg/models"
    "github.com/cilo/cilo/pkg/runtime/docker"
)

// ProviderType identifies a runtime provider
type ProviderType string

const (
    Docker ProviderType = "docker"
    Podman ProviderType = "podman"  // Future
)

// NewProvider creates a provider for the given type
func NewProvider(providerType ProviderType, host *models.Host) (Provider, error) {
    switch providerType {
    case Docker:
        return docker.NewProvider(host), nil
    case Podman:
        return nil, fmt.Errorf("podman provider not yet implemented")
    default:
        return nil, fmt.Errorf("unknown provider type: %s", providerType)
    }
}

// DetectProvider auto-detects available runtime
func DetectProvider() ProviderType {
    // Check for docker
    if _, err := exec.LookPath("docker"); err == nil {
        return Docker
    }
    
    // Check for podman
    if _, err := exec.LookPath("podman"); err == nil {
        return Podman
    }
    
    return Docker  // Default
}
```

**Usage in CLI:**
```go
// In cmd/root.go or lifecycle.go
providerType := runtime.DetectProvider()
provider, err := runtime.NewProvider(providerType, nil)  // nil = local host
```

---

## File Structure Changes

### Before
```
cilo/
  pkg/
    docker/
      docker.go          # Docker-specific operations
    compose/
      compose.go         # Compose transformation
    dns/
      dns.go             # DNS management
    state/
      state.go           # State operations
    models/
      models.go          # Data structures
```

### After
```
cilo/
  pkg/
    runtime/
      provider.go        # Interface definition
      factory.go         # Provider factory
      types.go           # Shared types (Status, Options, etc.)
      docker/
        provider.go      # Docker implementation
        compose.go       # Docker-specific compose handling
        network.go       # Docker network operations
    compose/             # Generic compose utilities
      validate.go
    dns/
      dns.go
    state/
      state.go
    models/
      models.go
```

---

## Testing Plan

### Unit Tests

**Test:** `pkg/runtime/docker/provider_test.go`

```go
func TestDockerProvider_Up(t *testing.T) {
    // Create test environment
    env := &models.Environment{
        Name: "test",
        Project: "testproj",
    }
    
    // Create test workspace
    workspace := setupTestWorkspace(t, env)
    defer cleanupTestWorkspace(t, workspace)
    
    // Create simple compose file
    writeComposeFile(t, workspace, `
version: '3'
services:
  nginx:
    image: nginx:alpine
`)
    
    provider := docker.NewProvider()
    
    ctx := context.Background()
    err := provider.Up(ctx, env, runtime.UpOptions{})
    
    if err != nil {
        t.Fatalf("Up failed: %v", err)
    }
    
    // Verify containers are running
    status, err := provider.Status(ctx, env)
    if err != nil {
        t.Fatalf("Status failed: %v", err)
    }
    
    if status.State != runtime.StateRunning {
        t.Errorf("Expected StateRunning, got %v", status.State)
    }
    
    // Cleanup
    provider.Down(ctx, env)
}
```

### Integration Tests

**Test:** Ensure existing integration tests still pass

```bash
# Run existing tests
go test ./...

# Should have same results as before refactor
```

---

## Migration Checklist

- [ ] Create `pkg/runtime/provider.go` with interface
- [ ] Create `pkg/runtime/types.go` with shared types
- [ ] Implement `pkg/runtime/docker/provider.go`
- [ ] Move docker operations from `pkg/docker` to provider
- [ ] Create `pkg/runtime/factory.go`
- [ ] Update `cmd/lifecycle.go` to use provider
- [ ] Update `cmd/commands.go` to use provider
- [ ] Update `pkg/compose` to use provider for validation
- [ ] Delete old `pkg/docker` package
- [ ] Run all existing tests - ensure they pass
- [ ] Add unit tests for Docker provider
- [ ] Update documentation

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing functionality | Run full test suite before/after, no behavior changes |
| Interface too narrow | Based on actual operations used, can expand later |
| Interface too wide | Kept minimal, only runtime-varying operations |
| Performance regression | Direct exec calls, same as before |

---

## Deliverables

1. **Code:**
   - `pkg/runtime/` package with interface and Docker implementation
   - Updated `cmd/` to use provider interface
   - Deleted `pkg/docker/` package

2. **Tests:**
   - Unit tests for Docker provider
   - All existing tests passing

3. **Documentation:**
   - Code comments explaining provider interface
   - Updated internal docs

---

## Next Steps

After Phase 0 completion:
- **Phase 1:** Use provider interface for improved compose handling, reconciliation
- **Phase 2B:** Add remote host support to provider
- **Future:** Add Podman provider (~1 day work, trivial after this foundation)

---

## Review Criteria

Before considering Phase 0 complete:

1. **No functional regressions:** All existing CLI commands work identically
2. **Clean abstraction:** No Docker-specific code outside `pkg/runtime/docker`
3. **Tests pass:** 100% of existing tests still pass
4. **Ready for extension:** Adding Podman provider requires only implementing interface, no core changes
5. **Code quality:** Clear separation of concerns, good error messages

---

## Estimated Timeline

- Interface design: 1 hour
- Docker provider implementation: 2 hours
- Update core code: 1 hour
- Testing and fixes: 1 hour
- Documentation: 30 minutes

**Total: ~5 hours** (fits in ½ day)
