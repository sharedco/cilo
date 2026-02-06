# Operations & Maintenance

Complete guide for managing Cilo environments, troubleshooting, and maintenance.

---

## Initial Setup (`cilo init`)

Running `sudo cilo init` performs a one-time configuration of your machine:

### What It Does

1. **Directory Scaffolding:** Creates `~/.cilo/` for state and workspace data.
2. **DNS Daemon:** Installs and starts a local `dnsmasq` instance listening on `127.0.0.1:5354`.
3. **System Resolver:**
   - **Linux:** Adds a config to `/etc/systemd/resolved.conf.d/` to forward `.test` queries.
   - **macOS:** Creates an entry in `/etc/resolver/test`.

### Why Sudo Is Required

Sudo is needed only for system DNS configuration. Cilo modifies:
- **macOS:** `/etc/resolver/test` (resolver configuration)
- **Linux:** `/etc/systemd/resolved.conf.d/cilo.conf` (systemd-resolved config)

All other Cilo operations run as your regular user.

### Custom Configuration

```bash
# Use custom subnet
sudo cilo init --base-subnet 10.225.

# Use different DNS port
sudo cilo init --dns-port 5454
```

---

## Environment Lifecycle Management

### Creating Environments

```bash
# Create from current directory
cilo create my-env

# Create from specific source
cilo create my-env --from /path/to/project

# Create empty environment
cilo create my-env --empty

# Copy only specific files
cilo create my-env --include "*.go"
```

### Starting/Stopping

```bash
# Start an environment
cilo up my-env
cilo up my-env --build       # Rebuild images
cilo up my-env --recreate    # Force recreate containers

# Stop an environment (preserves workspace)
cilo down my-env
```

### Viewing Status

```bash
# List all environments
cilo list
cilo list --all              # All projects
cilo list --format json      # JSON output
cilo list --format quiet     # Just names

# Detailed status
cilo status my-env

# View logs
cilo logs my-env
cilo logs my-env api         # Specific service
cilo logs my-env --follow    # Follow mode

# Execute commands
cilo exec my-env api bash
cilo exec my-env db psql -U postgres
```

### Destroying Environments

```bash
# Destroy single environment (with confirmation)
cilo destroy my-env

# Force destroy (no confirmation)
cilo destroy my-env --force

# Keep workspace, just remove containers
cilo destroy my-env --keep-workspace

# Destroy all environments
cilo destroy --all --force
```

**What Destroy Removes:**
- Docker containers
- Docker networks
- DNS entries
- Workspace directory (unless `--keep-workspace`)
- State tracking

---

## Automated Cleanup

### Regular Maintenance

```bash
# Weekly cleanup routine
#!/bin/bash
# cleanup-cilo.sh

# Destroy stopped environments older than 7 days
# (Note: This would require timestamp checking)

# Doctor check
cilo doctor --fix

# Prune orphaned Docker resources
docker system prune -f
```

### CI/CD Cleanup

Always clean up in CI pipelines:

```yaml
# GitHub Actions example
- name: Run tests
  run: cilo run npm test ci-$GITHUB_RUN_ID
  
- name: Cleanup
  if: always()  # Run even if tests fail
  run: cilo destroy ci-$GITHUB_RUN_ID --force
```

### Finding Orphaned Resources

```bash
# Check for resources not tracked in state
cilo doctor

# Example output:
# Found 3 orphaned resources:
#   - container: cilo-myapp-old-env-api-1
#   - network: cilo-myapp-old-env
#   - volume: cilo_myapp_old_env_db_data

# Clean them up
cilo doctor --fix
```

---

## Health & Repair (`cilo doctor`)

If DNS isn't resolving or environments feel "stuck":

### Diagnostic Mode

```bash
cilo doctor
```

Checks performed:
- Docker daemon availability
- dnsmasq process status
- State/runtime synchronization
- Orphaned Docker resources
- Network configuration

### Repair Mode

```bash
cilo doctor --fix
```

Repairs include:
- Recreate missing networks
- Restart DNS daemon
- Prune orphaned containers
- Regenerate DNS configuration
- Fix state inconsistencies

---

## Troubleshooting

### DNS Not Resolving

**Symptom:** `curl http://myapp.env.test` fails

**Diagnosis:**
```bash
# Check if dnsmasq is running
pgrep dnsmasq

# Test DNS directly
dig @127.0.0.1 -p 5354 myapp.env.test

# Check system resolver
cat /etc/resolver/test  # macOS
cat /etc/systemd/resolved.conf.d/cilo.conf  # Linux
```

**Fix:**
```bash
cilo doctor --fix

# Or manually restart
sudo killall dnsmasq
sudo cilo init
```

### Containers Won't Start

**Symptom:** `cilo up my-env` hangs or fails

**Diagnosis:**
```bash
# Check Docker
docker info

# View compose logs
cd ~/.cilo/envs/myapp/my-env
docker compose logs

# Check for port conflicts
sudo lsof -i :5354  # DNS port
```

**Fix:**
```bash
# Force recreate
cilo up my-env --recreate

# Or destroy and recreate
cilo destroy my-env --force
cilo run opencode my-env "task"
```

### Workspace Permission Issues

**Symptom:** Permission denied when accessing workspace

**Fix:**
```bash
# Fix ownership
sudo chown -R $USER:$USER ~/.cilo/envs/

# Or destroy and recreate
cilo destroy my-env --force
cilo create my-env
```

### State Corruption

**Symptom:** Environment exists in `cilo list` but doesn't work

**Fix:**
```bash
# Reconcile state with reality
cilo doctor --fix

# If still broken, manual cleanup:
# 1. Stop containers manually
docker stop $(docker ps -q --filter "name=cilo-myapp-my-env")
# 2. Remove from state
rm -rf ~/.cilo/envs/myapp/my-env
# 3. Remove state entry
# (Edit ~/.cilo/state.json carefully)
```

---

## Manual Uninstallation

If you need to remove Cilo completely:

### 1. Stop All Environments

```bash
# List all
cilo list --all

# Destroy each
for env in $(cilo list --all --format quiet); do
  cilo destroy $env --force
done

# Or destroy all at once
cilo destroy --all --force
```

### 2. Clean System DNS

**macOS:**
```bash
sudo rm /etc/resolver/test
# If using custom suffixes, remove those too
sudo rm /etc/resolver/custom
```

**Linux (systemd-resolved):**
```bash
sudo rm /etc/systemd/resolved.conf.d/cilo.conf
sudo systemctl restart systemd-resolved
```

**Linux (other resolvers):**
```bash
# Remove from /etc/resolv.conf if manually added
# Or disable dnsmasq service if system-wide
sudo systemctl stop dnsmasq
sudo systemctl disable dnsmasq
```

### 3. Remove Data

```bash
# Remove all Cilo data
rm -rf ~/.cilo

# Optional: Remove Docker networks
# (These should be gone, but just in case)
docker network prune

# Optional: Remove Docker volumes
docker volume ls | grep cilo-
```

### 4. Remove Binary

```bash
# Find and remove binary
rm $(which cilo)

# Or if installed to specific location
rm /usr/local/bin/cilo
rm ~/bin/cilo
```

### 5. Verify Removal

```bash
# Should return nothing
which cilo
ls ~/.cilo 2>/dev/null

# DNS should not resolve .test domains
dig myapp.test  # Should NOT return 127.0.0.1
```

---

## Port Conflicts

### DNS Port (5354)

By default, Cilo's DNS daemon uses port `5354`. If this port is taken:

```bash
# Check what's using it
sudo lsof -i :5354

# Use different port
sudo cilo init --dns-port 5454
```

### Common Conflicts

| Port | Service | Resolution |
|------|---------|------------|
| 5354 | Cilo DNS | Change with `--dns-port` |
| 53 | System DNS | Cilo doesn't use this |
| 80/443 | Web servers | Cilo uses DNS, not ports |

---

## Advanced Operations

### Network Management

```bash
# Recreate all networks (destructive!)
cilo network recreate

# View network configuration
cilo network info
```

### Environment Syncing

```bash
# Sync workspace with source
cilo sync my-env

# Sync specific files
cilo sync my-env --include "*.go"
```

### Backup and Restore

```bash
# Backup environment workspace
tar -czf my-env-backup.tar.gz ~/.cilo/envs/myapp/my-env

# Restore
mkdir -p ~/.cilo/envs/myapp/my-env
tar -xzf my-env-backup.tar.gz -C ~/.cilo/envs/myapp/my-env
```

---

## Performance Optimization

### Disk Usage

```bash
# Check actual vs apparent size
du -sh ~/.cilo/envs/myapp/*
du -sh --apparent-size ~/.cilo/envs/myapp/*

# Large environments
du -sh ~/.cilo/envs/*/* | sort -hr | head -10
```

### Subnet Management

Cilo automatically manages subnets in the `10.224.0.0/16` range:
- Each environment gets a `/24` subnet
- Maximum 256 environments per host
- Subnets are released when environments are destroyed

### State File Location

```
~/.cilo/
├── state.json           # Environment registry
├── config.json          # Global settings
├── envs/               # Workspaces
│   └── myapp/
│       ├── env-1/      # Environment workspace
│       └── env-2/
└── dns/                # DNS configurations
    └── dnsmasq.conf
```

---

## Getting Help

### Diagnostic Information

When reporting issues, include:

```bash
# Cilo version
cilo --version

# Environment status
cilo list --all
cilo doctor

# System info
uname -a
docker version
go version

# Logs (if relevant)
cilo logs my-env 2>&1 | head -100
```

### Community Resources

- [GitHub Issues](https://github.com/sharedco/cilo/issues)
- [Examples](../examples/)
- [Architecture Documentation](./ARCHITECTURE.md)
- [Resource Scaling Guide](./RESOURCE_SCALING.md)
