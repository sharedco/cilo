# Cilo Documentation

Technical documentation for Cilo's architecture, operations, and roadmap.

## Getting Started

**New to Cilo?** Start here:
- **[Main README](../README.md)** — Quick start and core concepts
- **[Examples](../examples/)** — Practical workflow patterns and use cases

## Core Documentation

### [Architecture](./ARCHITECTURE.md)

Deep dive into how Cilo works:
- **Subnet Model** — How isolation works (`10.224.x.x/24` per environment)
- **DNS Discovery** — How `dnsmasq` resolves `*.test` domains
- **Non-Destructive Overrides** — The `.cilo/override.yml` pattern
- **State Management** — Atomic operations and concurrent safety
- **Environment Variables** — Token rendering and workspace isolation

**Read this if:** You're contributing to Cilo, debugging network issues, or want to understand the internals.

### [Operations & Maintenance](./OPERATIONS.md)

Day-to-day operations and troubleshooting:
- Installation and one-time setup (`cilo init`)
- Health checks and diagnostics (`cilo doctor`)
- Common issues and solutions
- Manual uninstall procedures

**Read this if:** You're setting up Cilo for the first time, troubleshooting DNS issues, or need to tear down your installation.

### [Future: Remote & Mesh Networking](./REMOTE_NETWORKING.md)

Vision for multi-host networking:
- Remote environment synchronization
- Cross-host container communication
- Workspace state replication
- Tailscale/WG integration concepts

**Read this if:** You're interested in the roadmap, want to contribute to networking features, or need multi-host setups.

---

## Documentation by Task

| I want to... | Read this |
|--------------|-----------|
| Install Cilo for the first time | [Operations](./OPERATIONS.md) |
| Understand how DNS resolution works | [Architecture](./ARCHITECTURE.md) |
| Debug why services aren't resolving | [Operations](./OPERATIONS.md) |
| See real-world usage patterns | [Examples](../examples/) |
| Contribute to Cilo | [Architecture](./ARCHITECTURE.md) + [Contributing](../CONTRIBUTING.md) |
| Run Cilo in CI/CD | [Examples](../examples/README.md#workflow-patterns) |
| Set up custom DNS suffixes | [Examples](../examples/custom-dns-suffix/) |
| Understand the roadmap | [Remote Networking](./REMOTE_NETWORKING.md) |

---

## Quick Reference

### Common Commands

```bash
# Lifecycle
cilo create <env> --from <path>    # Create environment from project
cilo up <env>                      # Start environment
cilo down <env>                    # Stop environment
cilo destroy <env>                 # Remove environment

# Information
cilo list                          # Show all environments
cilo status <env>                  # Show environment details
cilo logs <env>                    # View container logs
cilo path <env>                    # Get workspace path

# Operations
cilo doctor                        # Health check
cilo doctor --fix                  # Auto-repair issues
cilo dns status                    # Check DNS resolution

# Agent Workflow
cilo run <command> <env> [args]    # Run command in environment
# Example: cilo run opencode feature-x "fix the bug"
```

### File Locations

```
~/.cilo/
├── state.json              # Global state (subnets, DNS config)
├── dns/
│   └── dnsmasq.conf        # DNS daemon configuration
└── envs/
    └── <project>/
        └── <env>/
            ├── docker-compose.yml      # Copied from source
            ├── .env                    # Environment variables
            └── .cilo/
                ├── override.yml        # Generated compose override
                └── meta.json           # Environment metadata
```

---

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for:
- Development setup
- Testing guidelines
- Code review process
- Release procedures
