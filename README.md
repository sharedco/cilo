# Cilo

[![Go Report Card](https://goreportcard.com/badge/github.com/sharedco/cilo)](https://goreportcard.com/report/github.com/sharedco/cilo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/version-0.2.1-blue.svg)](https://github.com/sharedco/cilo/releases)

**Cilo creates isolated, high-fidelity environments from Docker Compose projects.**

By treating networking as a first-class citizen, Cilo eliminates port collisions and provides production-like DNS discovery for local development.

---

## Why Cilo?

Most local development tools rely on port mapping (`localhost:8080`), leading to "port hell" when running multiple projects. Cilo takes a different approach:

- **DNS-First Discovery:** Access services via `http://nginx.dev.test` instead of `localhost:8080`.
- **Complete Isolation:** Each environment gets its own dedicated subnet (`10.224.x.x`) and workspace.
- **Agent-Ready:** Designed for AI agents that need stable, predictable, and isolated environments to execute tasks.
- **Zero-Conflict Overrides:** Uses native Docker Compose overrides. Your source files are never modified.
- **Reliable Foundations:** Built with concurrent-safe state management and atomic DNS updates.

## Architecture at a Glance

```text
       User / AI Agent
              │
      ┌───────▼───────┐
      │   Cilo CLI    │◀─── Manage Subnets & DNS
      └───────┬───────┘
              │
    ┌─────────┼─────────┐
    │         │         │
┌───▼───┐ ┌───▼───┐ ┌───▼───┐
│ Env A │ │ Env B │ │ Env C │  (Isolated Networks 10.224.x.x)
└───┬───┘ └───┬───┘ └───┬───┘
    └─────────┴─────────┘
              │
      ┌───────▼───────┐
      │ Local DNS     │ (dnsmasq)
      │ *.test        │
      └───────────────┘
```

## Install

```bash
# Add to PATH
export PATH="$PATH:/var/deployment/sharedco/cilo/bin"

# Build the binary
(cd /var/deployment/sharedco/cilo/cilo && go build -o cilo main.go)

# One-time system setup (requires sudo)
sudo cilo init
```

## Quick Start

```bash
# Create an environment named 'dev' from a project
cilo create dev --from ~/projects/myapp

# Spin it up
cilo up dev

# Access immediately via DNS
curl http://nginx.dev.test
```

## Documentation

- **[Core Architecture](docs/ARCHITECTURE.md)** - How it works today.
- **[Operations & Setup](docs/OPERATIONS.md)** - Setup, health, and teardown.
- **[Future: Remote & Mesh](docs/REMOTE_NETWORKING.md)** - Our vision for multi-host networking.
- **[Examples & Use Cases](examples/)** - Practical configurations.

## Comparison

| Feature | Docker Compose | Cilo |
|---------|----------------|------|
| **Service Access** | Port mapping (8080) | DNS (service.env.test) |
| **Isolation** | Shared ports/networks | Isolated Subnets |
| **Multi-Instance** | Manual port management | Automatic & Transparent |
| **Agent Support** | High Friction | Native / First-Class |
| **File Safety** | In-place edits | Non-destructive Overrides |

## License

MIT © 2026 Cilo Authors. See [LICENSE](LICENSE) for details.
