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
cd ~/projects/myapp && opencode agent "implement user auth"

# Agent 2: Working on feature B
cd ~/projects/myapp && opencode agent "add payment integration" 
# ^ Agents step on each other! Shared database, shared ports, chaos.

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
#   - http://api.myapp.feature-auth.test (Agent 1's API)
#   - http://api.myapp.feature-payment.test (Agent 2's API)  
#   - http://api.myapp.feature-analytics.test (Agent 3's API)
#
# Each gets its own database, redis, and network—completely isolated.
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

# Each agent gets:
# - Its own copy of the project
# - Its own database, cache, and API
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

# Each branch tests in isolation—no database pollution between runs
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
| `CILO_BASE_URL` | `http://myapp.agent-1.test` | Root URL for this environment |
| `CILO_DNS_SUFFIX` | `.test` | DNS TLD (configurable) |

**Service discovery:**
```bash
# Services are always at predictable URLs:
API_URL="http://api.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
DB_URL="http://db.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
```

---

## Quick Start

```bash
# 1. Install
export PATH="$PATH:/var/deployment/sharedco/cilo/bin"
(cd /var/deployment/sharedco/cilo/cilo && go build -o cilo main.go)

# 2. Initialize (one-time, requires sudo for DNS)
sudo cilo init

# 3. Run your first isolated agent
cilo run opencode demo "fix the login bug"

# 4. Access the environment
curl http://api.myapp.demo.test
```

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
- **[Future: Remote & Mesh](docs/REMOTE_NETWORKING.md)** — Multi-host agent distribution

---

## License

MIT © 2026 Cilo Authors. See [LICENSE](LICENSE) for details.
