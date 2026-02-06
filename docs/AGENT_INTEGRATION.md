# Agent Integration Guide

This guide shows how to integrate AI agents and applications with Cilo's environment discovery.

## Environment Variables Reference

Cilo automatically injects these environment variables when using `cilo run`:

| Variable | Example | Purpose |
|----------|---------|---------|
| `CILO_ENV` | `feature-auth` | Environment name |
| `CILO_PROJECT` | `myapp` | Project name |
| `CILO_WORKSPACE` | `/home/user/.cilo/envs/myapp/feature-auth` | Isolated workspace path |
| `CILO_BASE_URL` | `http://myapp.feature-auth.test` | Apex URL for ingress service |
| `CILO_DNS_SUFFIX` | `.test` | DNS TLD (configurable) |

## Service Discovery Patterns

### Pattern 1: Direct URL Construction

Build service URLs from environment variables:

```bash
# Bash
API_URL="http://api.${CILO_ENV}${CILO_DNS_SUFFIX}"
DB_URL="http://db.${CILO_ENV}${CILO_DNS_SUFFIX}"
REDIS_URL="http://redis.${CILO_ENV}${CILO_DNS_SUFFIX}"
```

```javascript
// JavaScript/Node.js
const env = process.env.CILO_ENV;
const dnsSuffix = process.env.CILO_DNS_SUFFIX;

const apiUrl = `http://api.${env}${dnsSuffix}`;
const dbUrl = `http://db.${env}${dnsSuffix}`;
const redisUrl = `http://redis.${env}${dnsSuffix}`;
```

```python
# Python
import os

env = os.environ['CILO_ENV']
dns_suffix = os.environ['CILO_DNS_SUFFIX']

api_url = f"http://api.{env}{dns_suffix}"
db_url = f"http://db.{env}{dns_suffix}"
```

```go
// Go
import (
    "fmt"
    "os"
)

env := os.Getenv("CILO_ENV")
dnsSuffix := os.Getenv("CILO_DNS_SUFFIX")

apiUrl := fmt.Sprintf("http://api.%s%s", env, dnsSuffix)
dbUrl := fmt.Sprintf("http://db.%s%s", env, dnsSuffix)
```

### Pattern 2: Using CILO_BASE_URL

For the main application entry point:

```javascript
// Express.js app
const BASE_URL = process.env.CILO_BASE_URL || 'http://localhost:3000';

app.listen(3000, () => {
  console.log(`App available at: ${BASE_URL}`);
});
```

```python
# Django settings.py
import os

BASE_URL = os.environ.get('CILO_BASE_URL', 'http://localhost:8000')
ALLOWED_HOSTS = [BASE_URL.replace('http://', '').replace('https://', '')]
```

### Pattern 3: Configuration File Template

Use environment variables in config files:

```yaml
# config.yaml
database:
  host: "db.${CILO_ENV}${CILO_DNS_SUFFIX}"
  port: 5432
  
redis:
  host: "redis.${CILO_ENV}${CILO_DNS_SUFFIX}"
  port: 6379
  
api:
  base_url: "${CILO_BASE_URL}"
```

Then load with envsubst or similar:
```bash
envsubst < config.yaml.template > config.yaml
```

## Framework-Specific Integration

### OpenCode

OpenCode works out of the box with Cilo. Just use `cilo run`:

```bash
cilo run opencode feature-xyz "implement user authentication"
```

OpenCode will run in the isolated workspace with all Cilo environment variables available.

### Custom Agents

For custom agents, wrap them with Cilo:

```bash
# Create a wrapper script
cat > run-agent.sh << 'EOF'
#!/bin/bash
ENV_NAME=$1
shift
cilo run ./my-agent $ENV_NAME "$@"
EOF

chmod +x run-agent.sh
./run-agent.sh agent-1 "task description"
```

### CI/CD Integration

```yaml
# GitHub Actions example
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Cilo
        run: |
          curl -sL https://github.com/sharedco/cilo/releases/download/v0.2.1/cilo-linux-amd64 -o cilo
          chmod +x cilo
          sudo ./cilo init
      - name: Run tests in isolated environment
        run: |
          cilo run npm test ci-${{ github.run_id }} -- --ci
```

## Best Practices

### 1. Always Check Environment Variables

```javascript
// Good: Fallback to localhost for development
const baseUrl = process.env.CILO_BASE_URL || 'http://localhost:3000';

// Bad: Will break outside Cilo
const baseUrl = 'http://localhost:3000';
```

### 2. Log the Environment

Help with debugging by logging Cilo variables:

```python
import os

if 'CILO_ENV' in os.environ:
    print(f"Running in Cilo environment: {os.environ['CILO_ENV']}")
    print(f"Workspace: {os.environ['CILO_WORKSPACE']}")
    print(f"Base URL: {os.environ['CILO_BASE_URL']}")
```

### 3. Use Service Discovery in Health Checks

```bash
# health-check.sh
#!/bin/bash
API_URL="http://api.${CILO_ENV}${CILO_DNS_SUFFIX}"

curl -f "$API_URL/health" || exit 1
```

### 4. Clean Up After Tests

```bash
# test-runner.sh
#!/bin/bash
ENV_NAME="test-$$"

cleanup() {
  cilo destroy $ENV_NAME --force 2>/dev/null || true
}
trap cleanup EXIT

cilo run npm test $ENV_NAME
```

## Troubleshooting

### "Cannot resolve *.test domains"

Check that DNS is set up:
```bash
cilo doctor --fix

# Test DNS resolution
dig @127.0.0.1 -p 5354 myapp.agent-1.test
```

### "Environment variables not set"

Make sure you're using `cilo run`, not just running directly:
```bash
# Wrong: Won't have CILO_* variables
cd ~/.cilo/envs/myapp/agent-1 && npm start

# Right: Variables injected automatically
cilo run npm start agent-1
```

### "Agent can't connect to services"

Verify the environment is running:
```bash
cilo list
cilo status agent-1
```

## Advanced: Custom DNS Suffix

If `.test` conflicts with your setup, use a custom suffix:

```bash
# cilo.yaml in project root
dns_suffix: ".local"
```

Then update agent code:
```javascript
const dnsSuffix = process.env.CILO_DNS_SUFFIX || '.test';
const apiUrl = `http://api.${env}${dnsSuffix}`;
```
