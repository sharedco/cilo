# Dual-Machine Cloud Test Guide

Test Cilo's distributed cloud architecture using two machines: a Linux server and a Mac agent.

## Architecture Overview

```
┌─────────────────┐         ┌─────────────────┐
│   Linux Server  │         │    Mac Agent    │
│                 │         │                 │
│ ┌─────────────┐ │   SSH   │ ┌─────────────┐ │
│ │ cilo-server │ │────────▶│ │ cilo-agent  │ │
│ │  (Go API)   │ │ Install │ │  (daemon)   │ │
│ └─────────────┘ │         │ └─────────────┘ │
│        │        │         │        │        │
│ ┌─────────────┐ │         │ ┌─────────────┐ │
│ │  PostgreSQL │ │         │ │   Docker    │ │
│ │   (state)   │ │         │ │ (runtime)   │ │
│ └─────────────┘ │         │ └─────────────┘ │
│        │        │         │        │        │
│ ┌─────────────┐ │         │ ┌─────────────┐ │
│ │   dnsmasq   │ │◀────────│ │ WireGuard   │ │
│ │   (DNS)     │ │  Tunnel │ │   (VPN)     │ │
│ └─────────────┘ │         │ └─────────────┘ │
└─────────────────┘         └─────────────────┘
         ▲                           ▲
         │                           │
         └────────── CLI ────────────┘
              (cilo cloud up)
```

## Prerequisites

- Linux machine with Docker and Docker Compose
- Mac machine with:
  - Docker Desktop installed
  - SSH enabled (System Preferences > Sharing > Remote Login)
  - User account with passwordless sudo OR ability to enter password
- Both machines on the same network (or Mac accessible via IP from Linux)

## Step-by-Step Setup

### Step 1: Start Server on Linux

On your Linux machine:

```bash
cd /var/deployment/sharedco/cilo
./scripts/setup-server.sh
```

This will:
- Start PostgreSQL database
- Start cilo-server API
- Create an admin API key (save this!)
- Expose server on http://localhost:8080

### Step 2: Prepare Mac for Agent Installation

On your Mac:

```bash
# Generate SSH key for server to connect
ssh-keygen -t ed25519 -f ~/.ssh/cilo_agent -N ""

# Show the public key
cat ~/.ssh/cilo_agent.pub
```

Copy this public key - you'll need it on the Linux machine.

Also get your Mac's IP address:

```bash
ipconfig getifaddr en0  # WiFi
# or
ipconfig getifaddr en1  # Ethernet
```

### Step 3: Add Mac to Server Pool

On your Linux machine:

```bash
# Copy the SSH public key to authorized_keys
echo "<paste the Mac SSH public key here>" >> ~/.ssh/authorized_keys

# Add the machine to cilo
./scripts/add-machine.sh mac-agent <mac-ip-address> <mac-username>
```

The server will:
- SSH to the Mac
- Download and install cilo-agent binary
- Start the agent daemon
- Mark machine as "ready"

Check status:

```bash
cd deploy/self-host
docker compose exec server cilo-server machines list
```

You should see:
```
NAME        STATUS    HOST            ASSIGNED_TO
mac-agent   ready     192.168.x.x     -
```

### Step 4: Run the Dual-Machine Test

On your Linux machine:

```bash
# Set environment variables
export CILO_SERVER_URL=http://localhost:8080
export CILO_API_KEY=<your-api-key-from-step-1>

# Run the test
./scripts/test-cloud-dual.sh
```

This will:
1. Login to the cloud server
2. List available machines
3. Create a remote environment on the Mac
4. Verify DNS resolution works
5. Test HTTP access through WireGuard tunnel
6. Clean up the environment

## What Gets Tested

### ✅ Server Functionality
- API authentication with keys
- Machine pool management
- Environment lifecycle (create/start/stop/destroy)
- State persistence in PostgreSQL

### ✅ Agent Functionality
- Agent installation via SSH
- Docker runtime management
- Environment execution
- Health reporting

### ✅ Networking
- WireGuard tunnel establishment
- Multi-peer key exchange
- DNS resolution (dnsmasq → remote)
- HTTP routing through tunnel

### ✅ CLI Integration
- `cilo cloud login`
- `cilo cloud up/down/destroy`
- `cilo cloud status`
- WireGuard interface management

## Troubleshooting

### Server won't start
```bash
cd deploy/self-host
docker compose logs server
```

### Machine shows as "unavailable"
```bash
# Check SSH connectivity from Linux to Mac
ssh -i ~/.ssh/cilo_agent <mac-user>@<mac-ip>

# Check agent logs on Mac
ssh <mac-user>@<mac-ip> 'tail -f /tmp/cilo-agent.log'
```

### DNS not resolving
```bash
# Check dnsmasq is running
pgrep dnsmasq

# Test direct query
dig @127.0.0.1 -p 5354 web.test-env.test

# Check system resolver
systemd-resolve --status | grep cilo
```

### WireGuard tunnel fails
```bash
# Check interface exists
sudo wg show

# Check routes
ip route | grep 10.225

# Test direct ping to agent
ping 10.225.0.100
```

## Expected Test Output

```
========================================
Cilo Cloud Dual-Machine Test
========================================
Server: http://localhost:8080

Step 1: Login to cloud...
✓ Logged in

Step 2: Check machine pool status...
{
  "machines": [
    {
      "id": "mac-agent",
      "status": "ready",
      "host": "192.168.1.100"
    }
  ]
}

Step 3: Create a remote environment...
Creating environment dual-test-1707512345 on remote machine...
✓ Environment created

Step 4: Check environment status...
Environment: dual-test-1707512345
Status: running
Machine: mac-agent (192.168.1.100)
Services:
  web: 10.224.1.3 (running)

Step 5: Verify DNS resolution...
10.224.1.3

Step 6: Test HTTP access...
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...

Step 7: Cleanup...
✓ Environment destroyed

========================================
Test Complete!
========================================
```

## Success Criteria

All these must work for the test to pass:

1. ✅ Server starts and accepts API calls
2. ✅ Agent installs successfully on Mac
3. ✅ Machine appears as "ready" in pool
4. ✅ Environment creates on Mac (not Linux)
5. ✅ WireGuard tunnel establishes
6. ✅ DNS resolves `.test` domains to Mac IPs
7. ✅ HTTP request reaches nginx on Mac
8. ✅ Cleanup removes everything

## Security Notes

- API keys are stored in `~/.cilo/cloud-auth.json`
- SSH keys are used only for initial agent installation
- WireGuard provides encrypted tunnels
- Agent listens only on WireGuard interface (not public)
- Server authenticates all requests with API keys

## Next Steps

Once this test passes, you can:

1. **Add more machines** - Register additional Macs or Linux boxes
2. **Test concurrent environments** - Create multiple envs on different machines
3. **Test CI/CD integration** - Use `--ci` flag for automated workflows
4. **Enable billing** - Set `CILO_BILLING_ENABLED=true` to meter usage
5. **Add cloud provider** - Switch to Hetzner for auto-provisioning VMs

## Full Architecture Validation

This test validates the complete distributed architecture:

- ✅ **Parser abstraction** - EnvironmentSpec works remotely
- ✅ **Runtime abstraction** - Docker runtime on Mac controlled from Linux
- ✅ **State management** - PostgreSQL tracks cross-machine state
- ✅ **Networking** - WireGuard + DNS works end-to-end
- ✅ **Security** - API keys + SSH + WireGuard encryption
- ✅ **Scalability** - Machine pool can grow beyond single host

You're running a real distributed dev environment platform on your local network!
