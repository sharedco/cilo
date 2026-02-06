# Cilo Examples

Real-world patterns for running multiple AI agents in isolated environments.

## The Core Problem

You have **one Docker Compose project** but want to run **multiple agents** simultaneously:

```bash
# This fails—agents step on each other
cd ~/projects/myapp && opencode "fix bug A" &
cd ~/projects/myapp && opencode "fix bug B" &
# Same database, same ports, chaos.

# This also fails—same compose file, different folders
git worktree add ../agent-2 && cd ../agent-2 && docker-compose up
# ERROR: bind: address already in use
# ERROR: network name collision
```

**Cilo solves this:** Each agent gets its own isolated copy with isolated networks.

---

## Quickstart: Run 3 Agents

```bash
# Initialize cilo (one-time)
cilo init

# Run 3 agents on the same codebase simultaneously
cilo run opencode agent-1 "implement feature A" --from ./examples/basic &
cilo run opencode agent-2 "implement feature B" --from ./examples/basic &
cilo run opencode agent-3 "write tests" --from ./examples/basic &

# Each agent gets its own:
# - Isolated workspace (~/.cilo/envs/basic/agent-N/)
# - Isolated database, cache, and API
# - Unique DNS names (nginx.agent-1.test, nginx.agent-2.test, etc.)

# Access each environment
curl http://nginx.agent-1.test
curl http://nginx.agent-2.test
curl http://nginx.agent-3.test
```

---

## Example Projects

### `examples/basic` — Hello World

Simple 3-service stack: nginx + API + redis. Perfect for testing multi-agent isolation.

**Try it:**
```bash
# Run 2 agents on the same project
cilo run opencode demo-1 "list all files" --from ./examples/basic &
cilo run opencode demo-2 "check the API" --from ./examples/basic &

# Each sees different environment variables:
#   demo-1: CILO_ENV=demo-1, CILO_BASE_URL=http://basic.demo-1.test
#   demo-2: CILO_ENV=demo-2, CILO_BASE_URL=http://basic.demo-2.test
```

### `examples/basic-2` — Cross-Project Isolation

Same topology as `basic` but different content. Shows Cilo handles multiple projects too:

```bash
# Run both projects simultaneously
cilo up project-a --from ./examples/basic
cilo up project-b --from ./examples/basic-2

curl http://nginx.project-a.test  # "Hello from Basic"
curl http://nginx.project-b.test  # "Hello from Basic-2"
```

### `examples/env-render` — Dynamic Configuration

Shows how agents get environment-specific configuration:

```bash
# Create 2 environments from the same source
cilo create dev --from ./examples/env-render
cilo create staging --from ./examples/env-render
cilo up dev
cilo up staging

# Each environment renders different values
curl http://nginx.dev.test/env.txt
# CILO_PROJECT=myapp, CILO_ENV=dev, CILO_BASE_URL=http://myapp.dev.test

curl http://nginx.staging.test/env.txt
# CILO_PROJECT=myapp, CILO_ENV=staging, CILO_BASE_URL=http://myapp.staging.test
```

**Use case:** Agents that need to know their own URL or connect to sibling services.

### `examples/ingress-hostnames` — Virtual Hosts

Demonstrates subdomain routing—useful when agents need to test multi-tenant scenarios:

```bash
cilo run opencode vhost-test "test multi-tenant routing" --from ./examples/ingress-hostnames

# Inside the agent:
# curl http://app.myapp.vhost-test.test (routes to tenant app)
# curl http://admin.myapp.vhost-test.test (routes to admin panel)
```

### `examples/custom-dns-suffix` — Custom TLD

Change from `.test` to `.localhost` or your preferred domain:

```bash
cilo setup --name local --dns-suffix .localhost --from ./examples/custom-dns-suffix
cilo up local
curl http://nginx.local.localhost  # Uses custom suffix
```

---

## Agent Workflow Patterns

### Pattern 1: Parallel Feature Development

```bash
#!/bin/bash
# Run 5 agents on different features

FEATURES=("auth" "payment" "search" "notifications" "analytics")

for feature in "${FEATURES[@]}"; do
  cilo run opencode "$feature" "implement $feature" --from ~/projects/myapp &
done

wait
echo "All features completed in isolated environments"
```

**What happens:**
- 5 isolated environments created
- Each has its own database (no data pollution)
- Each has unique DNS names
- Agents work in parallel, zero conflicts

### Pattern 2: Parallel Testing Matrix

```bash
#!/bin/bash
# Test against multiple database versions simultaneously

for db_version in postgres-14 postgres-15 postgres-16; do
  export DB_VERSION=$db_version
  cilo run npm test "test-$db_version" -- --suite=integration &
done

wait
```

**What happens:**
- 3 test runs in parallel
- Each environment uses different DB version
- No shared state between test runs

### Pattern 3: CI/CD Parallelization

```bash
# .github/workflows/test.yml

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: cilo init
      - run: |
          # Create environment for this PR
          cilo run opencode "pr-${{ github.event.number }}" \
            "run tests and post results" \
            --from .
```

**What happens:**
- Each PR gets its own isolated environment
- Multiple PRs can be tested simultaneously
- No "port already in use" errors in CI

### Pattern 4: Safe Experimentation

```bash
# Try a risky refactor without breaking your main environment
cilo run opencode experiment "try the risky refactor"

# If it breaks, just destroy it
cilo destroy experiment --force

# Your main environment is untouched
```

### Pattern 5: Context Switching

```bash
# Morning: Working on feature
cilo run opencode feature-xyz "build the dashboard"

# Afternoon: Urgent bug reported
^C  # Exit feature agent
cilo run opencode hotfix "fix the critical bug"

# Evening: Back to feature—environment still there, instantly
cilo run opencode feature-xyz "finish the dashboard"
```

---

## Environment Variables Reference

When using `cilo run`, these are automatically injected:

| Variable | Example | Description |
|----------|---------|-------------|
| `CILO_ENV` | `agent-1` | Environment name (unique per agent) |
| `CILO_PROJECT` | `myapp` | Project name (from source folder) |
| `CILO_WORKSPACE` | `/home/user/.cilo/envs/myapp/agent-1` | Isolated workspace path |
| `CILO_BASE_URL` | `http://myapp.agent-1.test` | Root URL for this environment |
| `CILO_DNS_SUFFIX` | `.test` | DNS TLD (configurable per project) |

### Service Discovery Pattern

```bash
# Services follow predictable naming:
# http://<service>.<project>.<env><dns_suffix>

API_URL="http://api.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
DB_URL="http://db.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
REDIS_URL="http://redis.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
```

### Checking Environment in Scripts

```bash
#!/bin/bash
# agent-script.sh

echo "Running in environment: $CILO_ENV"
echo "Workspace: $CILO_WORKSPACE"
echo "API endpoint: $CILO_BASE_URL"

# Make requests to sibling services
curl "${CILO_BASE_URL}/api/health"
```

---

## Troubleshooting

**Agents can't connect to services?**
```bash
# Check DNS resolution
cilo dns status

# Test a specific domain
dig @127.0.0.1 -p 5354 nginx.agent-1.test

# Auto-fix common issues
cilo doctor --fix
```

**Environment won't start?**
```bash
# Check what's happening
cilo status agent-1
cilo logs agent-1

# Restart cleanly
cilo down agent-1
cilo up agent-1
```

**Need to clean up?**
```bash
# Destroy one environment
cilo destroy agent-1 --force

# Destroy all environments
cilo destroy --all --force

# Full system reset
sudo cilo teardown
```

---

## Common Commands

```bash
# Create and run
cilo run <command> <env> [args]       # One-shot: create + start + execute
cilo create <env> --from <path>       # Create environment
cilo up <env>                         # Start environment
cilo down <env>                       # Stop environment

# Information
cilo list                             # List all environments
cilo status <env>                     # Show environment status
cilo logs <env>                       # View container logs
cilo path <env>                       # Get workspace path

# Maintenance
cilo doctor                           # Health check
cilo destroy <env> --force            # Remove environment
```
