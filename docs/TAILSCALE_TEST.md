# Cilo + Tailscale Dual-Machine Test

Test Cilo's distributed cloud architecture using Tailscale for connectivity.

## Why Tailscale?

- ✓ **Cross-network** - Linux and Mac can be anywhere (home, office, cafe)
- ✓ **No SSH keys** - Tailscale handles authentication automatically
- ✓ **NAT traversal** - Works through firewalls/routers without config
- ✓ **Encryption** - WireGuard-based, zero-config
- ✓ **MagicDNS** - Use hostnames instead of IPs

## Prerequisites

- [Tailscale account](https://login.tailscale.com/start) (free tier works)
- Linux machine with Docker
- Mac with Docker Desktop
- Both machines on same Tailscale network

## Architecture with Tailscale

```
┌─────────────────────────────────────────────────────────────┐
│                    Tailscale Network                         │
│                      (100.x.x.x/32)                         │
│                                                             │
│  ┌─────────────────────┐      ┌─────────────────────┐      │
│  │   Linux (cilo-server) │      │   Mac (cilo-agent)  │      │
│  │   100.x.x.1          │◀────▶│   100.x.x.2        │      │
│  │                      │ SSH  │                    │      │
│  │ ┌─────────────────┐  │      │  ┌─────────────────┐│      │
│  │ │ cilo-server     │  │      │  │ cilo-agent      ││      │
│  │ │ PostgreSQL      │  │      │  │ Docker Desktop││      │
│  │ │ dnsmasq         │  │      │  │ WireGuard     ││      │
│  │ └─────────────────┘  │      │  └─────────────────┘│      │
│  └─────────────────────┘      └─────────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Step-by-Step Setup

### Step 1: Install Tailscale on Both Machines

**On Linux:**
```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
# Follow the auth link in browser
```

**On Mac:**
```bash
brew install tailscale
sudo tailscale up
# Or download from https://tailscale.com/download
```

Verify both are connected:
```bash
tailscale status
# Should show both machines with 100.x.x.x IPs
```

### Step 2: Enable Tailscale SSH (Optional but Recommended)

**On Mac** (allows SSH without managing keys):
```bash
# Enable Tailscale SSH on Mac
sudo tailscale up --ssh
```

This lets the Linux server SSH to the Mac using Tailscale auth.

### Step 3: Setup Server on Linux

```bash
cd /var/deployment/sharedco/cilo
./scripts/setup-server.sh
# Save the API key!
```

### Step 4: Add Mac to Server Pool via Tailscale

Get the Mac's Tailscale IP:
```bash
# On Mac
tailscale ip -4
# Returns: 100.x.x.x
```

Add to server pool:
```bash
# On Linux
./scripts/add-machine-tailscale.sh mac-agent <tailscale-ip> <mac-username>

# Example:
./scripts/add-machine-tailscale.sh mac-agent 100.64.32.1 isaiahdahl
```

### Step 5: Verify Machine Registration

```bash
cd deploy/self-host
docker compose exec server cilo-server machines list

# Expected output:
NAME        STATUS    HOST           ASSIGNED_TO
mac-agent   ready     100.64.32.1    -
```

### Step 6: Run the Tailscale Test

```bash
export CILO_API_KEY=<your-api-key>
./scripts/test-cloud-tailscale.sh
```

## The Magic of Tailscale

### No Manual SSH Keys
Traditional SSH requires:
```bash
# OLD WAY - Painful
ssh-keygen -t ed25519 -f ~/.ssh/cilo_agent
ssh-copy-id user@192.168.x.x  # May fail if NAT/firewall
# Plus port forwarding, static IPs, etc.
```

With Tailscale SSH:
```bash
# NEW WAY - Simple
ssh user@mac-agent  # Works via Tailscale magic
# Auth handled by Tailscale, encrypted by WireGuard
```

### Cross-Network Testing
Your machines can be:
- Linux at office
- Mac at home
- Both behind different NATs
- Still connect seamlessly

### Automatic Reconnection
If a machine moves (laptop to cafe, home, office), Tailscale reconnects automatically. No IP changes to track.

## Comparison: Direct vs Tailscale

| Aspect | Direct Connection | Tailscale |
|--------|------------------|-----------|
| Same network required | ✅ Yes | ❌ No |
| SSH key management | 😰 Manual | ✅ Automatic |
| NAT/firewall config | 😰 Manual port forwarding | ✅ Automatic |
| IP addresses | 😰 Dynamic (192.168.x.x) | ✅ Stable (100.x.x.x) |
| Encryption | ✅ SSH | ✅ WireGuard |
| Setup complexity | 😰 Medium | ✅ Low |
| Works from anywhere | ❌ No | ✅ Yes |

## Testing Across Networks

Want to test from different locations?

**Scenario 1: Office + Home**
```
Linux server at office → Tailscale → Mac at home
```

**Scenario 2: Both on WiFi (different networks)**
```
Linux at Starbucks → Tailscale → Mac at library
```

**Scenario 3: Headless Linux in cloud**
```
Linux VPS (DigitalOcean, AWS, etc.) → Tailscale → Your Mac
```

## Troubleshooting Tailscale

### Can't see machines in `tailscale status`
```bash
# Check if tailscale is running
sudo tailscale up

# Check if you're logged in
tailscale status
# If not logged in, run the auth URL shown
```

### SSH connection fails
```bash
# Test connectivity
ping 100.x.x.x  # Mac's Tailscale IP

# Test SSH with verbose
ssh -v user@100.x.x.x

# Check if Tailscale SSH is enabled on Mac
tailscale status  # Should show "SSH server" enabled
```

### Agent installation hangs
```bash
# On Mac, check if SSH is listening on Tailscale interface only
sudo lsof -i :22 | grep LISTEN

# Should show listening on 100.x.x.x:22 (Tailscale IP)
# Not on 0.0.0.0:22 or 192.168.x.x:22
```

### DNS resolution issues
```bash
# Check if MagicDNS is enabled in Tailscale console
# https://login.tailscale.com/admin/dns

# Test DNS
dig mac-agent.your-tailnet.ts.net +short
# Should return 100.x.x.x
```

## Advanced: Headscale (Self-Hosted Tailscale)

If you want to self-host the coordination server:

```bash
# Run headscale (open-source Tailscale control server)
docker run -d \
  --name headscale \
  -p 8080:8080 \
  -v headscale_data:/etc/headscale \
  headscale/headscale:latest

# Machines join your private network instead of Tailscale's
```

This is useful for:
- Complete self-hosting
- Corporate networks with compliance requirements
- Air-gapped environments

## Success Criteria

Same as regular dual-machine test, but now:

1. ✅ Machines can be on different networks
2. ✅ No manual SSH key management
3. ✅ Automatic NAT traversal
4. ✅ Secure by default (WireGuard encryption)
5. ✅ Stable IPs (100.x.x.x doesn't change)

## Next Level: Multiple Agents

With Tailscale, adding more agents is trivial:

```bash
# Add a second Mac
./scripts/add-machine-tailscale.sh mac-2 100.64.32.2 user2

# Add a Linux workstation
./scripts/add-machine-tailscale.sh linux-workstation 100.64.32.3 user3

# List all machines
docker compose exec server cilo-server machines list

# Distribute environments across the pool
cilo cloud up env-1 --from .  # Goes to mac-agent
cilo cloud up env-2 --from .  # Goes to mac-2
cilo cloud up env-3 --from .  # Goes to linux-workstation
```

## Why This Matters for Production

The Tailscale architecture is actually closer to how you'd deploy in production:

| Environment | Setup |
|-------------|-------|
| **Development** | Both machines on same network (what you have now) |
| **Production** | Server in cloud, agents distributed (Tailscale model) |

Tailscale testing validates:
- ✅ Cross-network connectivity (real-world scenario)
- ✅ Automatic authentication (no manual key rotation)
- ✅ Encryption without configuration
- ✅ Scalability (add machines without network reconfiguration)

**Tailscale is the better test because it matches how real distributed teams work.**
