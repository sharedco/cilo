Revised Implementation Plan: Shared Services
Architectural Decisions (Resolved)
| Decision | Choice | Rationale |
|----------|--------|-----------|
| Network | Option B (direct attachment) | DX simplicity, fewer moving parts. Practical for 3-10 environments. Can migrate to Option A later. |
| DNS | Transparent (elasticsearch.agent-1.test) | Maintains abstraction illusion. Code works identically whether shared or isolated. |
| Startup | Existing compose file | No new concepts to learn. Label or CLI flag changes behavior. |
| Scope | Project-scoped | Can widen to global later; narrowing would be breaking. |
| Resources | Compose file values | No Cilo-specific differentiation. |
---
Revised Phasing (Spike-First Approach)
Phase 0: SPIKE — Validate Docker Networking (Timeboxed: 2-3 hours)
Goal: Prove docker network connect works reliably before committing to architecture.
Create a standalone test script that:
1. Creates two environment networks (cilo_test1, cilo_test2)
2. Starts an Elasticsearch container on one network
3. Connects it to the second network
4. Verifies DNS resolution works from both networks
5. Tests both Linux and macOS behaviors
Success criteria:
- Container can be reached from both networks by name
- No connectivity issues between environments
- Clean disconnection/removal works
Files to create: spike/network-connect-test.sh (temporary, not committed)
---
Phase 1: Minimal Runtime Implementation
Goal: Get the core Docker operations working end-to-end.
New file: cilo/pkg/share/manager.go
type Manager struct {
    provider runtime.Provider
}
// EnsureSharedService creates or returns existing shared container
// Returns: container name, IP address, error
func (m *Manager) EnsureSharedService(serviceName string, composeFiles []string) (string, string, error)
// ConnectSharedServiceToEnvironment attaches shared container to env network
func (m *Manager) ConnectSharedServiceToEnvironment(serviceName, envName string) error
// DisconnectSharedServiceFromEnvironment removes network attachment
func (m *Manager) DisconnectSharedServiceFromEnvironment(serviceName, envName string) error
// GetSharedServiceIP returns IP of shared container for DNS
func (m *Manager) GetSharedServiceIP(serviceName string) (string, error)
Update: cilo/pkg/runtime/docker/provider.go
- Add ConnectContainerToNetwork(containerName, networkName) error
- Add DisconnectContainerFromNetwork(containerName, networkName) error
- Add GetContainerIP(containerName) (string, error)
- Add ListContainersWithLabel(labelKey, labelValue) ([]string, error)
Update: cilo/pkg/compose/loader.go
- Add GetServicesWithLabel(composeFiles []string, label string) ([]string, error) - returns services with cilo.share: "true"
---
Phase 2: Data Model (After Spike Validation)
Update: cilo/pkg/models/models.go
// Add to State
SharedServices map[string]*SharedService `json:"shared_services,omitempty"`
// New struct
type SharedService struct {
    Name         string    `json:"name"`           // Service name (e.g., "elasticsearch")
    Container    string    `json:"container"`      // Docker container name (e.g., "cilo_shared_elasticsearch")
    IP           string    `json:"ip"`
    Project      string    `json:"project"`
    Image        string    `json:"image"`          // For conflict detection
    CreatedAt    time.Time `json:"created_at"`
    UsedBy       []string  `json:"used_by"`        // Environment keys: "project/env"
    ConfigHash   string    `json:"config_hash"`    // Hash of compose service definition
}
// Add to Environment
UsesSharedServices []string `json:"uses_shared_services,omitempty"` // Names of shared services this env consumes
Conflict Detection Strategy:
- Store hash of service definition (image, env vars, volumes, etc.)
- On connect, compare hash with existing shared service
- If different: warn user, suggest options:
  - Use existing version (ignore mismatch)
  - Recreate shared service with new definition (affects all environments)
  - Don't share this service (create isolated instance)
---
Phase 3: CLI Integration
Update: cilo/cmd/run.go
- Add --share flag (comma-separated service names)
- Parse and pass to environment creation
Update: cilo/cmd/lifecycle.go
In up command:
// After loading compose files
sharedServices := compose.GetServicesWithLabel(composeFiles, "cilo.share")
if flagShare != "" {
    // CLI flag overrides/adds to label-based sharing
    sharedServices = merge(sharedServices, parseFlag(flagShare))
}
// Start shared services before environment
shareMgr := share.NewManager(docker.NewProvider())
for _, svc := range sharedServices {
    container, ip, err := shareMgr.EnsureSharedService(svc, composeFiles)
    // ... connect to environment network
}
// Update environment state with UsesSharedServices
env.UsesSharedServices = sharedServices
In down command:
// Disconnect shared services before stopping environment
shareMgr := share.NewManager(docker.NewProvider())
for _, svc := range env.UsesSharedServices {
    shareMgr.DisconnectSharedServiceFromEnvironment(svc, env.Name)
    // Decrement reference count, stop container if zero
}
---
Phase 4: Compose Override Updates
Update: cilo/pkg/compose/compose.go
Modify Transform() to handle shared services:
func Transform(env *models.Environment, baseFiles []string, overridePath, dnsSuffix string, sharedServices []string) error {
    // Existing service loading...
    
    serviceOverrides := override["services"].(map[string]interface{})
    for _, name := range SortedServiceNames(services) {
        if contains(sharedServices, name) {
            // Skip: this service is shared, don't create in this environment
            continue
        }
        // ... existing override generation
    }
    
    // For shared services, generate DNS aliases
    networks["default"]["aliases"] = generateSharedServiceAliases(env, sharedServices)
}
---
Phase 5: DNS Integration
Update: cilo/pkg/dns/render.go
Add entries for shared services with transparent naming:
// For shared service ES used by env "agent-1":
// elasticsearch.agent-1.test -> shared container IP
// (Same as isolated, maintaining abstraction)
---
Phase 6: cilo doctor Integration
Update: cilo/pkg/reconcile/reconcile.go (or new cilo/pkg/share/doctor.go)
Add checks:
1. Orphaned shared containers: Reference count = 0 but container still running
2. Missing shared containers: Reference count > 0 but container not running
3. Version conflicts: Shared service config hash differs from registered
Update: cilo/cmd/doctor.go
Add new check section:
fmt.Println("\n🔎 Checking shared services...")
orphanedShared, missingShared, conflicts := share.CheckSharedServices(st)
// Display and fix
---
Open Questions
1. Version conflict UX: When compose definitions differ, what's the default behavior?
   - A) Warn and use existing (conservative)
   - B) Warn and stop (fail fast)
   - C) Interactive prompt
   - D) Flag to control (--shared-conflict=stop|ignore|recreate)
2. Shared service teardown: Should there be a grace period before stopping a shared container when reference count hits zero? (Prevents churn if user is cycling environments quickly)
3. Status visibility: Should cilo list show which environments are sharing services, or is that too much noise?
4. Testing scope: Do you want integration tests for the networking spike, or manual validation sufficient for now?

Excellent feedback. Let me incorporate all of this into a refined plan.
Final Implementation Plan: Shared Services
Critical Technical Discovery
Network aliases are mandatory. When connecting a shared container to an environment network, we must attach an alias matching the original service name:
docker network connect --alias elasticsearch cilo-agent-1-network cilo_shared_elasticsearch
Without this, containers inside the environment can't resolve elasticsearch — they don't use the host's dnsmasq. This is make-or-break for real-world usage.
Updated Phase Details
Phase 0: SPIKE — Validate Docker Networking
Timebox: 2-3 hours
Test script must verify:
- [ ] docker network connect --alias <service> <network> <container> works
- [ ] Container A in env network can reach shared container via service name
- [ ] Both Linux and macOS behaviors
- [ ] Clean disconnection/removal
---
Phase 1: Minimal Runtime Implementation
Update: cilo/pkg/runtime/docker/provider.go
// Connect with alias support - CRITICAL for inter-container DNS
func (p *Provider) ConnectContainerToNetwork(containerName, networkName, alias string) error
func (p *Provider) DisconnectContainerFromNetwork(containerName, networkName) error
func (p *Provider) GetContainerIP(containerName) (string, error)
func (p *Provider) ListContainersWithLabel(labelKey, labelValue) ([]string, error)
New: cilo/pkg/share/manager.go
type Manager struct {
    provider runtime.Provider
}
func (m *Manager) EnsureSharedService(serviceName string, composeFiles []string) (containerName, ip string, err error)
func (m *Manager) ConnectSharedServiceToEnvironment(serviceName, envName string) error // Uses --alias
func (m *Manager) DisconnectSharedServiceFromEnvironment(serviceName, envName string) error
func (m *Manager) GetSharedServiceIP(serviceName string) (string, error)
func (m *Manager) StopSharedServiceIfUnused(serviceName string) error // With 60s grace period
---
Phase 2: Data Model
Update: cilo/pkg/models/models.go
type SharedService struct {
    Name       string    `json:"name"`
    Container  string    `json:"container"`
    IP         string    `json:"ip"`
    Project    string    `json:"project"`
    Image      string    `json:"image"`
    VolumeHash string    `json:"volume_hash"`    // Hash of volume mounts
    CreatedAt  time.Time `json:"created_at"`
    UsedBy     []string  `json:"used_by"`        // "project/env" keys
    DisconnectTimeout time.Time `json:"disconnect_timeout,omitempty"` // Grace period
}
type Environment struct {
    // ... existing fields ...
    UsesSharedServices []string `json:"uses_shared_services,omitempty"`
}
Config Hash Strategy:
- Include: image (with tag), volume mounts, ports, command, entrypoint
- Exclude: environment variables (shared service runs with one config regardless)
---
Phase 3: CLI Integration
Update: cilo/cmd/run.go
runCmd.Flags().String("share", "", "Share comma-separated services")
runCmd.Flags().String("isolate", "", "Isolate comma-separated services (override labels)")
Merge logic:
// 1. Start with services labeled cilo.share: "true"
shared := compose.GetServicesWithLabel(composeFiles, "cilo.share")
// 2. Add from --share flag
shared = append(shared, parseFlag(shareFlag)...)
// 3. Remove from --isolate flag  
shared = filterOut(shared, parseFlag(isolateFlag))
Update: cilo/cmd/lifecycle.go
In up:
// After starting isolated services
shareMgr := share.NewManager(docker.NewProvider())
for _, svc := range sharedServices {
    container, ip, err := shareMgr.EnsureSharedService(svc, composeFiles)
    if err != nil {
        if IsVersionConflict(err) {
            fmt.Printf("⚠️  Warning: %s definition differs from shared instance. Using existing.\n", svc)
            // Continue with existing
        } else {
            return err
        }
    }
    if err := shareMgr.ConnectSharedServiceToEnvironment(svc, env.Name); err != nil {
        return err
    }
}
env.UsesSharedServices = sharedServices
In down:
shareMgr := share.NewManager(docker.NewProvider())
for _, svc := range env.UsesSharedServices {
    shareMgr.DisconnectSharedServiceFromEnvironment(svc, env.Name)
    // Reference counting + grace period handled internally
}
---
Phase 4: Compose Processing
Update: cilo/pkg/compose/compose.go
func Transform(env *models.Environment, baseFiles []string, overridePath, dnsSuffix string, sharedServices []string) error {
    // ... load services ...
    
    for _, name := range SortedServiceNames(services) {
        if contains(sharedServices, name) {
            // Skip: don't create this service in the environment
            continue
        }
        
        // Check if this service depends_on a shared service
        if service.DependsOn != nil {
            serviceOverrides[name]["depends_on"] = filterOut(service.DependsOn, sharedServices)
        }
        
        // ... rest of override generation
    }
}
Update: cilo/pkg/compose/loader.go
func GetServicesWithLabel(composeFiles []string, labelKey string) ([]string, error)
func ComputeConfigHash(service *ComposeService) string // For conflict detection
---
Phase 5: DNS & Status
Update: cilo/pkg/dns/render.go
- Add transparent DNS entries for shared services (same naming as isolated)
Update: cilo/cmd/commands.go (status command)
// In status output:
fmt.Fprintf(w, "NAME\tTYPE\tIP\tURL\t\n")
for _, service := range env.Services {
    svcType := "isolated"
    if contains(env.UsesSharedServices, service.Name) {
        svcType = "shared"
    }
    fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", service.Name, svcType, service.IP, service.URL)
}
---
Phase 6: Grace Period & Reference Counting
In cilo/pkg/share/manager.go:
func (m *Manager) DisconnectSharedServiceFromEnvironment(serviceName, envName string) error {
    // 1. Remove env from UsedBy list
    // 2. If UsedBy is now empty:
    //    - Set DisconnectTimeout = now + 60s
    //    - Start background goroutine to check after 60s
    //    - If still unused after timeout, stop container
}
func (m *Manager) EnsureSharedService(serviceName string, composeFiles []string) (string, string, error) {
    // 1. If shared service exists and is within grace period:
    //    - Clear DisconnectTimeout
    //    - Return existing
    // 2. If doesn't exist:
    //    - Create from compose definition
    //    - Set ConfigHash
}
---
Phase 7: Doctor Integration
Update: cilo/pkg/share/doctor.go (new file)
func CheckSharedServices(st *models.State) (orphaned, missing, conflicts []Issue, err error)
type Issue struct {
    Service string
    Type    string // "orphaned", "missing", "conflict"
    Detail  string
}
Checks:
- [ ] Orphaned: container running but reference count = 0
- [ ] Missing: reference count > 0 but container not running
- [ ] Conflicts: config hash differs between envs using same shared service
---
Summary of Key Decisions
| Aspect | Decision |
|--------|----------|
| Version conflicts | Warn and use existing (default A) |
| Grace period | 60 seconds, not configurable |
| Status visibility | cilo status shows isolated/shared column |
| Inverse flag | Add --isolate to override labels |
| Network aliases | Required for inter-container DNS |
| Config hash scope | Image + volumes + ports + command (no env vars) |
---
Open Question
Grace period implementation detail: Should the grace period timer be:
1. In-memory only (simple, lost on Cilo restart, but probably fine)
2. Persisted in state (survives restart, more complex)
My leaning is option 1 for v1 — if Cilo restarts during the grace period, the shared container just gets cleaned up on next cilo doctor run or when someone notices. The 60s window is short enough that restart during it is unlikely. What's your take, objectivley review from first principles no bias?