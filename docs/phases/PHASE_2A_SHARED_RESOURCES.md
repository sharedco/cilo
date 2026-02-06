# Phase 2A: Shared Resources

**Duration:** 1-2 days  
**Dependencies:** Phase 1 (Foundations)  
**Goal:** Enable resource sharing between environments

---

## Objectives

1. Support external Docker networks for resource sharing
2. Track shared network dependencies in state
3. Lifecycle protection for shared resources
4. DNS visibility for shared services across environments

---

## Success Criteria

- [ ] Multiple environments can share a common database environment
- [ ] Destroying shared resources blocked when in use (or warned)
- [ ] Shared services accessible via DNS from consumer environments
- [ ] State tracks which environments reference which shared resources
- [ ] Clear error messages when shared resources unavailable

---

## Use Cases

### Use Case 1: Shared Database
```bash
# Create long-lived database environment
cilo create shared-db --from ~/db-services
cilo up shared-db

# Create multiple frontends sharing the database
cilo create frontend-1 --from ~/app --shared-network cilo-shared-db
cilo create frontend-2 --from ~/app --shared-network cilo-shared-db

# Both frontends can access postgres.shared-db.test
```

### Use Case 2: Shared Development Tools
```bash
# Create tools environment (mailhog, minio, redis)
cilo create dev-tools --from ~/dev-tools
cilo up dev-tools

# Multiple projects use shared tools
cilo create api --from ~/api --shared-network cilo-dev-tools
cilo create web --from ~/web --shared-network cilo-dev-tools
```

### Use Case 3: Prevent Accidental Deletion
```bash
cilo destroy shared-db
# Error: Cannot destroy shared-db, referenced by: frontend-1, frontend-2
# Use --force to destroy anyway (will break dependent environments)
```

---

## Detailed Tasks

### Task 1: State Schema for Shared Resources (2 hours)

#### 1.1 Update State Model

**File:** `pkg/models/models.go`

```go
// State v2 gains shared_networks tracking
type State struct {
    Version        int                     `json:"version"`
    SubnetCounter  int                     `json:"subnet_counter"`
    Hosts          map[string]*Host        `json:"hosts"`
    SharedNetworks map[string]*SharedNetwork `json:"shared_networks"`  // NEW
}

// SharedNetwork tracks a network shared between environments
type SharedNetwork struct {
    Name         string    `json:"name"`
    CreatedBy    string    `json:"created_by"`     // Environment key that created it
    CreatedAt    time.Time `json:"created_at"`
    ReferencedBy []string  `json:"referenced_by"`  // Environment keys using it
}

// Environment gains shared_networks field
type Environment struct {
    Name           string              `json:"name"`
    Project        string              `json:"project,omitempty"`
    CreatedAt      time.Time           `json:"created_at"`
    Subnet         string              `json:"subnet"`
    Status         string              `json:"status"`
    Source         string              `json:"source,omitempty"`
    Services       map[string]*Service `json:"services"`
    SharedNetworks []string            `json:"shared_networks,omitempty"`  // NEW
}
```

#### 1.2 Shared Network Operations

**File:** `pkg/state/shared.go`

```go
package state

// RegisterSharedNetwork adds a shared network to state
func RegisterSharedNetwork(networkName, createdBy string) error {
    return WithLock(func(state *State) error {
        if state.SharedNetworks == nil {
            state.SharedNetworks = make(map[string]*SharedNetwork)
        }
        
        if _, exists := state.SharedNetworks[networkName]; exists {
            return fmt.Errorf("shared network %s already exists", networkName)
        }
        
        state.SharedNetworks[networkName] = &SharedNetwork{
            Name:         networkName,
            CreatedBy:    createdBy,
            CreatedAt:    time.Now(),
            ReferencedBy: []string{createdBy},
        }
        
        return nil
    })
}

// AddSharedNetworkReference records that an environment uses a shared network
func AddSharedNetworkReference(networkName, envKey string) error {
    return WithLock(func(state *State) error {
        network, exists := state.SharedNetworks[networkName]
        if !exists {
            return fmt.Errorf("shared network %s does not exist", networkName)
        }
        
        // Add reference if not already present
        for _, ref := range network.ReferencedBy {
            if ref == envKey {
                return nil // Already referenced
            }
        }
        
        network.ReferencedBy = append(network.ReferencedBy, envKey)
        return nil
    })
}

// RemoveSharedNetworkReference removes an environment's reference
func RemoveSharedNetworkReference(networkName, envKey string) error {
    return WithLock(func(state *State) error {
        network, exists := state.SharedNetworks[networkName]
        if !exists {
            return nil // Already gone
        }
        
        // Remove reference
        newRefs := []string{}
        for _, ref := range network.ReferencedBy {
            if ref != envKey {
                newRefs = append(newRefs, ref)
            }
        }
        network.ReferencedBy = newRefs
        
        // If no more references and not the creator, remove network tracking
        // (Leave it if created_by still exists)
        if len(network.ReferencedBy) == 0 {
            delete(state.SharedNetworks, networkName)
        }
        
        return nil
    })
}

// GetSharedNetworkReferences returns environments using a shared network
func GetSharedNetworkReferences(networkName string) ([]string, error) {
    state, err := LoadState()
    if err != nil {
        return nil, err
    }
    
    network, exists := state.SharedNetworks[networkName]
    if !exists {
        return []string{}, nil
    }
    
    return network.ReferencedBy, nil
}
```

---

### Task 2: Create with Shared Networks (3 hours)

#### 2.1 CLI Flag

**File:** `cmd/lifecycle.go`

```go
var createCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new environment",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        from, _ := cmd.Flags().GetString("from")
        projectFlag, _ := cmd.Flags().GetString("project")
        sharedNetworks, _ := cmd.Flags().GetStringSlice("shared-network")  // NEW
        
        // ... existing project/source logic ...
        
        // Create environment
        env, err := state.CreateEnvironment(name, source, project)
        if err != nil {
            return err
        }
        
        // Add shared networks
        env.SharedNetworks = sharedNetworks
        
        // Validate shared networks exist
        for _, networkName := range sharedNetworks {
            // Check if network exists in Docker
            exists, err := checkNetworkExists(networkName)
            if err != nil {
                return fmt.Errorf("failed to check network %s: %w", networkName, err)
            }
            if !exists {
                return fmt.Errorf("shared network %s does not exist", networkName)
            }
            
            // Track reference in state
            envKey := makeEnvKey(project, name)
            if err := state.AddSharedNetworkReference(networkName, envKey); err != nil {
                return err
            }
        }
        
        // Update state
        if err := state.UpdateEnvironment(env); err != nil {
            return err
        }
        
        // ... rest of create logic ...
        
        return nil
    },
}

func init() {
    createCmd.Flags().StringSlice("shared-network", []string{}, "Join shared network(s)")
    // ... existing flags ...
}

func checkNetworkExists(networkName string) (bool, error) {
    cmd := exec.Command("docker", "network", "inspect", networkName)
    return cmd.Run() == nil, nil
}
```

#### 2.2 Auto-Create Shared Networks

**File:** `cmd/lifecycle.go`

```go
var upCmd = &cobra.Command{
    Use:   "up <name>",
    Short: "Start an environment",
    RunE: func(cmd *cobra.Command, args []string) error {
        // ... load environment ...
        
        // Create shared network if --create-shared flag present
        createShared, _ := cmd.Flags().GetBool("create-shared")
        if createShared {
            networkName := fmt.Sprintf("cilo-shared-%s", env.Name)
            
            // Create Docker network
            if err := createSharedNetwork(networkName, env.Subnet); err != nil {
                return err
            }
            
            // Register in state
            envKey := makeEnvKey(env.Project, env.Name)
            if err := state.RegisterSharedNetwork(networkName, envKey); err != nil {
                return err
            }
            
            fmt.Printf("✓ Created shared network: %s\n", networkName)
        }
        
        // ... rest of up logic ...
    },
}

func init() {
    upCmd.Flags().Bool("create-shared", false, "Create a shared network for this environment")
}

func createSharedNetwork(networkName, subnet string) error {
    cmd := exec.Command("docker", "network", "create",
        "--driver", "bridge",
        "--subnet", subnet,
        "--label", "cilo.shared=true",
        networkName,
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to create network: %w\nOutput: %s", err, output)
    }
    
    return nil
}
```

---

### Task 3: Override Generation with Shared Networks (2 hours)

#### 3.1 Update Override Generator

**File:** `pkg/compose/override.go`

```go
// GenerateOverride now handles shared networks
func GenerateOverride(env *models.Environment, outputPath string) error {
    override := models.ComposeFile{
        Services: make(map[string]*models.ComposeService),
        Networks: make(map[string]*models.ComposeNetwork),
    }
    
    // Environment's own network
    networkName := fmt.Sprintf("cilo-%s", env.Name)
    
    // For each service
    for name, svc := range env.Services {
        svcNetworks := map[string]interface{}{
            networkName: map[string]string{
                "ipv4_address": svc.IP,
            },
        }
        
        // Add shared networks (no IP assignment, just join)
        for _, sharedNet := range env.SharedNetworks {
            svcNetworks[sharedNet] = struct{}{}  // Join without specific IP
        }
        
        override.Services[name] = &models.ComposeService{
            ContainerName: svc.Container,
            Ports:         []string{},
            Networks:      svcNetworks,
            Labels: map[string]string{
                "cilo.environment": env.Name,
                "cilo.project":     env.Project,
            },
        }
    }
    
    // Add environment's own network
    override.Networks[networkName] = &models.ComposeNetwork{
        Name:   networkName,
        Driver: "bridge",
        IPAM: &models.ComposeIPAM{
            Config: []models.ComposeIPAMConfig{
                {Subnet: env.Subnet},
            },
        },
    }
    
    // Add shared networks as external
    for _, sharedNet := range env.SharedNetworks {
        override.Networks[sharedNet] = &models.ComposeNetwork{
            External: true,
            Name:     sharedNet,
        }
    }
    
    // ... rest of generation ...
}
```

---

### Task 4: Lifecycle Protection (2 hours)

#### 4.1 Destroy with Reference Check

**File:** `cmd/lifecycle.go`

```go
var destroyCmd = &cobra.Command{
    Use:   "destroy <name>",
    Short: "Destroy an environment",
    RunE: func(cmd *cobra.Command, args []string) error {
        // ... load environment ...
        
        force, _ := cmd.Flags().GetBool("force")
        
        // Check if this environment has shared networks being used by others
        envKey := makeEnvKey(project, name)
        
        for networkName, network := range state.SharedNetworks {
            if network.CreatedBy == envKey {
                // This env created the network, check references
                if len(network.ReferencedBy) > 1 {
                    // Others are using it
                    otherUsers := []string{}
                    for _, ref := range network.ReferencedBy {
                        if ref != envKey {
                            otherUsers = append(otherUsers, ref)
                        }
                    }
                    
                    if !force {
                        return fmt.Errorf(
                            "cannot destroy %s: shared network %s is used by: %s\nUse --force to destroy anyway",
                            envKey, networkName, strings.Join(otherUsers, ", "),
                        )
                    } else {
                        fmt.Printf("⚠️  Warning: destroying %s will break environments: %s\n",
                            networkName, strings.Join(otherUsers, ", "))
                    }
                }
            }
        }
        
        // ... rest of destroy logic ...
        
        // Clean up shared network references
        for _, networkName := range env.SharedNetworks {
            state.RemoveSharedNetworkReference(networkName, envKey)
        }
        
        return nil
    },
}
```

#### 4.2 List Shared Networks Command

**File:** `cmd/shared.go`

```go
package cmd

var sharedCmd = &cobra.Command{
    Use:   "shared",
    Short: "Manage shared networks",
}

var sharedListCmd = &cobra.Command{
    Use:   "list",
    Short: "List shared networks",
    RunE: func(cmd *cobra.Command, args []string) error {
        st, err := state.LoadState()
        if err != nil {
            return err
        }
        
        if len(st.SharedNetworks) == 0 {
            fmt.Println("No shared networks")
            return nil
        }
        
        fmt.Printf("%-30s %-20s %-10s %s\n", "NETWORK", "CREATED BY", "REFERENCES", "CREATED")
        for name, network := range st.SharedNetworks {
            fmt.Printf("%-30s %-20s %-10d %s\n",
                name,
                network.CreatedBy,
                len(network.ReferencedBy),
                network.CreatedAt.Format("2006-01-02"),
            )
            
            // Show references
            for _, ref := range network.ReferencedBy {
                fmt.Printf("  └─ %s\n", ref)
            }
        }
        
        return nil
    },
}

var sharedInspectCmd = &cobra.Command{
    Use:   "inspect <network>",
    Short: "Inspect shared network",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        networkName := args[0]
        
        st, err := state.LoadState()
        if err != nil {
            return err
        }
        
        network, exists := st.SharedNetworks[networkName]
        if !exists {
            return fmt.Errorf("shared network %s not found", networkName)
        }
        
        fmt.Printf("Network: %s\n", network.Name)
        fmt.Printf("Created By: %s\n", network.CreatedBy)
        fmt.Printf("Created At: %s\n", network.CreatedAt)
        fmt.Printf("References (%d):\n", len(network.ReferencedBy))
        for _, ref := range network.ReferencedBy {
            fmt.Printf("  - %s\n", ref)
        }
        
        return nil
    },
}

func init() {
    sharedCmd.AddCommand(sharedListCmd)
    sharedCmd.AddCommand(sharedInspectCmd)
    rootCmd.AddCommand(sharedCmd)
}
```

---

### Task 5: DNS for Shared Services (2 hours)

#### 5.1 Cross-Environment DNS Resolution

Shared services need to be accessible from other environments via DNS.

**Approach:** Shared network services get DNS entries visible globally.

**File:** `pkg/dns/render.go`

```go
func RenderConfig(state *State) (string, error) {
    var b strings.Builder
    
    // ... header and base config ...
    
    // Shared networks section
    if len(state.SharedNetworks) > 0 {
        b.WriteString("# Shared Networks\n")
        
        for networkName, network := range state.SharedNetworks {
            b.WriteString(fmt.Sprintf("# Network: %s (created by %s)\n", networkName, network.CreatedBy))
            
            // Find the environment that created this network
            // Its services are the "authoritative" ones for this shared network
            creatorEnv := findEnvironmentByKey(state, network.CreatedBy)
            if creatorEnv != nil {
                for _, svc := range creatorEnv.Services {
                    // Create global DNS entry for shared service
                    // Format: service.shared-network.test
                    sanitizedNetwork := strings.TrimPrefix(networkName, "cilo-shared-")
                    b.WriteString(fmt.Sprintf(
                        "address=/%s.%s.test/%s\n",
                        svc.Name, sanitizedNetwork, svc.IP,
                    ))
                }
            }
            
            b.WriteString("\n")
        }
    }
    
    // ... rest of per-environment DNS ...
}

func findEnvironmentByKey(state *State, envKey string) *Environment {
    for _, host := range state.Hosts {
        if env, exists := host.Environments[envKey]; exists {
            return env
        }
    }
    return nil
}
```

**Result:**
```bash
# Shared database environment: shared-db
postgres.shared-db.test -> 10.224.2.5

# Any environment can access it
# From frontend-1:
ping postgres.shared-db.test  # Works!
```

---

## Testing

### Integration Tests

**File:** `test/integration/shared_test.go`

```go
func TestSharedNetwork(t *testing.T) {
    h := NewHarness(t)
    defer h.Cleanup()
    
    // Create shared database environment
    dbProject := h.CreateProject("db", `
version: '3'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: test
`)
    
    dbEnv, _ := state.CreateEnvironment("shared-db", dbProject, "db")
    
    provider := docker.NewProvider()
    ctx := context.Background()
    
    // Up with shared network creation
    createSharedNetwork("cilo-shared-db", dbEnv.Subnet)
    state.RegisterSharedNetwork("cilo-shared-db", "db/shared-db")
    
    provider.Up(ctx, dbEnv, runtime.UpOptions{})
    
    // Create frontend environment using shared network
    appProject := h.CreateProject("app", `
version: '3'
services:
  web:
    image: nginx:alpine
`)
    
    appEnv, _ := state.CreateEnvironment("frontend", appProject, "app")
    appEnv.SharedNetworks = []string{"cilo-shared-db"}
    state.AddSharedNetworkReference("cilo-shared-db", "app/frontend")
    state.UpdateEnvironment(appEnv)
    
    provider.Up(ctx, appEnv, runtime.UpOptions{})
    
    // Verify connectivity
    // From web container, should be able to reach postgres
    execCmd := []string{
        "docker", "exec", "cilo_frontend_web",
        "ping", "-c", "1", "postgres.shared-db.test",
    }
    
    cmd := exec.Command(execCmd[0], execCmd[1:]...)
    if err := cmd.Run(); err != nil {
        t.Errorf("Cannot ping shared service: %v", err)
    }
    
    // Try to destroy shared-db (should fail)
    err := destroyEnvironment("db", "shared-db", false)
    if err == nil {
        t.Error("Expected destroy to fail due to references")
    }
    
    // Cleanup frontend first
    provider.Down(ctx, appEnv)
    state.DeleteEnvironment("app", "frontend")
    state.RemoveSharedNetworkReference("cilo-shared-db", "app/frontend")
    
    // Now destroy should work
    provider.Down(ctx, dbEnv)
    state.DeleteEnvironment("db", "shared-db")
}
```

---

## Examples

### Example 1: Shared Database

**Create database environment:**
```bash
cilo create db --from ~/database-services
cilo up db --create-shared
# Creates network: cilo-shared-db
```

**Create apps using shared database:**
```bash
cilo create api --from ~/api-service --shared-network cilo-shared-db
cilo create admin --from ~/admin-panel --shared-network cilo-shared-db

cilo up api
cilo up admin
```

**Access from apps:**
```javascript
// In api-service or admin-panel
const dbHost = 'postgres.db.test';  // Shared service via DNS
const connection = new Pool({ host: dbHost, ... });
```

### Example 2: Development Tools

**Create shared tools:**
```bash
# ~/dev-tools/docker-compose.yml
version: '3'
services:
  mailhog:
    image: mailhog/mailhog
  minio:
    image: minio/minio
  redis:
    image: redis:alpine

cilo create tools --from ~/dev-tools
cilo up tools --create-shared
```

**Use from multiple projects:**
```bash
cilo create project-a --from ~/project-a --shared-network cilo-shared-tools
cilo create project-b --from ~/project-b --shared-network cilo-shared-tools

# Both can access:
# - mailhog.tools.test:8025
# - minio.tools.test:9000
# - redis.tools.test:6379
```

---

## Documentation Updates

### README Updates

Add section:

```markdown
## Shared Resources

Share common services (databases, caches, tools) across multiple environments:

### Create Shared Network
```bash
cilo create db --from ~/database
cilo up db --create-shared
```

### Use Shared Network
```bash
cilo create app --from ~/myapp --shared-network cilo-shared-db
cilo up app
```

### List Shared Networks
```bash
cilo shared list
```

### Lifecycle Protection
Cilo prevents destroying shared resources while they're in use:
```bash
cilo destroy db
# Error: Cannot destroy db, shared network is used by: app1, app2
```
```

---

## Deliverables

- [ ] State tracking for shared networks
- [ ] CLI support: `--shared-network`, `--create-shared`
- [ ] Lifecycle protection (prevent destroy of in-use resources)
- [ ] DNS resolution for shared services
- [ ] `cilo shared` command group
- [ ] Integration tests
- [ ] Documentation updates

---

## Next Phase

Proceed to **Phase 2B: Remote Operation** - enables shared resources across remote hosts via mesh networking.
