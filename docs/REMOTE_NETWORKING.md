# PRD: Cilo Cloud — Isolated Dev Environments, Locally or Remotely

## Status: Draft
## Author: Engineering
## Last Updated: 2026-02-07

---

## 1. What Cilo Is

Cilo creates isolated copies of your application — each with its own network, DNS, and state — and runs them locally or on remote infrastructure. One command, zero config changes.

```bash
cilo up agent-1                        # Local: isolated copy on your laptop
cilo cloud up agent-1 --from .         # Remote: isolated copy on a server
```

Both produce the same result: every service addressable by name (`api.agent-1.test`), fully isolated from every other environment. Your application code doesn't change. Your config files don't change. Cilo figures out the rest.

### What Cilo Does

1. **Detects** your project format (Compose, devcontainer, Procfile)
2. **Isolates** it (unique network, unique DNS, no port conflicts)
3. **Runs** it (locally via Docker/Podman, or remotely on any server)
4. **Addresses** every service by name (`<service>.<env>.test`)

### What Cilo Isn't

Cilo is not an IDE, a deployment platform, or a Kubernetes abstraction. It doesn't replace your editor, your CI pipeline, or your production infrastructure. It runs your dev environment — wherever you want — and gets out of the way.

---

## 2. Positioning

### The Problem

Developers need isolated copies of their application. For PR previews, for parallel AI agents, for onboarding, for testing. The existing tools either require Kubernetes knowledge (Coder, Shipyard), platform lock-in (Codespaces, Vercel), or only handle frontend (Netlify, Vercel).

Teams with 5-50 developers and no platform engineering team are underserved. They have a `docker-compose.yml` (or a `devcontainer.json`, or a `Procfile`) that works. They want to run it remotely without learning Terraform, Kubernetes, or a new platform config language.

### The Positioning

**Isolated dev environments for any project — locally or remotely.**

Not "simpler Coder." Not "preview environments." Not "agent infrastructure." All of those are use cases. The product is the isolation engine.

```
What you do with it:                 All the same operation:
├── PR preview environments          cilo cloud up pr-42 --from .
├── Parallel AI agent workspaces     cilo run claude agent-1 "task" --cloud
├── Onboard a new developer          cilo cloud up sarah-dev --from .
├── Offload heavy services           cilo cloud up ml-training --from .
└── Isolated CI test runs            cilo cloud up ci-$RUN_ID --ci --from .
```

### Why Cilo vs Alternatives

| | Cilo | Coder | Shipyard | Vercel |
|---|---|---|---|---|
| Input format | Your existing files (auto-detected) | Terraform templates | Docker Compose → K8s | Framework-specific |
| Local environments | Yes (core feature) | No | No | No |
| Remote environments | Yes | Yes | Yes | Yes (frontend only) |
| Requires K8s knowledge | No | Yes | Hidden but yes | No |
| Self-hostable | Yes (open source) | Yes (AGPL) | No | No |
| Platform team required | No | Effectively yes | No | No |
| Full-stack (DB, workers, etc.) | Yes | Yes | Yes | No |

**Cilo's unique position:** The only tool that does local + remote isolation from existing project files, without requiring Kubernetes, Terraform, or platform lock-in.

### Competitive Context

**Coder** ($76M raised, 50M+ downloads) is the incumbent in remote dev environments. Terraform-based, Kubernetes-native, built for platform teams at 500+ developer organizations. Cilo serves teams that don't have (and don't want) a platform team. Coder is the enterprise play; Cilo is the developer play. These audiences overlap at the margins but the core buyers are different.

**Shipyard** is the closest direct competitor for preview environments. They accept Docker Compose but transpile it to Kubernetes. Not self-hostable. Cilo runs Compose natively and can be self-hosted.

**Vercel/Netlify** own frontend previews. Cilo handles the full stack — databases, workers, queues, ML services — which Vercel can't.

**Bunnyshell/Qovery/Release** are full PaaS platforms with preview features. They require adopting their platform config. Cilo works with what you already have.

### Distribution: Local → Remote → Hosted

The adoption funnel is the product itself:

```
Free: Local isolation (existing Cilo CLI, open source)
  ↓ laptop runs out of resources / need to share environments
Self-host: Remote environments on your own servers
  ↓ don't want to manage servers / want pre-warmed pools
Hosted: cilocloud.dev manages everything
```

Each step is a natural response to a real pain point. The local experience is the top of the funnel. The hosted product is the monetization layer.

---

## 3. Architecture

### 3.1 Core Abstraction: Parser + Runtime

Cilo's engine is format-agnostic. It detects your project type, parses it into a universal environment spec, and runs it on the appropriate runtime. This is the architectural decision that prevents Compose lock-in.

```go
// pkg/engine/spec.go — The universal environment description

type EnvironmentSpec struct {
    Name     string
    Services []ServiceSpec
    Networks []NetworkSpec
    Volumes  []VolumeSpec
    Source   string          // "compose", "devcontainer", "procfile"
}

type ServiceSpec struct {
    Name       string
    Image      string              // Container image
    Build      *BuildSpec          // Build context (if no pre-built image)
    Command    []string
    Ports      []PortSpec
    Env        map[string]string
    Volumes    []VolumeMountSpec
    DependsOn  []string
    HealthCheck *HealthCheckSpec
}
```

```go
// pkg/engine/parser.go — Pluggable format detection

type Parser interface {
    Detect(projectPath string) bool
    Parse(projectPath string) (*EnvironmentSpec, error)
}

// Detection order: most specific → least specific
var Parsers = []Parser{
    &ComposeParser{},        // docker-compose.yml, compose.yml
    &DevcontainerParser{},   // .devcontainer/devcontainer.json
    &ProcfileParser{},       // Procfile
}

func DetectAndParse(projectPath string) (*EnvironmentSpec, error) {
    for _, p := range Parsers {
        if p.Detect(projectPath) {
            return p.Parse(projectPath)
        }
    }
    return nil, fmt.Errorf("no supported project format found in %s", projectPath)
}
```

```go
// pkg/engine/runtime.go — Pluggable container runtime

type Runtime interface {
    Up(spec *EnvironmentSpec, env string) error
    Down(env string) error
    Destroy(env string) error
    Status(env string) ([]ServiceStatus, error)
    Logs(env string, service string) (io.ReadCloser, error)
}

// Implementations
type DockerRuntime struct{}    // Docker Engine + Compose plugin
type PodmanRuntime struct{}    // Podman + podman-compose
```

**Auto-detection in practice:**

```bash
cilo up agent-1
# Found docker-compose.yml → ComposeParser + DockerRuntime ✓

cilo up agent-1
# Found .devcontainer/devcontainer.json → DevcontainerParser + DockerRuntime ✓

cilo up agent-1
# Found Procfile → ProcfileParser + DockerRuntime (auto-containerize) ✓
```

### 3.2 Format Support Roadmap

| Format | Parser | Phase | Market |
|--------|--------|-------|--------|
| Docker Compose | `ComposeParser` | **Now (exists)** | Largest installed base for local dev |
| Podman runtime | `PodmanRuntime` | **Phase 2** | Enterprise (Red Hat, Docker licensing) |
| Devcontainer | `DevcontainerParser` | **Phase 4** | Codespaces/Cursor users, growing fast |
| Procfile | `ProcfileParser` | **Phase 6+** | Non-containerized apps, widest reach |

Compose is first because it's the largest market and Cilo already supports it. But the architecture doesn't trap us there. Adding a new format means implementing one interface.

### 3.3 Monorepo Structure

```
github.com/sharedco/cilo/
├── cilo/                          # CLI (MIT)
│   ├── cmd/
│   │   ├── root.go
│   │   ├── up.go                   # cilo up (local)
│   │   ├── run.go
│   │   ├── lifecycle.go
│   │   ├── cloud.go                # cilo cloud subcommand group
│   │   ├── cloud_login.go
│   │   ├── cloud_up.go
│   │   ├── cloud_down.go
│   │   ├── cloud_destroy.go
│   │   ├── cloud_status.go
│   │   ├── cloud_connect.go        # Multi-user environment access
│   │   └── cloud_logs.go
│   └── pkg/
│       ├── engine/                  # NEW: core abstraction layer
│       │   ├── spec.go              # EnvironmentSpec (universal format)
│       │   ├── parser.go            # Parser interface + auto-detection
│       │   ├── runtime.go           # Runtime interface
│       │   └── detect.go            # Project format detection
│       ├── parsers/                 # NEW: format-specific parsers
│       │   ├── compose.go           # ComposeParser (wraps existing compose pkg)
│       │   ├── devcontainer.go      # DevcontainerParser (Phase 4)
│       │   └── procfile.go          # ProcfileParser (Phase 6+)
│       ├── runtimes/                # NEW: runtime implementations
│       │   ├── docker.go            # DockerRuntime (wraps existing runtime pkg)
│       │   └── podman.go            # PodmanRuntime (Phase 2)
│       ├── compose/                 # Existing compose logic
│       ├── dns/                     # Existing DNS logic
│       ├── models/                  # Existing models
│       ├── runtime/                 # Existing Docker runtime
│       ├── state/                   # Existing state management
│       ├── share/                   # Shared services (separate feature)
│       └── cloud/                   # Cloud client package
│           ├── client.go
│           ├── auth.go
│           ├── sync.go
│           ├── tunnel.go
│           ├── privilege.go
│           └── dns.go
│
├── server/                         # Cloud API server (BSL 1.1)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── pkg/
│   │   ├── api/
│   │   │   ├── server.go
│   │   │   ├── routes.go
│   │   │   └── handlers/
│   │   │       ├── auth.go
│   │   │       ├── environments.go
│   │   │       ├── machines.go
│   │   │       ├── wireguard.go
│   │   │       └── status.go
│   │   ├── auth/
│   │   │   ├── apikey.go
│   │   │   └── store.go
│   │   ├── vm/
│   │   │   ├── pool.go
│   │   │   ├── provider.go         # Cloud provider interface
│   │   │   ├── manual.go           # Manual/SSH provider (Phase 1)
│   │   │   ├── hetzner.go          # Hetzner provider (Phase 1)
│   │   │   └── health.go
│   │   ├── wireguard/
│   │   │   ├── keys.go
│   │   │   ├── exchange.go         # Multi-peer support
│   │   │   └── config.go
│   │   ├── billing/                # Only when CILO_BILLING_ENABLED=true
│   │   │   ├── meter.go
│   │   │   ├── pricing.go
│   │   │   └── store.go
│   │   └── store/
│   │       ├── store.go
│   │       ├── postgres.go
│   │       └── migrations/
│   └── go.mod
│
├── agent/                          # Machine-side daemon (MIT)
│   ├── cmd/
│   │   └── agent/
│   │       └── main.go
│   ├── pkg/
│   │   ├── agent/
│   │   │   ├── server.go           # HTTP server (WireGuard interface only)
│   │   │   ├── workspace.go
│   │   │   ├── environment.go      # Uses engine.Parser + engine.Runtime
│   │   │   ├── health.go
│   │   │   └── wireguard.go        # Multi-peer WireGuard config
│   │   └── reporter/
│   │       └── reporter.go
│   └── go.mod
│
├── action/                         # GitHub Actions (MIT)
│   ├── action.yml
│   ├── entrypoint.sh
│   └── README.md
│
├── packer/                         # VM image build (MIT)
│   ├── cilo-vm.pkr.hcl
│   ├── scripts/
│   └── README.md
│
├── deploy/
│   ├── self-host/
│   │   ├── docker-compose.yml
│   │   ├── .env.example
│   │   ├── caddy/
│   │   │   └── Caddyfile
│   │   └── README.md
│   └── hosted/
│       ├── docker-compose.yml
│       └── terraform/
│
├── docs/
│   ├── SELF_HOSTING.md
│   ├── SELF_HOST_OPERATIONS.md
│   ├── API_REFERENCE.md
│   ├── ARCHITECTURE.md
│   ├── SUPPORTED_FORMATS.md        # Compose, devcontainer, Procfile
│   ├── SECURITY.md
│   └── CONTRIBUTING.md             # Community contribution guide
│
└── LICENSE
```

### 3.4 System Components

```
Developer's Machine                 Self-Hosted or cilocloud.dev
┌──────────────────┐               ┌──────────────────────────┐
│ cilo CLI         │               │ cilo-server              │
│ ┌──────────────┐ │               │  ├── Auth (API keys)     │
│ │ Parser:      │ │───HTTPS──────▶│  ├── Machine Pool        │
│ │  auto-detect │ │               │  ├── WG Key Exchange     │
│ │ Runtime:     │ │               │  └── Metering*           │
│ │  docker/     │ │               │         │                │
│ │  podman      │ │               │         ▼                │
│ └──────────────┘ │               │  Machines                │
│                  │               │  ┌──────────────────┐   │
│ WireGuard ───────┼───encrypted──▶│  │ cilo-agent       │   │
│ tunnel           │    tunnel     │  │  ├── Parser       │   │
│                  │               │  │  ├── Runtime      │   │
│ dnsmasq:         │               │  │  ├── WireGuard    │   │
│  *.a1.test       │               │  │  └── rsync        │   │
│   → remote IPs   │               │  └──────────────────┘   │
└──────────────────┘               └──────────────────────────┘
                                   * Metering optional for self-host
```

**cilo CLI** — Detects project format, parses into EnvironmentSpec, runs locally via Docker/Podman or delegates to cilo-server for remote execution. Manages WireGuard tunnels and local DNS.

**cilo-server** — Go HTTP service. Authenticates via API key, manages machine pool (or static machine list), handles WireGuard key exchange (multi-peer), meters usage optionally. Self-hostable with `docker compose up`.

**cilo-agent** — Lightweight daemon on each machine. Receives workspaces via rsync, uses the same Parser + Runtime engine as the CLI to run environments. Reports container IPs and health. Listens only on WireGuard interface.

### 3.5 Two Machine Providers (Both Phase 1)

#### Manual/SSH — "Point at Machines You Already Have"

For self-hosting teams with existing servers. No cloud API needed.

```bash
# Register existing machines
docker compose exec server cilo-server machines add \
  --name build-1 --host 192.168.1.100 --ssh-user deploy
```

cilo-server manages assignment and installs cilo-agent over SSH. Doesn't provision or destroy machines — they're static.

```go
// server/pkg/vm/manual.go

type ManualProvider struct{}

func (m *ManualProvider) Provision(config ProvisionConfig) (*Machine, error) {
    return nil, fmt.Errorf("manual provider: register machines with 'cilo-server machines add'")
}
func (m *ManualProvider) SetupAgent(machine *Machine) error     // Install agent via SSH
func (m *ManualProvider) HealthCheck(id string) (bool, error)
func (m *ManualProvider) Destroy(id string) error { return nil } // No-op: machines are persistent
```

#### Cloud Provider — "Cilo Provisions Machines For You"

Auto-provisioning. Hetzner first (cheapest), AWS later (enterprise demand).

```go
// server/pkg/vm/provider.go

type Provider interface {
    Provision(config ProvisionConfig) (*Machine, error)
    Destroy(providerID string) error
    List() ([]*Machine, error)
    HealthCheck(providerID string) (bool, error)
}
```

### 3.6 Networking: WireGuard with Multi-Peer

Each connection uses a point-to-point WireGuard tunnel. Multiple developers can connect to the same environment (required for PR previews where reviewers need access).

```
Developer A (10.225.0.1) ──┐
                            ├── WireGuard ──→ Machine (10.225.0.100)
Developer B (10.225.0.2) ──┘                  │
                                               ├── api       (10.224.1.3)
                                               ├── postgres  (10.224.1.4)
                                               └── redis     (10.224.1.5)
```

Key exchange supports adding/removing peers without disrupting existing connections:

```go
// server/pkg/wireguard/exchange.go

type PeerRegistration struct {
    EnvironmentID string
    UserID        string
    PublicKey     string
    AssignedIP    string    // Unique per peer: 10.225.0.1, 10.225.0.2, etc.
}

func (e *Exchange) AddPeer(machineID string, peer PeerRegistration) error
func (e *Exchange) RemovePeer(machineID string, publicKey string) error
```

### 3.7 WireGuard Privilege Model

Addressed upfront, not deferred.

**Phase 1:** Explicit sudo with clear messaging.

```go
// cilo/pkg/cloud/privilege.go

func CheckWireGuardPrivileges() error {
    if os.Geteuid() != 0 {
        return fmt.Errorf(
            "WireGuard tunnel requires elevated privileges.\n\n" +
            "Run with sudo:\n" +
            "  sudo cilo cloud up agent-1 --from .\n\n" +
            "Why: Creates a WireGuard interface to route traffic to your\n" +
            "remote environment. Requires CAP_NET_ADMIN.\n\n" +
            "See: https://docs.cilo.dev/security#wireguard-privileges")
    }
    return nil
}
```

**Phase 3:** Setuid helper installed during `sudo cilo init` (which already requires sudo for DNS). Minimal binary that only handles WireGuard interface operations. After installation, `cilo cloud up` no longer needs sudo.

**CI mode:** Bypasses WireGuard entirely. Runner accesses VM via public IP with scoped firewall rules. No privilege issues.

### 3.8 DNS Resolution

Local dnsmasq resolves environment DNS names to container IPs, routed through WireGuard for remote environments:

```
curl http://api.agent-1.test
  → dnsmasq: api.agent-1.test → 10.224.1.3
  → routed through WireGuard tunnel (remote) or Docker network (local)
  → hits the api container directly
```

Core invariant: `<service>.<env>.test` always resolves, regardless of where the container runs, what format the project was defined in, or what runtime executes it.

---

## 4. Self-Hosting: Setup and Ongoing Operations

Self-hosting is first-class. The PRD is honest about what's easy and what's ongoing work.

### Initial Setup

```bash
cd deploy/self-host
cp .env.example .env
# Edit .env
docker compose up -d

# Create first API key
docker compose exec server cilo-server admin create-key --scope admin --name "my-key"
```

With Manual/SSH provider (no cloud API needed):
```bash
docker compose exec server cilo-server machines add \
  --name build-1 --host 192.168.1.100 --ssh-user deploy
```

With Hetzner (auto-provisioning):
```bash
# In .env: HETZNER_API_TOKEN=your-token
cd ../../packer && packer build cilo-vm.pkr.hcl
```

Setup time: ~30 minutes (Manual), ~1 hour (Hetzner, includes image build).

### Ongoing Operational Burden

| Task | Frequency | Effort |
|------|-----------|--------|
| cilo-server updates | Monthly | Pull new image, restart. ~5 min. |
| VM image updates | Monthly | Rebuild Packer image, rotate VMs. ~30 min. |
| Database backups | Weekly | Standard pg_dump cron. |
| Monitoring integration | One-time | `/metrics` endpoint, connect to your Prometheus. |
| Pool scaling | As needed | Adjust env vars in .env, restart. |
| Security patches | As announced | Update server, rebuild images. |
| Disk cleanup | Monthly | `cilo doctor --fix` on VMs. |

**The honest tradeoff:** Self-hosting costs 2-4 hours/month of DevOps time. For a team paying a DevOps engineer $150k/year, 3 hours/month ≈ $275/month in labor — comparable to the hosted product's team tier. The hosted product is worth it when teams prefer to spend that time on product work.

### Configuration

```bash
# deploy/self-host/.env.example

# Required
POSTGRES_URL=postgres://cilo:password@postgres:5432/cilo
CILO_DOMAIN=cilo.internal.company.com

# Machine Provider
CILO_PROVIDER=manual                # or "hetzner"
HETZNER_API_TOKEN=                  # Only if provider=hetzner

# Pool (cloud providers only)
CILO_POOL_MIN_READY=3
CILO_POOL_MAX_TOTAL=20
CILO_POOL_VM_SIZE=cx31

# Optional
CILO_BILLING_ENABLED=false
CILO_AUTO_DESTROY_HOURS=8
CILO_METRICS_ENABLED=true
```

---

## 5. Data Models

```go
// server/pkg/store/models.go

type Team struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
}

type APIKey struct {
    ID        string    `db:"id"`
    TeamID    string    `db:"team_id"`
    KeyHash   string    `db:"key_hash"`      // bcrypt, never exposed
    Prefix    string    `db:"prefix"`        // First 8 chars for identification
    Scope     string    `db:"scope"`         // "admin", "developer", "ci"
    Name      string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
    LastUsed  time.Time `db:"last_used"`
}

type Environment struct {
    ID          string    `db:"id"`
    TeamID      string    `db:"team_id"`
    Name        string    `db:"name"`         // "agent-1", "pr-42"
    Project     string    `db:"project"`
    Format      string    `db:"format"`       // "compose", "devcontainer", "procfile"
    MachineID   string    `db:"machine_id"`
    Status      string    `db:"status"`
    Subnet      string    `db:"subnet"`
    Services    []Service                      // JSONB
    Peers       []Peer                         // JSONB — multiple users can connect
    CreatedAt   time.Time `db:"created_at"`
    CreatedBy   string    `db:"created_by"`
    Source      string    `db:"source"`        // "cli", "ci", "api"
}

// Status: "provisioning" → "syncing" → "running" → "stopped" → "destroying" → "destroyed"

type Service struct {
    Name string `json:"name"`
    IP   string `json:"ip"`
    Port int    `json:"port"`
}

type Peer struct {
    UserID      string    `json:"user_id"`
    WGPublicKey string    `json:"wg_public_key"`
    AssignedIP  string    `json:"assigned_ip"`
    ConnectedAt time.Time `json:"connected_at"`
}

type Machine struct {
    ID           string    `db:"id"`
    ProviderID   string    `db:"provider_id"`
    ProviderType string    `db:"provider_type"`  // "manual", "hetzner", "aws"
    PublicIP     string    `db:"public_ip"`
    WGPublicKey  string    `db:"wg_public_key"`
    WGEndpoint   string    `db:"wg_endpoint"`
    Status       string    `db:"status"`
    AssignedEnv  string    `db:"assigned_env"`
    SSHHost      string    `db:"ssh_host"`       // Manual provider
    SSHUser      string    `db:"ssh_user"`
    Region       string    `db:"region"`
    Size         string    `db:"size"`
    CreatedAt    time.Time `db:"created_at"`
}

// Machine Status: "provisioning" → "ready" → "assigned" → "draining" → "destroying"
// Manual provider: "registering" → "ready" → "assigned" (never "destroying")

type UsageRecord struct {
    ID            string    `db:"id"`
    TeamID        string    `db:"team_id"`
    EnvironmentID string    `db:"environment_id"`
    StartTime     time.Time `db:"start_time"`
    EndTime       time.Time `db:"end_time"`
    DurationSec   int64     `db:"duration_sec"`
}
```

---

## 6. API Endpoints

```
# Authentication
POST   /v1/auth/keys                # Create API key
DELETE /v1/auth/keys/:id             # Revoke
GET    /v1/auth/keys                 # List (admin only)

# Environments
POST   /v1/environments              # Create environment
GET    /v1/environments               # List
GET    /v1/environments/:id           # Details
DELETE /v1/environments/:id           # Destroy
POST   /v1/environments/:id/sync     # Signal workspace sync complete

# Networking
POST   /v1/wireguard/exchange         # Exchange keys (supports multiple peers)
DELETE /v1/wireguard/peers/:key       # Remove peer
GET    /v1/wireguard/status/:id       # Tunnel status

# Machines (Manual provider)
POST   /v1/machines                   # Register
DELETE /v1/machines/:id               # Remove
GET    /v1/machines                   # List

# Operations
GET    /v1/status                     # Instance status
GET    /v1/health                     # Health check
GET    /metrics                       # Prometheus (opt-in)
```

---

## 7. Implementation Plan

### Phase 1: cilo-server + Parser/Runtime Abstraction

**Goal:** Working self-hostable server. Parser and Runtime interfaces defined (Compose is the only implementation, but the abstraction exists).

**Deliverables:**
- cilo-server with auth, environment CRUD, machine pool
- Manual/SSH provider (register existing machines, install agent via SSH)
- Hetzner provider (auto-provision VMs, pool management)
- Parser interface with ComposeParser implementation
- Runtime interface with DockerRuntime implementation
- Self-host Docker Compose setup (`deploy/self-host/`)
- PostgreSQL migrations, API key management

**Server pool manager:**

```go
// server/pkg/vm/pool.go

type PoolConfig struct {
    MinReady int    `env:"CILO_POOL_MIN_READY" default:"3"`
    MaxTotal int    `env:"CILO_POOL_MAX_TOTAL" default:"20"`
    VMSize   string `env:"CILO_POOL_VM_SIZE" default:"cx31"`
    Region   string `env:"CILO_POOL_REGION" default:"nbg1"`
    ImageID  string `env:"CILO_POOL_IMAGE_ID"`
}

func (p *Pool) AssignMachine(envID string) (*Machine, error)
func (p *Pool) ReleaseMachine(machineID string) error
func (p *Pool) Reconcile(ctx context.Context) error               // Cloud providers only
func (p *Pool) RegisterMachine(config ManualMachineConfig) error   // Manual provider
```

---

### Phase 2: cilo-agent + Podman Runtime

**Goal:** Machine-side daemon + Podman as alternative runtime.

**Deliverables:**
- cilo-agent binary (uses same Parser + Runtime as CLI)
- Agent API (WireGuard interface only): `/compose/up`, `/compose/down`, `/compose/status`, `/health`, `/wireguard/add-peer`
- Packer template for VM images
- Agent installation via SSH for Manual provider
- PodmanRuntime implementation

**Agent uses the engine abstraction:**

```go
// agent/pkg/agent/environment.go

import "github.com/sharedco/cilo/cilo/pkg/engine"

func (a *Agent) Up(workspacePath string, envName string) error {
    spec, err := engine.DetectAndParse(workspacePath)
    if err != nil {
        return err
    }
    return a.runtime.Up(spec, envName)
}
```

**Why Podman in Phase 2:** Enterprise teams that can't use Docker (licensing, security policy) get Cilo immediately. The Runtime interface is already defined in Phase 1 — Podman is just a second implementation. Minimal incremental cost, real market expansion.

**VM image contents:** Ubuntu 24.04, Docker Engine + Compose plugin, Podman + podman-compose, WireGuard, cilo-agent, rsync, pre-pulled common images (postgres:16-alpine, redis:7-alpine, nginx:alpine, node:20-alpine).

---

### Phase 3: CLI Cloud Extensions + WireGuard Helper

**Goal:** `cilo cloud` subcommands + setuid helper for better UX.

**Commands:**

```bash
# Auth (server URL is always explicit)
cilo cloud login --server https://cilo.company.com
cilo cloud login --server https://api.cilocloud.dev
cilo cloud login --stdin                                  # CI

# Environment lifecycle
cilo cloud up <env> --from <path>
cilo cloud down <env>
cilo cloud destroy <env>
cilo cloud sync <env>

# Multi-user access
cilo cloud connect <env>

# One-shot agent execution
cilo run claude agent-1 "implement auth" --cloud

# Information
cilo cloud status
cilo cloud logs <env> [service]
```

**WireGuard helper:** Installed during `sudo cilo init`. Minimal setuid binary — only creates/destroys WireGuard interfaces and manages peers. After installation, `cilo cloud up` works without sudo.

**Auth storage:**

```json
// ~/.cilo/cloud-auth.json (0600)
{
  "server": "https://cilo.internal.company.com",
  "api_key": "cilo_key_abc123...",
  "team_id": "team_xyz"
}
```

---

### Phase 4: CI/CD Integration + Devcontainer Parser

**Goal:** GitHub Actions for PR previews. Devcontainer support broadens the addressable market.

**GitHub Action:**

```yaml
# action/action.yml
name: 'Cilo Environment'
inputs:
  server:
    description: 'Cilo server URL'
    required: true
  api-key:
    required: true
  environment-name:
    required: false
  project-path:
    default: '.'
  action:
    default: 'create'
  timeout:
    default: '60'
outputs:
  environment-url:
  environment-id:
```

**CI mode (`--ci`):** No WireGuard. Direct IP access with scoped firewall rules. Auto-destroy after timeout.

**PR lifecycle:**

```yaml
name: Preview Environment
on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

jobs:
  preview:
    if: github.event.action != 'closed'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: sharedco/cilo-action@v1
        id: preview
        with:
          server: ${{ vars.CILO_SERVER }}
          api-key: ${{ secrets.CILO_API_KEY }}
      - uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `🚀 Preview ready: ${{ steps.preview.outputs.environment-url }}`
            })

  cleanup:
    if: github.event.action == 'closed'
    runs-on: ubuntu-latest
    steps:
      - uses: sharedco/cilo-action@v1
        with:
          server: ${{ vars.CILO_SERVER }}
          api-key: ${{ secrets.CILO_API_KEY }}
          action: destroy
```

**DevcontainerParser:** Reads `.devcontainer/devcontainer.json`, converts to EnvironmentSpec. Handles single-container devcontainers, multi-container (via compose reference), and features.

```go
// cilo/pkg/parsers/devcontainer.go

type DevcontainerParser struct{}

func (d *DevcontainerParser) Detect(path string) bool {
    _, err := os.Stat(filepath.Join(path, ".devcontainer", "devcontainer.json"))
    return err == nil
}

func (d *DevcontainerParser) Parse(path string) (*engine.EnvironmentSpec, error) {
    config := readDevcontainerJSON(path)

    // If devcontainer.json references a docker-compose.yml, delegate
    if config.DockerComposeFile != "" {
        return (&ComposeParser{}).Parse(path)
    }

    // Single-container devcontainer → single-service EnvironmentSpec
    return &engine.EnvironmentSpec{
        Services: []engine.ServiceSpec{{
            Name:    "dev",
            Image:   config.Image,
            Build:   parseBuildConfig(config),
            Command: config.PostCreateCommand,
            Ports:   parseForwardPorts(config),
        }},
    }, nil
}
```

This captures the growing Codespaces/Cursor audience. Projects with a `devcontainer.json` work with Cilo automatically — no migration needed.

---

### Phase 5: Hosted Product Launch

**Goal:** Launch cilocloud.dev with billing. CLI-only, no dashboard.

**Why no dashboard yet:** At the scale of 10-20 early teams, `cilo cloud status` is sufficient. Engineering time goes to reliability and performance. Build the dashboard when 5+ paying teams request it.

**Billing:**

```go
// server/pkg/billing/meter.go — Only active when CILO_BILLING_ENABLED=true

func (m *Meter) Run(ctx context.Context) {
    if !m.enabled { return }
    ticker := time.NewTicker(60 * time.Second)
    for {
        select {
        case <-ticker.C:
            for _, env := range m.store.ListRunningEnvironments() {
                m.store.RecordUsage(UsageRecord{
                    TeamID: env.TeamID, EnvironmentID: env.ID,
                    StartTime: time.Now().Add(-60*time.Second), EndTime: time.Now(),
                    DurationSec: 60,
                })
            }
        case <-ctx.Done():
            return
        }
    }
}
```

**Pricing:**

```go
var DefaultTiers = []Tier{
    {Name: "trial", IncludedHours: 100, MaxConcurrent: 3, TrialDays: 14},
    {Name: "team", MonthlyBase: 4900, IncludedHours: 500, OveragePerHour: 20, MaxConcurrent: 20},
    {Name: "enterprise", OveragePerHour: 15, MaxConcurrent: 100},  // Custom
}
```

**Why trial, not free tier:** A perpetual free tier consumes real VM resources with zero revenue. At early scale with low utilization, idle VMs for free users destroy margins. A 14-day trial qualifies serious buyers. Teams that don't convert can self-host.

**AWS provider (if enterprise demand):** Some paying teams will require AWS. Add here if needed.

---

### Phase 6: Dashboard + Procfile Parser

**Goal:** Dashboard when customers request it. Procfile support for non-containerized apps.

**Dashboard gate:** Only build when 5+ paying teams have asked.

**Dashboard views:**
1. **Environments** — Status, creator, duration, format. Stop/destroy.
2. **Environment Detail** — Services, IPs, DNS, streamed logs.
3. **Usage** — Hours by user, project, time period. CSV export.
4. **Settings** — API keys, team config, pool settings.

**ProcfileParser:** Auto-containerizes process-based apps.

```go
// cilo/pkg/parsers/procfile.go

type ProcfileParser struct{}

func (p *ProcfileParser) Parse(path string) (*engine.EnvironmentSpec, error) {
    procs := parseProcfile(filepath.Join(path, "Procfile"))
    spec := &engine.EnvironmentSpec{}
    for name, command := range procs {
        spec.Services = append(spec.Services, engine.ServiceSpec{
            Name:    name,
            Build:   autoDetectBuild(path),  // Detect runtime, generate Dockerfile
            Command: strings.Fields(command),
        })
    }
    return spec, nil
}

// autoDetectBuild inspects the project for package.json, requirements.txt,
// Gemfile, go.mod, etc. and generates an appropriate build context.
func autoDetectBuild(path string) *engine.BuildSpec { ... }
```

---

## 8. Data Flow

### `cilo cloud up agent-1 --from .`

```
Step 1: CLI detects project format
  engine.DetectAndParse(".") → EnvironmentSpec (compose, 4 services)

Step 2: CLI → cilo-server
  POST /v1/environments { name: "agent-1", project: "myapp", format: "compose" }
  ← { id: "env-abc", machine: { public_ip, wg_endpoint, wg_public_key } }

Step 3: CLI — WireGuard
  Generate keypair → POST /v1/wireguard/exchange { env_id, client_public_key }
  Configure local interface via helper (no sudo needed after Phase 3)
  Add route: 10.224.1.0/24 via cilo0

Step 4: CLI → Machine (via tunnel)
  rsync workspace to 10.225.0.100:~/.cilo/envs/myapp/agent-1/

Step 5: CLI → cilo-agent (via tunnel)
  POST http://10.225.0.100:8080/environment/up

Step 6: cilo-agent
  engine.DetectAndParse(workspace) → EnvironmentSpec
  runtime.Up(spec, "agent-1") → containers running
  Report service IPs to cilo-server

Step 7: CLI ← cilo-server
  GET /v1/environments/env-abc → { services: [...], peers: [...] }

Step 8: CLI — DNS
  dnsmasq: api.agent-1.test → 10.224.1.3

Step 9: CLI → stdout
  "Environment agent-1 is ready (compose, 4 services)"
  "  api:      http://api.agent-1.test"
  "  nginx:    http://nginx.agent-1.test"
  "  postgres: postgres.agent-1.test:5432"
```

### `cilo cloud connect agent-1` (Second User)

```
Step 1: CLI → cilo-server
  POST /v1/wireguard/exchange { env_id: "env-abc", client_public_key: "new-key" }
  ← { assigned_ip: "10.225.0.2", machine_wg_public_key, wg_endpoint }

Step 2: cilo-server → cilo-agent
  POST /wireguard/add-peer { public_key: "new-key", assigned_ip: "10.225.0.2" }

Step 3: CLI — WireGuard + DNS
  Configure interface, same DNS entries as original creator.

Result: Second developer sees identical environment at identical DNS names.
```

---

## 9. Security

**Authentication:** API keys scoped to `admin`, `developer`, `ci`. bcrypt hashed. All requests require `Authorization: Bearer <key>`.

**Network:** WireGuard end-to-end encryption. cilo-agent listens only on WireGuard interface. Machines firewalled to WireGuard (51820/UDP) + SSH (22/TCP, server management only).

**Isolation:** One environment per machine (v1). Docker/Podman networks isolated per environment. Machine wiped between assignments (cloud) or environment directory cleaned (Manual).

**Secrets:** Developer keys in `~/.cilo/cloud-auth.json` (0600). Server credentials via environment variables. No secrets in VM images.

**Privilege escalation:** WireGuard helper is a minimal setuid binary with strict input validation. Only handles interface operations. Source available for audit.

---

## 10. Operations

**Auto-Cleanup:** Idle environments destroyed after `CILO_AUTO_DESTROY_HOURS` (default: 8). CI environments after `--timeout` (default: 60 min). Cloud VMs recycled within 60 seconds. Manual machines cleaned but not destroyed.

**Failure Recovery:** Machine unhealthy → mark degraded, notify user. Server down → existing tunnels keep working. Tunnel drops → WireGuard auto-reconnects.

**Monitoring:** Prometheus metrics on `/metrics`. API latency/errors, machine pool utilization, active tunnels, environment creation rate, average lifetime, format distribution (compose vs devcontainer vs procfile).

---

## 11. Success Metrics

| Phase | Metric | Target |
|-------|--------|--------|
| 1-3 | `cilo cloud up` to DNS-accessible | < 60 seconds |
| 1-3 | Workspace sync (< 500MB) | < 30 seconds |
| 1-3 | Self-host setup (Manual provider) | < 30 minutes |
| 1-3 | Self-hosted teams validating | 5+ |
| 2 | Podman runtime parity with Docker | Feature-complete |
| 4 | GitHub Action preview creation | < 90 seconds |
| 4 | Devcontainer projects auto-detected | 100% of standard configs |
| 5 | Trial → paid conversion | > 20% |
| 5 | Paying teams within 3 months | 10+ |
| 5 | Weekly environment-hours | 1,000+ |
| 6 | Dashboard requested by paying teams | 5+ before building |

---

## 12. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Docker Compose usage declines among early adopters | Shrinking primary market | Parser abstraction enables devcontainer + Procfile without refactoring |
| Coder ships `--from-compose` flag | Direct feature parity on remote Compose | Local + remote isolation is a deeper moat; Coder's Terraform architecture makes Compose a bolt-on, not native |
| AI agent market takes 2+ years | Growth play delayed | Lead with general isolation value; agents are a use case, not the product |
| Self-hosting too easy → low hosted conversion | Revenue ceiling | Operational burden is honest in docs; hosted value is pre-warmed pools + zero-config + scale management |
| Small team operating distributed platform | Reliability, burnout | Manual provider reduces infra to manage; phased launch limits blast radius |
| Podman ecosystem less mature than Docker | Runtime parity issues | Docker remains primary; Podman is additive for enterprise |
| Devcontainer spec evolves rapidly | Parser maintenance burden | Track spec changes; community contributions help |

---

## 13. Community Strategy

The moat isn't network effects (Cilo has none). It's community. The product is only defensible if contributors build around the format-agnostic workflow.

**What gets contributed:**
- New parsers (format support for new project types)
- New runtime implementations (containerd, etc.)
- Cloud provider plugins (DigitalOcean, Vultr, OVH)
- CI/CD integrations (GitLab CI, Bitbucket Pipelines, CircleCI)
- Compose optimizations (image caching, layer sharing)

**Where contributors engage:**
- GitHub Issues + Discussions (primary)
- Discord (real-time help, feature discussion)
- `CONTRIBUTING.md` with clear "good first issue" labels

**Governance:**
- Parser and Runtime interfaces are stable APIs — breaking changes require RFC
- Community parsers can live in-tree or as external plugins
- Core team maintains Compose, Podman, Devcontainer; community maintains the rest

**What keeps community loyal:**
- BSL → Apache conversion means the code is eventually fully open
- MIT license on CLI and agent removes all friction
- Self-hosting works without any vendor dependency
- Parser/Runtime plugin architecture means contributors own their code

---

## 14. Open Decisions

| Decision | Recommendation | Resolve By |
|----------|----------------|------------|
| One machine per env vs multi-tenant | Single-tenant v1 | Phase 1 |
| WireGuard helper implementation | Setuid binary, security audit before release | Phase 3 |
| Cloud providers | Hetzner + Manual v1. AWS when enterprise demands. | Phase 1 |
| Database | PostgreSQL | Phase 1 |
| Preview URL domain | Configurable `CILO_DOMAIN`. Hosted uses `cilocloud.dev`. | Phase 4 |
| Dashboard framework | React SPA. Only build when 5+ teams request. | Phase 6 |
| Procfile auto-containerization strategy | Nixpacks vs custom Dockerfile generation | Phase 6 |
| Devcontainer feature support depth | Core features only vs full spec | Phase 4 |

---

## 15. Out of Scope

- **Mesh networking / Headscale / DERP** — Point-to-point WireGuard is sufficient.
- **Hybrid local + remote** — Separate roadmap. Requires shared services first.
- **Browser IDE / terminal** — Not a Coder/Codespaces competitor.
- **Continuous file sync** — Sync is explicit. Use Mutagen alongside if needed.
- **Container migration** — Destroy and recreate.
- **Multi-region** — Single region v1.
- **Dashboard before Phase 6** — CLI and API are sufficient for early customers.
- **Kubernetes as input format** — Directly competes with Coder/Tilt/Skaffold. Avoid.
- **Terraform as input format** — This is Coder's core. Not our territory.
- **Nix/devenv support** — Process-level isolation is architecturally different. Revisit if demand materializes.