# Cilo

[![Go Report Card](https://goreportcard.com/badge/github.com/sharedco/cilo)](https://goreportcard.com/report/github.com/sharedco/cilo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/version-0.2.1-blue.svg)](https://github.com/sharedco/cilo/releases)

**Run unlimited isolated agents on the same Docker Compose project. No conflicts, no overlaps.**

---

## The Problem You Know

```bash
# You want to run 3 AI agents on the same codebase simultaneously...

# Agent 1: Working on feature A
cd ~/projects/myapp && opencode "implement user auth"

# Agent 2: Working on feature B  
cd ~/projects/myapp && opencode "add payment integration"
# ^ Agents step on each other! Same containers, same ports, chaos.

# OR: You try git worktrees...
git worktree add ../myapp-agent-2
cd ../myapp-agent-2 && docker-compose up
# ERROR: bind: address already in use (port 5432 in use by agent 1)
# ERROR: network name collision
# 
# Same compose file, different folder, still conflicts.
```

## The Cilo Way

```bash
# 3 agents, 1 codebase, zero conflicts

# Agent 1 gets isolated environment
cilo run opencode feature-auth "implement user authentication"

# Agent 2 gets isolated environment  
cilo run opencode feature-payment "add payment integration"

# Agent 3 gets isolated environment
cilo run opencode feature-analytics "add analytics dashboard"

# Each agent sees:
#   - http://api.feature-auth.test (Agent 1's API)
#   - http://api.feature-payment.test (Agent 2's API)  
#   - http://api.feature-analytics.test (Agent 3's API)
#
# Each gets its own isolated containers (database, redis, etc.) and network.
```

---

## What Cilo Does

**Cilo creates isolated copies of your Docker Compose project** that run side-by-side without conflicts:

- **Same source code** — All environments copy from the same project
- **Isolated workspaces** — Each environment gets its own folder (`~/.cilo/envs/myapp/<name>/`)
- **Isolated networks** — Each environment gets its own subnet (`10.224.x.x/24`)
- **DNS names** — Services accessible by name, not port numbers

---

## The Agent Workflow

### Pattern 1: Parallel Feature Development

```bash
# Run 3 agents simultaneously on the same codebase—zero overlap
cilo run opencode agent-1 "implement login" &
cilo run opencode agent-2 "fix navigation" &
cilo run opencode agent-3 "add search" &

# Each session gets:
# - Its own copy of the project
# - Its own containers (database, cache, API) with isolated state
# - Environment variables pointing to its own services
```

### Pattern 2: Parallel Testing

```bash
# Run the same test suite against multiple branches
for branch in feature-a feature-b feature-c; do
  git checkout $branch
  cilo run npm test $branch -- --suite=e2e &
done
wait

# Each branch tests in isolation—no state pollution between runs
```

### Pattern 3: CI/CD Parallelization

```bash
# In your CI pipeline—each PR gets isolated env
cilo run opencode pr-$PR_NUMBER "review and fix issues"

# 10 PRs = 10 isolated environments, all from the same docker-compose.yml
```

### Pattern 4: Context Switching

```bash
# Hotfix comes in while you're deep in feature work
$ cilo run opencode feature-xyz "build new dashboard"
^C

# Switch to hotfix instantly—feature env keeps running
$ cilo run opencode hotfix-urgent "fix payment bug"

# Back to feature—environment still there, instant resume
$ cilo run opencode feature-xyz "finish the dashboard"
```

---

## How It Works

```
Your Source Code (git repo)
└── docker-compose.yml (single source of truth)
         │
         │  cilo creates isolated copies:
         │
    ┌────┴────┬────────┬────────┐
    │         │        │        │
┌───▼───┐ ┌──▼───┐ ┌──▼───┐ ┌──▼───┐
│agent-1│ │agent-│ │agent-│ │hotfix│  ← Isolated workspaces
│       │ │  2   │ │  3   │ │      │     (~/.cilo/envs/myapp/...)
└───┬───┘ └──┬───┘ └──┬───┘ └──┬───┘
    │        │        │        │
    └────────┴───┬────┴────────┘
                 │
        ┌────────▼────────┐
        │ 10.224.0.0/16   │  ← Each env gets unique /24 subnet
        │   (isolated)    │
        └────────┬────────┘
                 │
        ┌────────▼────────┐
        │  dnsmasq        │  ← Resolves *.test domains
        │  *.test → IP    │
        └─────────────────┘
```

**Key Principles:**

1. **Isolate, Don't Modify** — Your source code stays untouched; environments are copies
2. **DNS-First** — Access services by name (`api.agent-1.test`) not ports (`localhost:8080`)
3. **Parallel by Default** — Run as many environments as you need, simultaneously
4. **Agent-Native** — One command creates, starts, and runs: `cilo run <command> <env>`

---

## Environment Variables

When using `cilo run`, these are injected automatically:

| Variable | Example | Purpose |
|----------|---------|---------|
| `CILO_ENV` | `agent-1` | Environment name |
| `CILO_PROJECT` | `myapp` | Project name |
| `CILO_WORKSPACE` | `~/.cilo/envs/myapp/agent-1` | Path to isolated workspace |
| `CILO_BASE_URL` | `http://myapp.agent-1.test` | Apex URL (project.env.test) for ingress service |
| `CILO_DNS_SUFFIX` | `.test` | DNS TLD (configurable) |

**Service discovery:**
```bash
# Services are always at predictable URLs:
# Format: http://<service>.<env><dns_suffix>
API_URL="http://api.${CILO_ENV}${CILO_DNS_SUFFIX}"      # api.agent-1.test
DB_URL="http://db.${CILO_ENV}${CILO_DNS_SUFFIX}"        # db.agent-1.test
REDIS_URL="http://redis.${CILO_ENV}${CILO_DNS_SUFFIX}"  # redis.agent-1.test

# Apex/ingress URL (from CILO_BASE_URL):
# http://<project>.<env><dns_suffix> → routes to ingress service
```

---

## Quick Start

```bash
# 1. Install (build from source for now—releases coming soon)
git clone https://github.com/sharedco/cilo.git
cd cilo/cilo && go build -o cilo main.go
export PATH="$PATH:$(pwd)"

# 2. Initialize (one-time, requires sudo for DNS setup)
sudo cilo init

# 3. Run your first isolated agent
cilo run opencode demo "fix the login bug"

# 4. Access the environment
curl http://api.demo.test  # service.env.test format
```

---

## Installation

**Current (v0.2.1): Build from source**
```bash
git clone https://github.com/sharedco/cilo.git
cd cilo/cilo
go build -o cilo main.go
mv cilo /usr/local/bin/  # or anywhere in your PATH
```

**Coming soon:**
- Homebrew: `brew install sharedco/tap/cilo`
- Releases: Pre-built binaries on GitHub Releases

---

## Managing Environments

Each `cilo run` creates an isolated environment. Here's how to manage them:

```bash
# See all your environments
cilo list                    # Current project
cilo list --all              # All projects

# Stop an environment (keeps workspace)
cilo down agent-1

# Destroy an environment (removes workspace & containers)
cilo destroy agent-1
cilo destroy agent-1 --force # Skip confirmation

# Clean up everything
cilo destroy --all --force

# Check health and find orphaned resources
cilo doctor                  # Diagnose issues
cilo doctor --fix            # Auto-repair
```

**Environment Lifecycle:**
- **Created:** `cilo run` or `cilo create` makes a workspace copy
- **Running:** Containers are active, DNS entries exist
- **Stopped:** `cilo down` stops containers but keeps workspace
- **Destroyed:** `cilo destroy` removes everything (containers, workspace, DNS)

---

## DNS Setup (What sudo Changes)

`sudo cilo init` configures local DNS resolution for `.test` domains. Here's exactly what changes:

**macOS:**
- Creates `/etc/resolver/test` → points to `127.0.0.1:5354`

**Linux (systemd-resolved):**
- Creates `/etc/systemd/resolved.conf.d/cilo.conf`
- Adds DNS stub listener on `127.0.0.1:5354`

**To remove:**
```bash
# macOS
sudo rm /etc/resolver/test

# Linux
sudo rm /etc/systemd/resolved.conf.d/cilo.conf
sudo systemctl restart systemd-resolved
```

See [Operations Guide](docs/OPERATIONS.md#manual-uninstallation) for full details.

---

## Resource Considerations

Cilo environments share your machine's resources. Plan accordingly:

**Container Count:**
```
10 environments × 5 services each = 50 containers
```

**Resource Estimates:**
| Environment Type | RAM | CPU | Disk |
|-----------------|-----|-----|------|
| Simple (1-2 containers) | 256MB | 0.25 cores | ~50MB |
| Typical (3-5 containers) | 512MB-1GB | 0.5 cores | ~100MB |
| Heavy (DB + cache + apps) | 2-4GB | 1-2 cores | ~500MB |

**Recommendations:**
- Start with 3-5 environments
- Monitor with `docker stats` and `cilo list`
- Use `cilo destroy` aggressively—environments are cheap to recreate
- Consider Docker resource limits in your compose files

See [Resource Scaling Guide](docs/RESOURCE_SCALING.md) for detailed guidance.

---

## Disk Efficiency (Copy-on-Write)

**Cilo uses copy-on-write (CoW) for efficient storage.** When creating environment copies:

- **Linux (btrfs/XFS):** Uses `FICLONE` ioctl—near-zero extra disk space
- **macOS (APFS):** Uses file cloning—near-zero extra disk space
- **Fallback:** Standard copy if CoW unavailable

**What this means:**
```bash
# Your source code: 500MB
# 10 environments with CoW: ~500MB total (not 5GB!)
# 10 environments without CoW: ~5GB

# Check actual disk usage
du -sh ~/.cilo/envs/myapp/*  # Shows apparent size
du -sh --apparent-size ~/.cilo/envs/myapp/*  # Shows logical size
```

**Requirements:**
- Modern filesystem (APFS, btrfs, XFS with reflink)
- Same filesystem as source code

---

## Agent Integration

Cilo provides environment variables for agents to discover services automatically:

```bash
# In your agent or application
export API_URL="http://api.${CILO_ENV}${CILO_DNS_SUFFIX}"
export DB_URL="http://db.${CILO_ENV}${CILO_DNS_SUFFIX}"
```

**Example: Node.js agent reading CILO_BASE_URL**
```javascript
// config.js
const baseUrl = process.env.CILO_BASE_URL || 'http://localhost:3000';
module.exports = { baseUrl };
```

**Example: Python agent discovering services**
```python
import os

env = os.environ['CILO_ENV']
dns_suffix = os.environ['CILO_DNS_SUFFIX']

api_url = f"http://api.{env}{dns_suffix}"
db_url = f"http://db.{env}{dns_suffix}"
```

See [Agent Integration Guide](docs/AGENT_INTEGRATION.md) for complete patterns.

---

## Why Teams Switch to Cilo

| Without Cilo | With Cilo |
|--------------|-----------|
| Agents step on each other's files/database | Each agent has isolated workspace |
| Can't run multiple agents on same project | Unlimited parallel environments |
| Port conflicts when using git worktrees | Each environment gets unique subnet |
| "Wait, which port is this agent using?" | Predictable DNS names per environment |
| Manual docker-compose overrides | Zero file modifications—fully automated |

---

## Documentation

- **[Examples](examples/)** — Real-world agent patterns and workflows
- **[Core Architecture](docs/ARCHITECTURE.md)** — How isolation works
- **[Operations & Setup](docs/OPERATIONS.md)** — Troubleshooting and maintenance
- **[Resource Scaling](docs/RESOURCE_SCALING.md)** — Resource planning and limits
- **[Agent Integration](docs/AGENT_INTEGRATION.md)** — Integrating agents with Cilo
- **[Concerns Assessment](docs/CONCERNS_ASSESSMENT.md)** — Objective review of project concerns
- **[Future: Remote & Mesh](docs/REMOTE_NETWORKING.md)** — Multi-host agent distribution

---

## License

MIT © 2026 Cilo Authors. See [LICENSE](LICENSE) for details.
