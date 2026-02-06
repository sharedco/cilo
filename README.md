# Cilo

**Cilo creates isolated environments from Docker Compose projects.**

An environment is a runnable copy with:
- its own workspace under `~/.cilo/envs/<env>/`
- its own Docker network (subnet `10.224.x.x`)
- optional DNS names under `*.test`

## Install

```bash
# Add to PATH
export PATH="$PATH:/var/deployment/sharedco/cilo/bin"

# Build the binary once
(cd /var/deployment/sharedco/cilo/cilo && go build -o cilo main.go)

cilo --help
cilo --version
```

## Quick Start

```bash
# One-time setup (requires sudo)
sudo cilo init

# Create from any project with docker-compose.yml
cilo create dev --from ~/projects/myapp
cilo up dev

# Access via DNS
curl http://nginx.dev.test
```

## What Does `cilo init` Do?

**`cilo init` is a ONE-TIME PER MACHINE setup** (like installing Docker).

It creates:
- `~/.cilo/` directory for all cilo data
- `~/.cilo/dns/dnsmasq.conf` for local DNS
- System DNS configuration to resolve `*.test` domains

**NOT per project.** After running init once, you can create unlimited environments from any number of projects:

```bash
# Run once on your machine
sudo cilo init

# Then create as many environments as you want
# From project 1
cilo create myapp-dev --from ~/projects/myapp
cilo create myapp-prod --from ~/projects/myapp

# From project 2
cilo create website-dev --from ~/projects/website

# From project 3
cilo create api-staging --from ~/projects/api-service
```

All environments share the same DNS infrastructure set up by init.

## How It Works

### Project vs Environment

| Term | Meaning | Example |
|------|---------|---------|
| **Project** | Directory with `docker-compose.yml` | `~/projects/myapp` |
| **Environment** | Isolated copy with its own network/DNS | `dev`, `staging`, `feature-x` |

### DNS Organization

Cilo supports two DNS models:

#### Model 1: Simple (default)
Format: `<service>.<env>.test`

```
nginx.dev.test → 10.224.1.2
api.dev.test → 10.224.1.3
```

#### Model 2: Project-based with wildcard
Format: `(<hostname>.)<project>.<env>.test`

```bash
cilo create dev --project pc --from ~/projects/pc
```

DNS generated:
```
*.pc.dev.test → 10.224.1.2 (wildcard)
pc.dev.test → 10.224.1.2 (apex)
api.pc.dev.test → 10.224.1.2
admin.pc.dev.test → 10.224.1.2
```

## Defining Hostnames

You have three options:

### Option 1: Docker Compose Labels

Add to your `docker-compose.yml`:

```yaml
services:
  nginx:
    image: nginx:alpine
    labels:
      cilo.ingress: "true"
      cilo.hostnames: "api.pc,admin.pc,cpanel.pc"
```

Auto-detection: Services named `nginx`, `web`, `app`, or `frontend` are auto-marked as ingress.

### Option 2: CLI Commands

```bash
# Add hostnames dynamically
cilo hostnames add dev api.pc
cilo hostnames add dev admin.pc,cpanel.pc

# List current hostnames
cilo hostnames list dev

# Remove hostnames
cilo hostnames remove dev old.pc
```

### Option 3: Project Flag

The `--project` flag sets the DNS namespace:

```bash
cilo create dev --project pc --from ~/projects/pc
```

## Common Commands

```bash
# Environment lifecycle
cilo create <env> --from <path> [--project <name>]
cilo up <env>
cilo down <env>
cilo destroy <env> --force

# Hostname management
cilo hostnames list <env>
cilo hostnames add <env> <hostname>...
cilo hostnames remove <env> <hostname>...
cilo hostnames set <env> --file <file>

# Health check and repair (v0.1.20+)
cilo doctor              # Check system health
cilo doctor --fix        # Fix issues automatically

# Info & debugging
cilo list
cilo status <env>
cilo dns status
```

## Cleanup

```bash
# Stop environment
cilo down dev

# Remove completely
cilo destroy dev --force

# Reset everything
rm -rf ~/.cilo
cilo init
```

## DNS Troubleshooting

```bash
# Check DNS config
cat ~/.cilo/dns/dnsmasq.conf

# Test directly against dnsmasq
dig @127.0.0.1 -p 5354 nginx.dev.test +short

# Check system resolver
resolvectl query nginx.dev.test

# Flush caches
sudo resolvectl flush-caches
```

## System Reliability (v0.1.20+)

Cilo now has a hardened foundation for concurrent use:

- **File locking:** Concurrent-safe state operations prevent corruption
- **Atomic writes:** State and DNS updates use temp-file + rename pattern
- **Collision detection:** Automatic subnet conflict detection with existing Docker networks
- **Graceful DNS reload:** SIGHUP signal reconfigures dnsmasq without interruption
- **System DNS detection:** Automatically uses your system's DNS (systemd-resolved, NetworkManager, macOS)

## Implementation Details

**Storage locations:**
- State: `~/.cilo/state.json` (with file locking)
- Workspaces: `~/.cilo/envs/<env>/`
- DNS config: `~/.cilo/dns/dnsmasq.conf` (full render from state)

**Wildcard DNS:**
When both `project` and `ingress` service are set, cilo generates:
- `/.project.env.test/` → ingress IP (wildcard for all subdomains)
- `/project.env.test/` → ingress IP (apex)
- Plus explicit hostnames from labels or CLI
