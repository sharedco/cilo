# Cilo Examples

Real-world patterns for using Cilo in development and CI/CD workflows.

## Quickstart: First Time User

```bash
# 1. Initialize cilo (one-time setup)
cilo init

# 2. Create an environment from any example
cilo create demo --from ./examples/basic
cilo up demo

# 3. Access services via DNS
curl http://nginx.demo.test
curl http://api.demo.test:8080

# 4. Clean up
cilo destroy demo --force
```

---

## Example Projects

### `examples/basic` — Hello World

A simple 3-service stack: nginx + API + redis.

**Try the agent workflow:**
```bash
# One command: create, start, and run
cilo run curl demo http://nginx.demo.test

# Launch an interactive session
cilo run bash demo
# Inside: curl http://api.demo.test:8080/health
```

### `examples/basic-2` — Multi-Project Demo

Same topology as `basic` but different content. Use this to demonstrate running two unrelated projects simultaneously:

```bash
# Run both at the same time—zero conflicts
cilo up demo1 --from ./examples/basic
cilo up demo2 --from ./examples/basic-2

curl http://nginx.demo1.test  # "Hello from Basic"
curl http://nginx.demo2.test  # "Hello from Basic-2"
```

### `examples/ingress-hostnames` — Virtual Hosts

Shows the "multiple hostnames behind one nginx" pattern common in production:

```bash
cilo create vhost --from ./examples/ingress-hostnames
cilo up vhost

# Both hostnames resolve to the same nginx
curl http://app.myapp.vhost.test
curl http://admin.myapp.vhost.test
```

**Use case:** Testing subdomain routing, multi-tenant apps, or API versioning.

### `examples/env-render` — Dynamic Configuration

Demonstrates config-driven environment variable rendering using `.cilo/config.yml`:

```bash
cilo create dev --from ./examples/env-render
cilo up dev

# See dynamically rendered values
curl http://nginx.dev.test/env.txt
# Shows: CILO_PROJECT=myapp, CILO_ENV=dev, CILO_BASE_URL=http://myapp.dev.test
```

**Use case:** Apps that need to know their own URL, database connection strings with env-specific hosts, or injecting build metadata.

### `examples/custom-dns-suffix` — Custom TLD

Change the DNS suffix from `.test` to anything you want (`.localhost`, `.dev`, etc.):

```bash
cilo setup --name local --dns-suffix .localhost --from ./examples/custom-dns-suffix
sudo cilo dns setup --dns-suffix .localhost

cilo up local
curl http://nginx.local.localhost  # Uses .localhost instead of .test
```

**Use case:** Corporate environments where `.test` is blocked, or preference for `.localhost` semantics.

---

## Workflow Patterns

### Pattern 1: The Agent Session

```bash
# Create a dedicated environment for an agent task
cilo run opencode fix-login "fix the authentication bug in the login flow"

# What happens:
# 1. Creates 'fix-login' environment if it doesn't exist
# 2. Starts the environment (docker-compose up)
# 3. Launches 'opencode' with CILO_* environment variables
#
# The agent can immediately access:
#   - http://api.myapp.fix-login.test
#   - http://db.myapp.fix-login.test
#   - $CILO_BASE_URL for API calls
```

### Pattern 2: Parallel Testing

```bash
#!/bin/bash
# ci-test.sh - Run tests against multiple feature branches

for pr in 123 124 125; do
  cilo run npm test "pr-$pr" -- --suite=e2e &
done
wait

# Each PR gets its own isolated database and API
# No port conflicts, no shared state contamination
```

### Pattern 3: Context Switching

```bash
# Hotfix comes in while you're working on a feature
cilo run opencode feature-xyz "implement new dashboard"
# ^C to exit

# Switch to hotfix instantly—feature env keeps running
cilo run opencode hotfix-urgent "fix the payment bug"

# Back to feature—already running, instant context switch
cilo run opencode feature-xyz "finish the dashboard"
```

### Pattern 4: Staging Comparison

```bash
# Run your feature branch alongside staging
cilo up staging
cilo up my-feature

# Compare responses
diff <(curl -s http://api.myapp.staging.test/data) \
     <(curl -s http://api.myapp.my-feature.test/data)
```

---

## Environment Variables Available

When using `cilo run`, these are automatically injected:

| Variable | Example Value | Description |
|----------|---------------|-------------|
| `CILO_ENV` | `demo` | Environment name |
| `CILO_PROJECT` | `myapp` | Project name |
| `CILO_WORKSPACE` | `/home/user/.cilo/envs/myapp/demo` | Full path to workspace |
| `CILO_BASE_URL` | `http://myapp.demo.test` | Root URL for the environment |
| `CILO_DNS_SUFFIX` | `.test` | DNS TLD (configurable) |

**Service discovery pattern:**
```bash
# Services are always at:
# http://<service>.<project>.<env><dns_suffix>
API_URL="http://api.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
DB_URL="http://db.${CILO_PROJECT}.${CILO_ENV}${CILO_DNS_SUFFIX}"
```

---

## Troubleshooting

**DNS not resolving?**
```bash
cilo doctor              # Check system health
cilo doctor --fix        # Attempt auto-repair
```

**Environment won't start?**
```bash
cilo status <env>        # Check what's running
cilo logs <env>          # View container logs
cilo down <env> && cilo up <env>  # Restart
```

**Need to nuke everything?**
```bash
cilo destroy <env> --force    # Remove one environment
cilo destroy --all --force    # Remove all environments
sudo cilo teardown            # Full system uninstall
```
