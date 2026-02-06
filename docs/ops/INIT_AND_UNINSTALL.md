# Init and Uninstall Runbook

This document describes what `cilo init` creates and how to fully uninstall cilo.

---

## What `cilo init` Creates

### User Files (`~/.cilo/`)

| Path | Purpose |
|------|---------|
| `~/.cilo/` | Root directory for all cilo data |
| `~/.cilo/state.json` | Environment state (subnets, services, metadata) |
| `~/.cilo/envs/` | Workspace directories for each environment |
| `~/.cilo/dns/dnsmasq.conf` | Generated dnsmasq configuration |
| `~/.cilo/dns/dnsmasq.pid` | PID file for dnsmasq process |

### System Files (require sudo)

#### Linux (systemd-resolved)

| Path | Purpose |
|------|---------|
| `/etc/systemd/resolved.conf.d/cilo.conf` | Tells systemd-resolved to forward `.test` to cilo's dnsmasq |

#### macOS

| Path | Purpose |
|------|---------|
| `/etc/resolver/test` | Tells macOS resolver to forward `.test` to cilo's dnsmasq |

### Processes

| Process | Purpose |
|---------|---------|
| `dnsmasq` (port 5354) | Local DNS server for `.test` domain resolution |

---

## Uninstall

### Quick Uninstall (removes everything)

```bash
# Stop dnsmasq
pkill -f "dnsmasq.*cilo" || true

# Remove user data
rm -rf ~/.cilo

# Remove system DNS config (Linux)
sudo rm -f /etc/systemd/resolved.conf.d/cilo.conf
sudo systemctl restart systemd-resolved

# Remove system DNS config (macOS)
sudo rm -f /etc/resolver/test
```

### Partial Uninstall (keep environments, remove DNS)

```bash
# Stop dnsmasq
pkill -f "dnsmasq.*cilo" || true

# Remove DNS config only
rm -rf ~/.cilo/dns

# Remove system DNS config
sudo rm -f /etc/systemd/resolved.conf.d/cilo.conf  # Linux
sudo rm -f /etc/resolver/test                       # macOS
sudo systemctl restart systemd-resolved             # Linux only
```

### Keep Data, Reset DNS

```bash
# Stop dnsmasq and remove PID
pkill -f "dnsmasq.*cilo" || true
rm -f ~/.cilo/dns/dnsmasq.pid

# Re-run init to regenerate DNS
sudo cilo init
```

---

## Diagnostics

### Check if cilo is initialized

```bash
ls -la ~/.cilo/
cat ~/.cilo/state.json
```

### Check if dnsmasq is running

```bash
pgrep -f "dnsmasq.*cilo"
cat ~/.cilo/dns/dnsmasq.pid
```

### Check DNS resolution

```bash
# Direct query to cilo's dnsmasq
dig @127.0.0.1 -p 5354 nginx.dev.test +short

# Through system resolver
dig nginx.dev.test +short

# Linux: check systemd-resolved
resolvectl query nginx.dev.test
```

### Check system DNS configuration

```bash
# Linux
cat /etc/systemd/resolved.conf.d/cilo.conf
resolvectl status

# macOS
cat /etc/resolver/test
scutil --dns
```

---

## Conflict Resolution

### Port 5354 already in use

```bash
# Find what's using the port
sudo lsof -i :5354

# Kill it (if safe)
sudo kill <pid>

# Re-run init
sudo cilo init
```

### Another dnsmasq running

```bash
# Check for other dnsmasq instances
pgrep -a dnsmasq

# If conflicting, stop the other one or configure cilo to use a different port
```

### DNS not resolving after init

```bash
# Restart systemd-resolved (Linux)
sudo systemctl restart systemd-resolved

# Flush DNS cache (macOS)
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder

# Verify dnsmasq is running
pgrep -f "dnsmasq.*cilo" || sudo cilo init
```

---

## Recovery

### State corruption

```bash
# Back up current state
cp ~/.cilo/state.json ~/.cilo/state.json.backup

# Reset state (loses environment tracking, not workspaces)
echo '{"version":1,"subnet_counter":0,"environments":{}}' > ~/.cilo/state.json

# Re-import existing environments manually or recreate them
```

### Orphaned Docker resources

```bash
# Find cilo-managed containers
docker ps -a --filter "label=cilo.environment"

# Find cilo-managed networks  
docker network ls --filter "name=cilo_"

# Clean up manually
docker rm -f $(docker ps -aq --filter "label=cilo.environment")
docker network rm $(docker network ls -q --filter "name=cilo_")
```
