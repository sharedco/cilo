# Cilo

[![Go Report Card](https://goreportcard.com/badge/github.com/sharedco/cilo)](https://goreportcard.com/report/github.com/sharedco/cilo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/version-0.2.1-blue.svg)](https://github.com/sharedco/cilo/releases)

**Run unlimited Docker Compose environments side-by-side. No port conflicts. Access by DNS name.**

---

## The Problem You Know

```bash
# You're working on a feature...
docker-compose up -d
# Works! API at localhost:8080

# PM asks you to check something on staging...
docker-compose -f docker-compose.staging.yml up -d
# ERROR: bind: address already in use
# 
# Now you stop everything, change ports in the compose file, 
# forget which port maps to which service, repeat tomorrow.
```

## The Cilo Way

```bash
# Create isolated environments
$ cilo create feature-x --from ~/projects/myapp
$ cilo create staging --from ~/projects/myapp

# Run both simultaneously—zero conflicts
$ cilo up feature-x     # API at http://api.myapp.feature-x.test
$ cilo up staging       # API at http://api.myapp.staging.test

# Access services by name, not port numbers
$ curl http://api.myapp.feature-x.test/health
$ curl http://api.myapp.staging.test/health
```

---

## Agent-Native Workflow

**One command** to create, start, and run—perfect for AI agents and automated workflows:

```bash
# Launch an agent directly in an isolated environment
$ cilo run opencode feature-x "implement user authentication"

# The agent receives these environment variables:
#   CILO_ENV=feature-x
#   CILO_BASE_URL=http://myapp.feature-x.test
#   CILO_WORKSPACE=/home/user/.cilo/envs/myapp/feature-x
#
# Services are accessible at predictable DNS names:
#   - http://api.myapp.feature-x.test
#   - http://db.myapp.feature-x.test
#   - http://redis.myapp.feature-x.test
```

**Parallel testing without conflicts:**

```bash
# Run the same test suite in 3 isolated environments simultaneously
$ cilo run npm test pr-123 -- --suite=e2e &
$ cilo run npm test pr-124 -- --suite=e2e &
$ cilo run npm test pr-125 -- --suite=e2e &

# Each gets its own database, cache, and API—no shared state, no port hell
```

---

## Why Developers Switch

| Without Cilo | With Cilo |
|--------------|-----------|
| `localhost:8080`, `localhost:3000`, `localhost:5432` | `api.myapp.dev.test`, `web.myapp.dev.test`, `db.myapp.dev.test` |
| Port conflicts when running multiple environments | Each environment isolated on its own subnet |
| Manually edit docker-compose.yml to change ports | Zero file modifications—non-destructive overrides |
| Memorize port mappings | DNS auto-discovery |
| Stop environment A to start environment B | Run unlimited environments side-by-side |
| Agents need to track ports | Agents get `CILO_BASE_URL` injected |

---

## Quick Start

```bash
# 1. Install
export PATH="$PATH:/var/deployment/sharedco/cilo/bin"
(cd /var/deployment/sharedco/cilo/cilo && go build -o cilo main.go)

# 2. Initialize (one-time, requires sudo for DNS setup)
sudo cilo init

# 3. Create and run your first environment
cilo create demo --from ./examples/basic
cilo up demo

# 4. Access via DNS
curl http://nginx.demo.test
```

---

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│  Your Project                                                │
│  └── docker-compose.yml (untouched)                         │
└──────────────────────┬──────────────────────────────────────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
   ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
   │ feature │    │ staging │    │  demo   │  ← Isolated workspaces
   │   -x    │    │         │    │         │     (~/.cilo/envs/...)
   └────┬────┘    └────┬────┘    └────┬────┘
        │              │              │
        └──────────────┼──────────────┘
                       │
              ┌────────▼────────┐
              │  10.224.x.x/24  │  ← Each env gets unique subnet
              │   (isolated)    │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │  dnsmasq        │  ← Resolves *.test domains
              │  *.test → IP    │
              └─────────────────┘
```

**Key Principles:**

1. **DNS-First Discovery** — Services are accessed by name (`api.myapp.dev.test`) not port numbers
2. **Complete Isolation** — Each environment gets its own `/24` subnet (`10.224.x.x`)
3. **Zero File Modifications** — Cilo generates hidden `.cilo/override.yml` files; your source is never touched
4. **Atomic Operations** — State and DNS updates are atomic; safe for concurrent agent usage

---

## Documentation

- **[Core Architecture](docs/ARCHITECTURE.md)** — Subnet model, DNS discovery, state management
- **[Operations & Setup](docs/OPERATIONS.md)** — Setup, troubleshooting with `cilo doctor`, teardown
- **[Examples](examples/)** — Practical configurations and use cases
- **[Future: Remote & Mesh](docs/REMOTE_NETWORKING.md)** — Multi-host networking vision

---

## Comparison

| Feature | Docker Compose | Cilo |
|---------|----------------|------|
| **Service Access** | Port mapping (`:8080`) | DNS (`service.env.test`) |
| **Multi-Instance** | Manual port management | Automatic & transparent |
| **Isolation** | Shared ports/networks | Dedicated subnets per env |
| **File Safety** | In-place edits required | Non-destructive overrides |
| **Agent Support** | High friction (track ports) | Native (`CILO_BASE_URL`) |
| **Parallel Environments** | Not possible | Unlimited |

---

## License

MIT © 2026 Cilo Authors. See [LICENSE](LICENSE) for details.
