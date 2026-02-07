# Remote & Hybrid Environments

Run containers locally or remotely — transparently. Your code doesn't know the difference.

---

## Three Operating Modes

Cilo environments can run in three modes. Each is independently useful, and they build on each other:

```
Full Local (default)     Everything runs on your machine.
                         What Cilo does today.

Full Remote              Everything runs on a remote host.
                         Your laptop just handles DNS and your editor.
                         Simplest remote case — no network bridging needed.

Hybrid                   Some services local, some remote.
                         The hardest case, but the most powerful.
```

### Full Remote — Offload Everything

Your laptop is out of RAM. You're running 10 agent environments. You have a beefy staging server sitting idle. Just move the whole thing:

```bash
# Entire environment runs on staging
cilo run opencode agent-1 "do the thing" --remote staging

# All DNS still resolves from your laptop:
# api.agent-1.test → staging
# db.agent-1.test → staging
# redis.agent-1.test → staging
```

This is the simplest remote case because all containers are on the same host — Docker's internal networking handles inter-container communication natively, exactly like it does locally. Cilo just syncs your files, starts compose over SSH, and points your local DNS at the remote host.

### Hybrid — Keep What's Fast, Offload What's Heavy

You want hot reload on your frontend and API, but Elasticsearch needs more RAM than your laptop has:

```bash
# nginx and api run locally, heavy services run on staging
cilo run opencode agent-1 "refactor search" \
  --remote-services elasticsearch,postgres \
  --remote-host staging

# From your code's perspective, nothing changed:
# elasticsearch.agent-1.test still resolves
# postgres.agent-1.test still resolves
# nginx and api run locally with hot reload
```

This is the harder case — it requires network bridging between local and remote containers. But it's also where the real value is for day-to-day development.

## What This Solves

Today, running containers on a remote host requires VPNs, port forwarding, manual DNS, and config changes per environment. Cilo makes the container's location transparent — your code, your agents, and your tooling don't know or care whether a service is local or remote. The DNS name resolves either way.

---

## Scope & Non-Goals

Cilo owns **container placement and DNS resolution** — where containers run and how they're discovered. It does not own your editor, your IDE, or your full development environment.

| Cilo Does | Cilo Does Not |
|-----------|---------------|
| Place containers on local or remote hosts | Provide remote IDE or editor access |
| Make DNS resolution transparent across hosts | Manage persistent remote workspaces beyond containers |
| Sync project files to remote hosts for container builds | Provide continuous bidirectional file sync |
| Generate ingress config for remote routing | Replace Coder, Gitpod, or Codespaces |

The boundary is intentional. Tools like Coder solve "where does development happen" at an organizational level. Cilo solves "where do my containers run" at a project level. These are complementary, not competing.

---

## Architecture

### The Hybrid Model

The core idea extends Cilo's existing shared services architecture. Instead of sharing a container with another local environment, you're sharing with (or placing on) a remote host. The compose override logic is nearly identical — skip the service locally, point DNS elsewhere.

```
Local Machine                       Remote Host (staging)
┌────────────────────┐             ┌────────────────────┐
│ nginx   (isolated) │             │ elasticsearch      │
│ api     (isolated) │             │ postgres           │
│ worker  (isolated) │             │                    │
│                    │             │ cilo-ingress:      │
│ dnsmasq:           │             │   routes traffic   │
│  nginx.a1.test     │             │   to containers    │
│   → 10.224.1.2     │             │                    │
│  es.a1.test        │             │                    │
│   → 100.64.0.5  ───┼──Tailscale─┼─▶ 100.64.0.5      │
│  pg.a1.test        │             │                    │
│   → 100.64.0.5  ───┼─────"──────┼─▶ 100.64.0.5      │
└────────────────────┘             └────────────────────┘
```

**From application code:** `elasticsearch.agent-1.test` resolves the same way whether the container is local or remote. The DNS abstraction Cilo already provides is what makes this possible.

### Transport Layer

Cilo does not implement its own mesh network. It uses whatever connectivity already exists between your machine and the remote host:

| Transport | Setup Required | Best For |
|-----------|---------------|----------|
| SSH | None (already have keys) | Getting started, small teams |
| Tailscale | Install Tailscale on both machines | Teams, persistent staging hosts |
| WireGuard | Configure tunnel | Infrastructure-managed environments |

Cilo detects available transport and uses the best option. SSH is always the fallback.

---

## Implementation Phases

Each phase is independently useful. You can stop at any phase and have a working feature.

### Phase 1: Full Remote Environments via SSH

**Goal:** Run entire environments on a remote machine. This is the full-remote mode — the simplest and most immediately useful remote capability.

**Why this first:** Full-remote doesn't require any network bridging between local and remote containers. All containers are on the same remote host, so Docker's internal networking handles inter-container communication natively. Cilo only needs to sync files, run compose over SSH, and point local DNS at the remote host.

**Use cases:**
- Your laptop can't handle 10 parallel agent environments — offload to a server
- You need GPU access for ML workloads
- CI/CD on remote runners
- Your team shares a beefy staging machine

```bash
# Register a remote host (one-time)
cilo remote add staging --host staging.example.com --ssh-key ~/.ssh/id_rsa

# Run an entire environment remotely
cilo run opencode agent-1 "do the thing" --remote staging

# Run 10 agents on the remote host — your laptop barely notices
for i in $(seq 1 10); do
  cilo run opencode "agent-$i" "task $i" --remote staging &
done
wait
```

**What happens:**
1. `rsync` syncs workspace to `staging:~/.cilo/envs/myapp/agent-1/`
2. `ssh staging "cd ~/.cilo/envs/... && docker compose up -d"` starts containers
3. Container IPs are retrieved via `ssh staging "docker inspect ..."`

**Implementation:**

```go
// cilo/pkg/remote/provider.go

type SSHProvider struct {
    Host   string
    User   string
    SSHKey string
}

func (p *SSHProvider) SyncWorkspace(localPath, remotePath string) error
func (p *SSHProvider) ComposeUp(remotePath string, overrides []string) error
func (p *SSHProvider) ComposeDown(remotePath string) error
func (p *SSHProvider) GetContainerIP(containerName string) (string, error)
func (p *SSHProvider) Exec(command string) (string, error)
```

**DNS at this phase:** SSH port forwarding as a functional bridge. Cilo opens tunnels for each remote service and points local DNS at `127.0.0.1` with forwarded ports.

```bash
# Cilo opens in background:
ssh -L 9200:10.224.1.5:9200 staging.example.com
# elasticsearch.agent-1.test → 127.0.0.1 (port 9200 forwarded)
```

This reintroduces port mapping, which is what Cilo exists to avoid — but it works immediately with zero infrastructure. It validates the workflow before investing in better transport.

**Limitations:** Port forwarding means you're back to managing ports for remote services. Fine for 1-3 services, clunky for more. Phase 2 removes this limitation.

---

### Phase 2: DNS Bridging via Mesh IP

**Goal:** Remove port forwarding by routing DNS directly to the remote host's mesh IP.

**Prerequisite:** Tailscale, WireGuard, or any VPN that gives both machines routable IPs to each other.

```bash
# Cilo detects Tailscale is available
cilo remote add staging \
  --host staging.example.com \
  --mesh-ip 100.64.0.5        # Tailscale IP
```

Now instead of SSH tunnels, Cilo's local dnsmasq resolves remote services to the mesh IP:

```
elasticsearch.agent-1.test → 100.64.0.5
```

The remote host needs something listening that routes traffic to the right container. This is Phase 3's ingress daemon, but for now a simple nginx/Caddy config works:

```nginx
# Generated by Cilo on remote host
server {
    listen 9200;
    server_name elasticsearch.agent-1.test;
    location / {
        proxy_pass http://10.224.1.5:9200;  # Container IP
    }
}
```

**Implementation:**

```go
// cilo/pkg/remote/mesh.go

type MeshTransport struct {
    MeshIP   string   // e.g., 100.64.0.5
    Provider string   // "tailscale", "wireguard", "manual"
}

func DetectMeshTransport(host string) (*MeshTransport, error)
func (t *MeshTransport) IsReachable() bool
```

**DNS changes:**

```go
// cilo/pkg/dns/render.go
// For remote services, render mesh IP instead of local container IP

func RenderDNSEntry(service Service) DNSEntry {
    if service.Remote != nil {
        return DNSEntry{
            Hostname: service.DNSName,        // elasticsearch.agent-1.test
            IP:       service.Remote.MeshIP,  // 100.64.0.5
        }
    }
    return DNSEntry{
        Hostname: service.DNSName,
        IP:       service.LocalIP,
    }
}
```

---

### Phase 3: Remote Ingress Daemon

**Goal:** Automate traffic routing on the remote host so Cilo manages the full path.

A lightweight process on the remote host that reads Cilo's state and generates reverse proxy config:

```
Remote Host
┌─────────────────────────────────────┐
│  cilo-ingress                       │
│  ├── reads: ~/.cilo/state.json      │
│  ├── generates: nginx/caddy config  │
│  └── reloads: on state change       │
│                                     │
│  Routes:                            │
│   es.agent-1.test → 10.224.1.5     │
│   pg.agent-1.test → 10.224.1.6     │
│   es.agent-2.test → 10.224.2.5     │
└─────────────────────────────────────┘
```

**This is not a custom proxy.** Cilo generates config for Caddy or nginx and triggers a reload. These tools already handle connection pooling, health checks, TLS, and graceful reload. Cilo's job is generating the routing table, not reimplementing HTTP.

**Implementation:**

```go
// cilo/pkg/remote/ingress.go

type IngressManager struct {
    backend  string  // "caddy" or "nginx"
    configPath string
}

type IngressRoute struct {
    Hostname    string  // elasticsearch.agent-1.test
    Target      string  // 10.224.1.5:9200
    Environment string  // agent-1
    Project     string  // myapp
}

func (m *IngressManager) SyncRoutes(routes []IngressRoute) error  // Generate config + reload
func (m *IngressManager) RemoveRoutes(envName string) error       // Clean up on destroy
```

**Installation on remote host:**

```bash
# Run once on the remote host
cilo remote init

# This:
# 1. Installs Caddy/nginx if not present
# 2. Creates the ingress config directory
# 3. Sets up a systemd service for config watching
```

---

### Phase 4: Hybrid Local + Remote Environments

**Goal:** Run some services locally, some remotely, in the same environment.

This is the real payoff — the convergence of shared services and remote execution:

```bash
# nginx and api run locally (hot reload, fast iteration)
# elasticsearch and postgres run on staging (resources, real data)
cilo run opencode agent-1 "refactor search" \
  --remote-services elasticsearch,postgres \
  --remote-host staging
```

**Implementation builds on shared services architecture:**

The `--remote-services` flag works exactly like `--share`, but instead of connecting to a local shared container, Cilo:

1. Skips the service in the local compose override (same as shared)
2. Strips `depends_on` references to remote services (same as shared)
3. Syncs and starts the service on the remote host
4. Points local DNS to the remote host's mesh IP (Phase 2) or opens SSH tunnels (Phase 1 fallback)
5. Connects via network alias if the remote container needs to call back to local services (reverse direction)

```go
// cilo/pkg/compose/compose.go
// Transform already handles shared services — extend for remote

func Transform(env *Environment, opts TransformOptions) error {
    for _, service := range services {
        if contains(opts.SharedServices, service.Name) {
            continue  // Handled by share manager
        }
        if contains(opts.RemoteServices, service.Name) {
            continue  // Handled by remote manager
        }
        // ... generate local override as before
    }
}
```

**Reverse connectivity (remote → local):** If the remote Elasticsearch needs to call back to the local API (e.g., for webhooks), Cilo can open a reverse SSH tunnel. This is opt-in and documented as an advanced pattern, not a default.

---

### Phase 5: Multi-User Remote Hosts (If Needed)

**Goal:** Support teams sharing a remote host without stepping on each other.

**Build this only when:** Multiple developers are actively colliding on the same remote host. Not before.

The existing Cilo model — per-environment subnets, per-environment DNS namespaces — already provides isolation. What's needed for multi-user is:

**Namespace separation:** Environments on a shared host are scoped by user.

```
staging:~/.cilo/envs/
├── alice/
│   └── myapp/agent-1/    # Alice's environment
└── bob/
    └── myapp/agent-2/    # Bob's environment
```

**Resource awareness:** Cilo reports available resources on the remote host before creating environments.

```bash
cilo remote status staging
# Host: staging.example.com
# CPU: 12/16 cores in use
# RAM: 48/64 GB in use  
# Environments: 8 active (alice: 3, bob: 5)
# Subnets available: 248/256
```

**What this explicitly does not include:**
- Authentication layer or API keys for the Cilo daemon
- Quota enforcement or resource limits beyond reporting
- Admin interfaces or dashboards

These are features of a hosted platform, not a developer tool. If teams need auth and quotas, they should use SSH access control (which they already have) and team communication. Cilo reports the state; humans manage the policy.

---

## File Sync Strategy

Workspace synchronization is explicit, one-directional, and non-magical.

```bash
# Sync happens at specific moments:
cilo up agent-1 --remote staging       # Syncs once before starting
cilo sync agent-1                      # Manual re-sync
cilo run opencode agent-1 "task"       # Syncs once before running
```

**Why not continuous sync:**
- Partial syncs during file writes cause container build failures
- Large directories (`node_modules`, `.git`) make syncs slow and unpredictable
- Bidirectional sync (if the remote modifies files too) is a notorious source of conflicts

**Sync implementation:**

```bash
# One-directional: local → remote
rsync -avz --delete \
  --exclude node_modules \
  --exclude .git \
  --exclude .cilo \
  ~/.cilo/envs/myapp/agent-1/ \
  staging:~/.cilo/envs/myapp/agent-1/
```

If developers want continuous sync, they can use purpose-built tools (Mutagen, Unison) alongside Cilo. Cilo doesn't compete with file sync tools — it uses them.

---

## DNS Resolution: The Full Picture

The DNS layer is what makes all of this transparent. Here's how resolution works across all modes:

```
Application code: fetch("http://elasticsearch.agent-1.test:9200")
                                    │
                                    ▼
                        ┌─────────────────────┐
                        │  Local dnsmasq      │
                        │  (127.0.0.1:5354)   │
                        └──────────┬──────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
               Local Only    Shared Local    Remote Service
                    │              │              │
            ┌───────▼───────┐     │      ┌───────▼───────┐
            │ 10.224.1.2    │     │      │ 100.64.0.5    │
            │ (container IP │     │      │ (mesh IP of   │
            │  on local     │     │      │  remote host) │
            │  subnet)      │     │      │       │       │
            └───────────────┘     │      │  ┌────▼────┐  │
                                  │      │  │ ingress │  │
                           ┌──────▼──────┐  │ routes  │  │
                           │ Shared      │  │ to      │  │
                           │ container   │  │ container│  │
                           │ (local,     │  └─────────┘  │
                           │  multi-net) │               │
                           └─────────────┘               │
                                                         │
                                              ┌──────────▼──────────┐
                                              │ 10.224.50.5         │
                                              │ (container on       │
                                              │  remote host)       │
                                              └─────────────────────┘
```

**The key invariant:** Application code always uses `<service>.<env>.test`. The resolution path changes based on where the container runs, but the hostname never does.

---

## Configuration

### Remote Host Registration

```bash
# Add a remote host
cilo remote add staging --host staging.example.com --ssh-key ~/.ssh/id_rsa

# With explicit mesh IP (if auto-detection fails)
cilo remote add staging --host staging.example.com --mesh-ip 100.64.0.5

# List registered hosts
cilo remote list

# Test connectivity
cilo remote ping staging
```

**Stored in:** `~/.cilo/config.json`

```json
{
  "remotes": {
    "staging": {
      "host": "staging.example.com",
      "user": "deploy",
      "ssh_key": "~/.ssh/id_rsa",
      "mesh_ip": "100.64.0.5",
      "transport": "tailscale"
    }
  }
}
```

### Per-Project Defaults

Via compose labels (consistent with shared services):

```yaml
services:
  elasticsearch:
    image: elasticsearch:8.12
    labels:
      cilo.share: "true"
      cilo.remote: "staging"     # Default to remote host "staging"
  
  nginx:
    image: nginx:alpine
    # No labels = local and isolated by default
```

### CLI Override Hierarchy

```bash
# Labels define team defaults
# CLI flags override per-run

cilo run opencode agent-1 "task" \
  --remote staging                          # All services on staging
  --remote-services elasticsearch,postgres  # Only these on staging
  --local-services nginx,api               # Force these local (override labels)
```

---

## Failure Modes & Recovery

| Failure | Behavior | Recovery |
|---------|----------|----------|
| SSH connection drops | Local services keep running, remote services unreachable | `cilo sync agent-1` to re-establish |
| Remote host reboots | Containers stop, DNS entries stale | `cilo up agent-1 --remote staging` restarts |
| Mesh network down | Same as SSH drop — remote services unreachable | Falls back to SSH tunnels if configured |
| Sync conflict | Not possible — sync is one-directional | N/A |
| Remote disk full | `docker compose up` fails on remote | `cilo remote status staging` shows disk usage |

**Doctor integration:**

```bash
cilo doctor
# ...existing checks...
# 🔎 Checking remote connections...
#   staging: reachable (Tailscale, 2ms)
#   staging: 3 remote environments active
#   staging: elasticsearch.agent-1.test → 100.64.0.5 ✓
#   staging: postgres.agent-1.test → 100.64.0.5 ✓
```

---

## Relationship to Shared Services

Remote environments build directly on the shared services feature:

| Concept | Shared Services | Remote Services |
|---------|----------------|-----------------|
| Service excluded from local compose | ✓ | ✓ |
| `depends_on` stripped for excluded services | ✓ | ✓ |
| DNS resolves transparently | ✓ | ✓ |
| Network alias for inter-container DNS | ✓ (local `docker network connect`) | Via ingress/proxy |
| Reference counting for lifecycle | ✓ | Per-host tracking |
| `--share` / `--remote-services` flag | ✓ | ✓ |
| `--isolate` / `--local-services` override | ✓ | ✓ |

The compose override logic (`Transform()`) treats shared and remote services identically — both are skipped in the local override. The difference is only in where the container runs and how DNS resolves to it.

---

## What's Explicitly Deferred

These are ideas that surfaced during design but are not part of this roadmap:

- **Subnet routing over mesh** — Advertising `10.224.x.x` subnets via Tailscale/WireGuard for direct container IP access. The ingress proxy approach is simpler and sufficient. Revisit only if proxy latency becomes a measurable problem.
- **Bidirectional file sync** — Continuous or two-way sync between local and remote workspaces. Use Mutagen or Unison if needed. Cilo syncs once per operation.
- **Remote-only mode without local Cilo** — Running Cilo entirely on a remote host with no local component. This is Coder's territory, not Cilo's.
- **GUI or dashboard for remote environments** — Use `cilo remote status` and `cilo list`. A dashboard is a separate product.
- **Container migration** — Moving a running container from local to remote or vice versa. Destroy and recreate is fast enough.