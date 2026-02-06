# Phase 1: Foundations

**Status:** ✅ **COMPLETE** (v0.2.0)  
**Duration:** 2-3 days  
**Dependencies:** Phase 0 (Runtime Abstraction) ✅  
**Goal:** Fix critical reliability and correctness issues

## ✅ What Was Delivered

### State Correctness
- **File locking:** `github.com/gofrs/flock` for cross-platform exclusive locks
- **Atomic writes:** Temp file + rename pattern prevents corruption
- **WithLock() wrapper:** All mutations (Create, Update, Delete) use consistent locking

### DNS Rendering Model  
- **Full render from state:** Complete dnsmasq config regenerated on every change (no fragile marker editing)
- **System resolver detection:** Linux (systemd-resolved → /etc/resolv.conf), macOS (scutil --dns)
- **SIGHUP graceful reload:** No service interruption during config updates
- **Atomic DNS writes:** Temp file + rename for crash safety

### Collision Detection
- **Docker network inspection:** Queries existing networks before subnet allocation
- **Automatic retry:** Tries next subnet on collision (10.224.x.0/24)
- **Error on exhaustion:** Clear error if no non-conflicting subnet found

### Reconciliation & Doctor
- **cilo doctor:** Health checks for Docker, dnsmasq, state sync
- **cilo doctor --fix:** Reconciles state with runtime, regenerates DNS
- **reconcile package:** Runtime as source of truth for environment status
- **Orphan detection:** Finds Docker networks with cilo=true label not in state

---

## Objectives

1. Make state operations concurrent-safe and atomic
2. Switch from deep compose rewriting to override model
3. Render DNS configs from state (no text editing)
4. Implement reconciliation system
5. Add integration test suite
6. Implement collision detection and safeguards

---

## ✅ Success Criteria (MET)

- [x] Concurrent-safe state operations (file locking + atomic writes)
- [x] DNS updates are atomic and crash-safe (full render + temp/rename)
- [x] Reconciliation system implemented (`cilo doctor --fix`)
- [x] Subnet collision detection with Docker networks
- [x] Graceful DNS reload (SIGHUP) without service interruption
- [ ] 100 concurrent `cilo create` operations (load tested) - framework ready
- [ ] Integration test suite (test infrastructure in place)
- [ ] Works with complex compose files (extends, profiles) - partially done

---

## Detailed Tasks

### Task 1: Concurrent-Safe State Operations (Day 1, 4 hours)

#### 1.1 Add File Locking

**File:** `pkg/state/lock.go`

```go
package state

import (
    "github.com/gofrs/flock"
    "time"
)

const lockTimeout = 30 * time.Second

type LockedState struct {
    state    *State
    lock     *flock.Flock
    filePath string
}

// WithLock executes fn with exclusive state lock
func WithLock(fn func(*State) error) error {
    statePath := getStatePath()
    lock := flock.New(statePath + ".lock")
    
    // Try to acquire lock with timeout
    ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
    defer cancel()
    
    locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
    if err != nil {
        return fmt.Errorf("failed to acquire state lock: %w", err)
    }
    if !locked {
        return fmt.Errorf("state lock timeout after %v", lockTimeout)
    }
    defer lock.Unlock()
    
    // Load current state
    state, err := loadStateUnsafe()
    if err != nil {
        return err
    }
    
    // Apply migration if needed
    if err := ApplyMigrations(state); err != nil {
        return err
    }
    
    // Execute mutation
    if err := fn(state); err != nil {
        return err
    }
    
    // Atomic write
    return atomicWriteState(state)
}
```

#### 1.2 Atomic State Writes

**File:** `pkg/state/state.go`

```go
func atomicWriteState(state *State) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal state: %w", err)
    }
    
    statePath := getStatePath()
    tmpPath := statePath + ".tmp"
    
    // Write to temp file
    if err := os.WriteFile(tmpPath, data, 0600); err != nil {
        return fmt.Errorf("failed to write temp state: %w", err)
    }
    
    // Atomic rename
    if err := os.Rename(tmpPath, statePath); err != nil {
        // Cleanup temp file on failure
        os.Remove(tmpPath)
        return fmt.Errorf("failed to rename state file: %w", err)
    }
    
    return nil
}

// Update all state mutation functions to use WithLock
func CreateEnvironment(name, source, project string) (*Environment, error) {
    var env *Environment
    
    err := WithLock(func(state *State) error {
        // Check if exists
        key := makeEnvKey(project, name)
        if _, exists := state.Environments[key]; exists {
            return fmt.Errorf("environment %q already exists", key)
        }
        
        // Validate name
        if err := validateName(name); err != nil {
            return err
        }
        
        // Allocate subnet
        state.SubnetCounter++
        subnet := fmt.Sprintf("%s%d.0/24", baseSubnet, state.SubnetCounter)
        
        // Create environment
        env = &Environment{
            Name:      name,
            Project:   project,
            CreatedAt: time.Now(),
            Subnet:    subnet,
            Status:    "created",
            Source:    source,
            Services:  make(map[string]*Service),
        }
        
        state.Environments[key] = env
        return nil
    })
    
    return env, err
}
```

#### 1.3 State Schema Migration

**File:** `pkg/state/migrations.go`

```go
package state

type Migration interface {
    Version() int
    Up(*State) error
    Down(*State) error
}

var migrations = []Migration{
    &MigrateV1ToV2{},
}

type MigrateV1ToV2 struct{}

func (m *MigrateV1ToV2) Version() int { return 2 }

func (m *MigrateV1ToV2) Up(state *State) error {
    // V1: environments are flat map
    // V2: environments are nested under hosts
    
    if state.Version == 2 {
        return nil // Already migrated
    }
    
    // Create hosts structure
    localHost := &Host{
        ID:           "local",
        Provider:     "docker",
        Environments: make(map[string]*Environment),
    }
    
    // Move all environments to local host
    for key, env := range state.Environments {
        localHost.Environments[key] = env
    }
    
    // Replace flat environments with hosts structure
    state.Hosts = map[string]*Host{
        "local": localHost,
    }
    state.Environments = nil  // Clear old structure
    state.Version = 2
    
    return nil
}

func (m *MigrateV1ToV2) Down(state *State) error {
    // Flatten back to v1
    state.Environments = make(map[string]*Environment)
    
    for _, host := range state.Hosts {
        for key, env := range host.Environments {
            state.Environments[key] = env
        }
    }
    
    state.Hosts = nil
    state.Version = 1
    return nil
}

func ApplyMigrations(state *State) error {
    for _, migration := range migrations {
        if state.Version < migration.Version() {
            if err := migration.Up(state); err != nil {
                return fmt.Errorf("migration to v%d failed: %w", migration.Version(), err)
            }
            state.Version = migration.Version()
        }
    }
    return nil
}
```

**Dependencies:** `go get github.com/gofrs/flock`

---

### Task 2: Compose Override Model (Day 1-2, 6 hours)

#### 2.1 Override Generation

**File:** `pkg/compose/override.go`

```go
package compose

import (
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/cilo/cilo/pkg/models"
    "gopkg.in/yaml.v3"
)

// GenerateOverride creates a minimal compose override file
func GenerateOverride(env *models.Environment, outputPath string) error {
    override := models.ComposeFile{
        Services: make(map[string]*models.ComposeService),
        Networks: make(map[string]*models.ComposeNetwork),
    }
    
    networkName := fmt.Sprintf("cilo-%s", env.Name)
    
    // For each service in environment state
    for name, svc := range env.Services {
        override.Services[name] = &models.ComposeService{
            ContainerName: svc.Container,
            Ports:         []string{},  // Disable all ports
            Networks: map[string]interface{}{
                networkName: map[string]string{
                    "ipv4_address": svc.IP,
                },
            },
            Labels: map[string]string{
                "cilo.environment": env.Name,
                "cilo.project":     env.Project,
                "cilo.service":     name,
            },
        }
    }
    
    // Add network definition
    override.Networks[networkName] = &models.ComposeNetwork{
        Name:   networkName,
        Driver: "bridge",
        IPAM: &models.ComposeIPAM{
            Config: []models.ComposeIPAMConfig{
                {Subnet: env.Subnet},
            },
        },
    }
    
    // Add shared networks if any
    for _, sharedNet := range env.SharedNetworks {
        override.Networks[sharedNet] = &models.ComposeNetwork{
            External: true,
            Name:     sharedNet,
        }
        
        // All services join shared network
        for name := range override.Services {
            svcNetworks := override.Services[name].Networks.(map[string]interface{})
            svcNetworks[sharedNet] = struct{}{}
        }
    }
    
    // Marshal to YAML
    data, err := yaml.Marshal(&override)
    if err != nil {
        return fmt.Errorf("failed to marshal override: %w", err)
    }
    
    // Add header comment
    header := `# Auto-generated by cilo
# DO NOT EDIT - this file is regenerated on each 'cilo up'
# Edit the original docker-compose.yml in the parent directory

`
    
    // Write file
    if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
        return err
    }
    
    return os.WriteFile(outputPath, []byte(header+string(data)), 0644)
}
```

#### 2.2 Update Docker Provider to Use Override

**File:** `pkg/runtime/docker/provider.go`

```go
func (p *Provider) Up(ctx context.Context, env *models.Environment, opts runtime.UpOptions) error {
    workspace := getWorkspace(env)
    composePath := filepath.Join(workspace, "docker-compose.yml")
    overridePath := filepath.Join(workspace, ".cilo/override.yml")
    
    // Generate override file
    if err := compose.GenerateOverride(env, overridePath); err != nil {
        return fmt.Errorf("failed to generate override: %w", err)
    }
    
    // Build docker compose command with both files
    args := []string{
        "compose",
        "-f", composePath,
        "-f", overridePath,
        "-p", fmt.Sprintf("cilo_%s", env.Name),
    }
    
    if opts.Build {
        args = append(args, "--build")
    }
    
    args = append(args, "up", "-d")
    
    if opts.Recreate {
        args = append(args, "--force-recreate")
    }
    
    cmd := exec.CommandContext(ctx, "docker", args...)
    cmd.Dir = workspace
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("docker compose up failed: %w\nOutput: %s", err, output)
    }
    
    return nil
}
```

#### 2.3 Service Discovery from Compose

**File:** `pkg/compose/discover.go`

```go
// DiscoverServices reads compose file and returns service definitions
func DiscoverServices(composePath string) (map[string]*ServiceConfig, error) {
    data, err := os.ReadFile(composePath)
    if err != nil {
        return nil, err
    }
    
    var composeFile models.ComposeFile
    if err := yaml.Unmarshal(data, &composeFile); err != nil {
        return nil, err
    }
    
    services := make(map[string]*ServiceConfig)
    
    for name, svc := range composeFile.Services {
        config := &ServiceConfig{
            Name: name,
        }
        
        // Check for ingress markers
        if svc.Labels != nil {
            if ingressLabel, ok := svc.Labels["cilo.ingress"]; ok && ingressLabel == "true" {
                config.IsIngress = true
            }
            
            if hostnamesLabel, ok := svc.Labels["cilo.hostnames"]; ok {
                config.Hostnames = strings.Split(hostnamesLabel, ",")
                for i := range config.Hostnames {
                    config.Hostnames[i] = strings.TrimSpace(config.Hostnames[i])
                }
            }
        }
        
        // Auto-detect ingress
        if !config.IsIngress && isIngressServiceName(name) {
            config.IsIngress = true
        }
        
        services[name] = config
    }
    
    return services, nil
}

func isIngressServiceName(name string) bool {
    ingressNames := []string{"nginx", "web", "app", "frontend", "traefik", "caddy"}
    for _, n := range ingressNames {
        if name == n {
            return true
        }
    }
    return false
}
```

---

### Task 3: Rendered DNS Configs (Day 2, 3 hours)

#### 3.1 DNS Rendering from State

**File:** `pkg/dns/render.go`

```go
package dns

import (
    "fmt"
    "strings"
    
    "github.com/cilo/cilo/pkg/models"
)

// RenderConfig generates complete dnsmasq config from state
func RenderConfig(state *models.State) (string, error) {
    var b strings.Builder
    
    // Header
    b.WriteString("# Auto-generated by cilo\n")
    b.WriteString("# DO NOT EDIT MANUALLY\n")
    b.WriteString(fmt.Sprintf("# Generated: %s\n\n", time.Now().Format(time.RFC3339)))
    
    // Base config
    b.WriteString(fmt.Sprintf("port=%d\n", dnsPort))
    b.WriteString("bind-interfaces\n")
    b.WriteString("listen-address=127.0.0.1\n")
    
    // Forward non-.test queries to system resolver (not hardcoded upstream)
    // This preserves VPN/corporate DNS functionality
    upstreams := getSystemUpstreams()
    for _, upstream := range upstreams {
        b.WriteString(fmt.Sprintf("server=%s\n", upstream))
    }
    b.WriteString("\n")
    
    // For each host
    for hostID, host := range state.Hosts {
        b.WriteString(fmt.Sprintf("# Host: %s\n", hostID))
        
        // For each environment
        for envKey, env := range host.Environments {
            b.WriteString(fmt.Sprintf("# Environment: %s\n", envKey))
            
            // Service DNS entries
            for _, svc := range env.Services {
                // service.env.test -> service IP
                b.WriteString(fmt.Sprintf(
                    "address=/%s.%s.test/%s\n",
                    svc.Name, env.Name, svc.IP,
                ))
                
                // Custom hostnames
                for _, hostname := range svc.Hostnames {
                    b.WriteString(fmt.Sprintf(
                        "address=/%s.%s.test/%s\n",
                        hostname, env.Name, svc.IP,
                    ))
                }
            }
            
            // Project wildcard (if project set and has ingress)
            if env.Project != "" {
                if ingress := getIngressService(env); ingress != nil {
                    // Wildcard: *.project.env.test -> ingress IP
                    b.WriteString(fmt.Sprintf(
                        "address=/.%s.%s.test/%s\n",
                        env.Project, env.Name, ingress.IP,
                    ))
                    // Apex: project.env.test -> ingress IP
                    b.WriteString(fmt.Sprintf(
                        "address=/%s.%s.test/%s\n",
                        env.Project, env.Name, ingress.IP,
                    ))
                }
            }
            
            b.WriteString("\n")
        }
    }
    
    return b.String(), nil
}

func getIngressService(env *models.Environment) *models.Service {
    for _, svc := range env.Services {
        if svc.IsIngress {
            return svc
        }
    }
    return nil
}

// getSystemUpstreams returns the system's DNS servers to forward non-.test queries
func getSystemUpstreams() []string {
    // Linux: parse from systemd-resolved or /etc/resolv.conf
    // macOS: use scutil --dns or /etc/resolv.conf
    
    // Try systemd-resolved first (Linux)
    if runtime.GOOS == "linux" {
        if servers := getSystemdResolvedUpstreams(); len(servers) > 0 {
            return servers
        }
    }
    
    // Fall back to /etc/resolv.conf
    if servers := parseResolvConf("/etc/resolv.conf"); len(servers) > 0 {
        return servers
    }
    
    // Last resort: public DNS (but warn in logs)
    log.Println("Warning: could not detect system DNS, falling back to 8.8.8.8")
    return []string{"8.8.8.8", "8.8.4.4"}
}

func getSystemdResolvedUpstreams() []string {
    // resolvectl status | grep "DNS Servers"
    cmd := exec.Command("resolvectl", "status")
    output, err := cmd.Output()
    if err != nil {
        return nil
    }
    
    var servers []string
    for _, line := range strings.Split(string(output), "\n") {
        if strings.Contains(line, "DNS Servers:") {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 {
                for _, s := range strings.Fields(parts[1]) {
                    if net.ParseIP(s) != nil {
                        servers = append(servers, s)
                    }
                }
            }
        }
    }
    return servers
}

func parseResolvConf(path string) []string {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil
    }
    
    var servers []string
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "nameserver ") {
            server := strings.TrimPrefix(line, "nameserver ")
            if net.ParseIP(server) != nil && server != "127.0.0.53" {
                // Skip 127.0.0.53 (systemd-resolved stub)
                servers = append(servers, server)
            }
        }
    }
    return servers
}
```

#### 3.2 Atomic DNS Updates

**File:** `pkg/dns/dns.go`

```go
// UpdateDNS regenerates DNS config from state and reloads
func UpdateDNS(state *models.State) error {
    // Render config
    config, err := RenderConfig(state)
    if err != nil {
        return fmt.Errorf("failed to render DNS config: %w", err)
    }
    
    // Validate
    if err := ValidateConfig(config); err != nil {
        return fmt.Errorf("invalid DNS config: %w", err)
    }
    
    // Atomic write
    configPath := getDNSConfigPath()
    tmpPath := configPath + ".tmp"
    
    if err := os.WriteFile(tmpPath, []byte(config), 0644); err != nil {
        return err
    }
    
    if err := os.Rename(tmpPath, configPath); err != nil {
        os.Remove(tmpPath)
        return err
    }
    
    // Graceful reload
    return reloadDNSMasq()
}

func ValidateConfig(config string) error {
    // Check for duplicate addresses
    lines := strings.Split(config, "\n")
    addresses := make(map[string]string)
    
    for _, line := range lines {
        if strings.HasPrefix(line, "address=") {
            parts := strings.SplitN(line, "/", 3)
            if len(parts) == 3 {
                domain := parts[1]
                ip := parts[2]
                
                if existingIP, exists := addresses[domain]; exists && existingIP != ip {
                    return fmt.Errorf("duplicate address for %s: %s and %s", domain, existingIP, ip)
                }
                addresses[domain] = ip
            }
        }
    }
    
    return nil
}

func reloadDNSMasq() error {
    pidPath := filepath.Join(getDNSDir(), dnsPidFile)
    
    data, err := os.ReadFile(pidPath)
    if err != nil {
        // Not running, start it
        return startDNS()
    }
    
    pid := strings.TrimSpace(string(data))
    pidInt, err := strconv.Atoi(pid)
    if err != nil {
        return startDNS()
    }
    
    // Send SIGHUP for graceful reload (not SIGTERM!)
    process, err := os.FindProcess(pidInt)
    if err != nil {
        return startDNS()
    }
    
    // SIGHUP tells dnsmasq to reload config
    return process.Signal(syscall.SIGHUP)
}
```

---

### Task 4: Reconciliation System (Day 2-3, 4 hours)

#### 4.1 Reconcile Service State

**File:** `pkg/reconcile/reconcile.go`

```go
package reconcile

import (
    "context"
    "fmt"
    
    "github.com/cilo/cilo/pkg/models"
    "github.com/cilo/cilo/pkg/runtime"
    "github.com/cilo/cilo/pkg/state"
)

// Environment reconciles a single environment
func Environment(ctx context.Context, env *models.Environment, provider runtime.Provider) (*models.Environment, error) {
    // Get actual runtime status
    status, err := provider.Status(ctx, env)
    if err != nil {
        return env, fmt.Errorf("failed to get status: %w", err)
    }
    
    // Get actual services
    services, err := provider.Services(ctx, env)
    if err != nil {
        return env, fmt.Errorf("failed to get services: %w", err)
    }
    
    // Update environment
    env.Status = string(status.State)
    
    // Reconcile services
    env.Services = make(map[string]*models.Service)
    for _, svc := range services {
        env.Services[svc.Name] = svc
    }
    
    return env, nil
}

// All reconciles all environments in state
func All(ctx context.Context, st *models.State, providerFactory func(string) (runtime.Provider, error)) []error {
    var errors []error
    
    for hostID, host := range st.Hosts {
        provider, err := providerFactory(hostID)
        if err != nil {
            errors = append(errors, fmt.Errorf("host %s: failed to get provider: %w", hostID, err))
            continue
        }
        
        for envKey, env := range host.Environments {
            reconciled, err := Environment(ctx, env, provider)
            if err != nil {
                errors = append(errors, fmt.Errorf("env %s: %w", envKey, err))
                continue
            }
            
            // Update in state
            host.Environments[envKey] = reconciled
        }
    }
    
    return errors
}

// Orphans finds Docker resources not tracked in state
func Orphans(ctx context.Context, st *models.State, provider runtime.Provider) ([]OrphanedResource, error) {
    var orphans []OrphanedResource
    
    // List all cilo-prefixed containers
    // Compare with state
    // Return differences
    
    // TODO: Implement based on provider capabilities
    
    return orphans, nil
}

type OrphanedResource struct {
    Type        string  // "container", "network", "volume"
    ID          string
    Name        string
    Environment string  // Inferred from labels/name
}
```

#### 4.2 Doctor Command with Reconciliation

**File:** `cmd/root.go`

```go
var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Check and repair cilo configuration",
    RunE: func(cmd *cobra.Command, args []string) error {
        fix, _ := cmd.Flags().GetBool("fix")
        
        fmt.Println("🔍 Checking cilo configuration...\n")
        
        // Check Docker
        checkDocker()
        
        // Check dnsmasq
        checkDNS()
        
        // Load state
        st, err := state.LoadState()
        if err != nil {
            return err
        }
        
        // Reconcile all environments
        fmt.Println("\n📊 Reconciling environments...")
        providerFactory := func(hostID string) (runtime.Provider, error) {
            // For now, just Docker locally
            return docker.NewProvider(), nil
        }
        
        errors := reconcile.All(context.Background(), st, providerFactory)
        
        if len(errors) > 0 {
            fmt.Printf("⚠️  Found %d issues:\n", len(errors))
            for _, err := range errors {
                fmt.Printf("  - %v\n", err)
            }
        } else {
            fmt.Println("✓ All environments reconciled")
        }
        
        // Save reconciled state
        if fix && len(errors) > 0 {
            if err := state.SaveState(st); err != nil {
                return err
            }
            fmt.Println("✓ State updated")
            
            // Regenerate DNS
            if err := dns.UpdateDNS(st); err != nil {
                return err
            }
            fmt.Println("✓ DNS updated")
        }
        
        return nil
    },
}

func init() {
    doctorCmd.Flags().Bool("fix", false, "Fix issues automatically")
}
```

---

### Task 5: Collision Detection (Day 3, 2 hours)

#### 5.1 Subnet Collision Detection

**File:** `pkg/network/collision.go`

```go
package network

import (
    "context"
    "net"
    "os/exec"
    "strings"
)

// CheckSubnetCollision checks if subnet conflicts with existing Docker networks
func CheckSubnetCollision(ctx context.Context, subnet string) (bool, error) {
    _, ipnet, err := net.ParseCIDR(subnet)
    if err != nil {
        return false, err
    }
    
    // Get all Docker networks
    cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{.ID}}")
    output, err := cmd.Output()
    if err != nil {
        return false, err
    }
    
    networkIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
    
    for _, id := range networkIDs {
        // Inspect each network
        inspectCmd := exec.CommandContext(ctx, "docker", "network", "inspect", id, "--format", "{{range .IPAM.Config}}{{.Subnet}}{{end}}")
        subnetOutput, err := inspectCmd.Output()
        if err != nil {
            continue
        }
        
        existingSubnet := strings.TrimSpace(string(subnetOutput))
        if existingSubnet == "" {
            continue
        }
        
        _, existingNet, err := net.ParseCIDR(existingSubnet)
        if err != nil {
            continue
        }
        
        // Check for overlap
        if subnetsOverlap(ipnet, existingNet) {
            return true, nil
        }
    }
    
    return false, nil
}

func subnetsOverlap(a, b *net.IPNet) bool {
    return a.Contains(b.IP) || b.Contains(a.IP)
}

// CheckRouteCollision checks if subnet conflicts with host routes
func CheckRouteCollision(subnet string) (bool, error) {
    _, ipnet, err := net.ParseCIDR(subnet)
    if err != nil {
        return false, err
    }
    
    // Get routing table
    cmd := exec.Command("ip", "route", "show")
    output, err := cmd.Output()
    if err != nil {
        // Try macOS
        cmd = exec.Command("netstat", "-rn")
        output, err = cmd.Output()
        if err != nil {
            return false, err
        }
    }
    
    // Parse routes and check for overlap
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        // Parse line for CIDR
        fields := strings.Fields(line)
        if len(fields) > 0 {
            if _, routeNet, err := net.ParseCIDR(fields[0]); err == nil {
                if subnetsOverlap(ipnet, routeNet) {
                    return true, nil
                }
            }
        }
    }
    
    return false, nil
}
```

#### 5.2 Use in State Operations

**File:** `pkg/state/state.go`

```go
func CreateEnvironment(name, source, project string) (*Environment, error) {
    var env *Environment
    
    err := WithLock(func(state *State) error {
        // ... existing checks ...
        
        // Allocate subnet
        state.SubnetCounter++
        subnet := fmt.Sprintf("%s%d.0/24", baseSubnet, state.SubnetCounter)
        
        // Check for collisions
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if collision, err := network.CheckSubnetCollision(ctx, subnet); err != nil {
            return fmt.Errorf("failed to check subnet collision: %w", err)
        } else if collision {
            // Try next subnet
            state.SubnetCounter++
            subnet = fmt.Sprintf("%s%d.0/24", baseSubnet, state.SubnetCounter)
            
            // Check again
            if collision, _ := network.CheckSubnetCollision(ctx, subnet); collision {
                return fmt.Errorf("failed to allocate non-conflicting subnet after 2 attempts")
            }
        }
        
        // Check route collision
        if collision, err := network.CheckRouteCollision(subnet); err != nil {
            return fmt.Errorf("failed to check route collision: %w", err)
        } else if collision {
            return fmt.Errorf("subnet %s conflicts with existing route", subnet)
        }
        
        // ... rest of environment creation ...
    })
    
    return env, err
}
```

---

### Task 6: Integration Tests (Day 3, 4 hours)

#### 6.1 Test Infrastructure

**File:** `test/integration/harness.go`

```go
package integration

import (
    "os"
    "path/filepath"
    "testing"
)

type TestHarness struct {
    t             *testing.T
    tempDir       string
    stateFile     string
    dnsConfigFile string
}

func NewHarness(t *testing.T) *TestHarness {
    tempDir, err := os.MkdirTemp("", "cilo-test-*")
    if err != nil {
        t.Fatal(err)
    }
    
    h := &TestHarness{
        t:             t,
        tempDir:       tempDir,
        stateFile:     filepath.Join(tempDir, "state.json"),
        dnsConfigFile: filepath.Join(tempDir, "dnsmasq.conf"),
    }
    
    // Override state path for tests
    os.Setenv("CILO_STATE_PATH", h.stateFile)
    os.Setenv("CILO_DNS_CONFIG", h.dnsConfigFile)
    
    return h
}

func (h *TestHarness) Cleanup() {
    os.RemoveAll(h.tempDir)
}

func (h *TestHarness) CreateProject(name string, composeContent string) string {
    projectDir := filepath.Join(h.tempDir, "projects", name)
    os.MkdirAll(projectDir, 0755)
    
    composePath := filepath.Join(projectDir, "docker-compose.yml")
    os.WriteFile(composePath, []byte(composeContent), 0644)
    
    return projectDir
}
```

#### 6.2 Test Cases

**File:** `test/integration/lifecycle_test.go`

```go
package integration

import (
    "context"
    "testing"
    "time"
    
    "github.com/cilo/cilo/pkg/state"
    "github.com/cilo/cilo/pkg/runtime/docker"
)

func TestCreateUpDownDestroy(t *testing.T) {
    h := NewHarness(t)
    defer h.Cleanup()
    
    // Create project
    projectDir := h.CreateProject("testapp", `
version: '3'
services:
  nginx:
    image: nginx:alpine
    labels:
      cilo.ingress: "true"
`)
    
    // Initialize state
    if err := state.InitializeState(); err != nil {
        t.Fatal(err)
    }
    
    // Create environment
    env, err := state.CreateEnvironment("dev", projectDir, "testapp")
    if err != nil {
        t.Fatalf("CreateEnvironment failed: %v", err)
    }
    
    if env.Name != "dev" {
        t.Errorf("Expected name=dev, got %s", env.Name)
    }
    
    // Up
    provider := docker.NewProvider()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := provider.Up(ctx, env, runtime.UpOptions{}); err != nil {
        t.Fatalf("Up failed: %v", err)
    }
    
    // Check status
    status, err := provider.Status(ctx, env)
    if err != nil {
        t.Fatalf("Status failed: %v", err)
    }
    
    if status.State != runtime.StateRunning {
        t.Errorf("Expected StateRunning, got %v", status.State)
    }
    
    // Down
    if err := provider.Down(ctx, env); err != nil {
        t.Fatalf("Down failed: %v", err)
    }
    
    // Destroy
    if err := provider.Destroy(ctx, env); err != nil {
        t.Fatalf("Destroy failed: %v", err)
    }
    
    // Delete from state
    if err := state.DeleteEnvironment("testapp", "dev"); err != nil {
        t.Fatalf("DeleteEnvironment failed: %v", err)
    }
}

func TestConcurrentCreate(t *testing.T) {
    h := NewHarness(t)
    defer h.Cleanup()
    
    projectDir := h.CreateProject("testapp", simpleCompose)
    
    state.InitializeState()
    
    // Create 10 environments concurrently
    concurrency := 10
    errors := make(chan error, concurrency)
    
    for i := 0; i < concurrency; i++ {
        go func(idx int) {
            name := fmt.Sprintf("env-%d", idx)
            _, err := state.CreateEnvironment(name, projectDir, "testapp")
            errors <- err
        }(i)
    }
    
    // Collect results
    for i := 0; i < concurrency; i++ {
        if err := <-errors; err != nil {
            t.Errorf("Concurrent create %d failed: %v", i, err)
        }
    }
    
    // Verify all environments created
    st, _ := state.LoadState()
    if len(st.Hosts["local"].Environments) != concurrency {
        t.Errorf("Expected %d environments, got %d", concurrency, len(st.Hosts["local"].Environments))
    }
}

func TestDNSReconciliation(t *testing.T) {
    h := NewHarness(t)
    defer h.Cleanup()
    
    // ... test DNS rendering after env create/up ...
}
```

---

## Dependencies

```bash
# Add to go.mod
go get github.com/gofrs/flock
```

---

## Migration Guide for Existing Users

### Automatic Migration

On first run after upgrade:
1. `cilo` detects v1 state
2. Automatically migrates to v2
3. Backs up old state to `state.json.v1.backup`
4. Regenerates DNS config from scratch

### Manual Migration (if needed)

```bash
# Backup current state
cp ~/.cilo/state.json ~/.cilo/state.json.backup

# Run any command to trigger migration
cilo list

# Verify
cilo doctor
```

---

## Testing Checklist

- [ ] Unit tests for state locking
- [ ] Unit tests for state migrations
- [ ] Unit tests for DNS rendering
- [ ] Integration test: create/up/down/destroy
- [ ] Integration test: concurrent creates
- [ ] Integration test: crash recovery
- [ ] Integration test: DNS updates
- [ ] Manual test: complex compose file (extends, profiles)
- [ ] Manual test: migration from v1 to v2

---

## Deliverables

1. **Concurrent-safe state operations**
2. **Compose override model**
3. **Rendered DNS configs**
4. **Reconciliation system**
5. **Collision detection**
6. **Integration test suite**
7. **Migration system**

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Migration breaks existing setups | Automatic backup, rollback instructions |
| File locking issues on Windows | Use cross-platform flock library |
| DNS reload fails | Graceful fallback to restart |
| Concurrent tests flaky | Proper cleanup, isolated temp dirs |

---

## Next Phase

After Phase 1, proceed to:
- **Phase 2A:** Shared Resources (builds on solid foundation)
- **Phase 2B:** Remote Operation (uses provider interface + stable state)
