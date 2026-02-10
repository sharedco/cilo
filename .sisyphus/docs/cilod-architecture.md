# How cilod and cilo connect Work

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           YOUR MAC (CLI)                            │
│                                                                     │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐      │
│  │  cilo CLI    │──────│ cilod Client │──────│ WireGuard    │      │
│  │              │      │ (HTTP/HTTPS) │      │   Tunnel     │      │
│  └──────────────┘      └──────────────┘      └──────┬───────┘      │
│         │                                            │              │
│         │ cilo create myenv --on big-box            │              │
│         │ cilo up myenv --on big-box               │              │
│         │ cilo connect big-box.example.com ────────┘              │
│         │                                                            │
└─────────┼────────────────────────────────────────────────────────────┘
          │                           WireGuard Tunnel (encrypted)
          │                           Your Mac IP: 10.225.0.2/32
          │                           Big Box IP: 10.225.0.1/32
          │
┌─────────┼────────────────────────────────────────────────────────────┐
│         │                      REMOTE MACHINE (big-box)               │
│         │                                                            │
│         │         ┌──────────────────────────────────────┐           │
│         └────────►│           cilod daemon              │           │
│                   │  (upgraded agent, runs on :8081)    │           │
│                   └──────────────────────────────────────┘           │
│                                    │                                 │
│                                    │ manages                         │
│                                    ▼                                 │
│                   ┌──────────────────────────────────────┐           │
│                   │    Docker/Podman/Container Runtime    │           │
│                   │         (manages containers)          │           │
│                   └──────────────────────────────────────┘           │
│                                    │                                 │
│                    ┌───────────────┼───────────────┐                │
│                    ▼               ▼               ▼                │
│            ┌──────────┐   ┌──────────┐   ┌──────────┐              │
│            │  myenv   │   │  myenv   │   │  myenv   │              │
│            │  nginx   │   │   api    │   │  redis   │              │
│            └──────────┘   └──────────┘   └──────────┘              │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

## Key Concepts

### 1. cilod (cilod Daemon)

**What it is:**
- An HTTP server that runs on every machine that hosts cilo environments
- It's the upgraded version of the "agent" binary (`cmd/cilo-agent`)
- Listens on port 8081 by default

**What it does:**
- **Environment Management:** Create, start, stop, destroy Docker environments
- **WireGuard Management:** Handle VPN peer connections for remote access
- **Status API:** Report what environments are running and their status
- **Log Streaming:** Stream container logs via WebSocket
- **Exec:** Execute commands in containers via WebSocket

**API Endpoints:**
```
POST   /auth/connect              — SSH key auth (get session token)
GET    /environments              — List all environments on this machine
POST   /environments/:name/up     — Create and start environment
POST   /environments/:name/down  — Stop environment
DELETE /environments/:name       — Destroy environment
GET    /environments/:name/status — Get environment status
GET    /environments/:name/logs   — Stream logs (WebSocket)
POST   /environments/:name/exec  — Exec into container (WebSocket)
POST   /wireguard/exchange        — Exchange WireGuard keys
POST   /sync/:name                — Receive workspace files
```

### 2. cilo connect

**What it does:**
Establishes a persistent WireGuard VPN tunnel between your Mac and a remote machine running cilod.

**The Flow:**

```
Step 1: You run "cilo connect big-box.example.com"
        │
        ▼
Step 2: CLI reads your SSH public key (~/.ssh/id_ed25519.pub)
        │
        ▼
Step 3: CLI calls cilod /auth/connect with your SSH key
        │
        ▼
Step 4: cilod verifies SSH key against its authorized_keys
        │
        ▼
Step 5: cilod returns a session token + WireGuard config
        │
        ▼
Step 6: CLI generates WireGuard keypair
        │
        ▼
Step 7: CLI calls cilod /wireguard/exchange with WG public key
        │
        ▼
Step 8: cilod assigns you an IP (e.g., 10.225.0.2/32) and returns
        its WG public key + server IP (10.225.0.1/32)
        │
        ▼
Step 9: CLI starts WireGuard tunnel daemon
        │
        ▼
Step 10: Tunnel established! Your Mac can now reach cilod at 10.225.0.1
         All DNS for .test domains routes through this tunnel
```

**What's stored locally:**
```bash
~/.cilo/machines/big-box.example.com/
├── state.json          # Host, token, assigned WG IP
└── wg-key              # Your WireGuard private key
```

### 3. cilo up --on big-box

**What happens:**

```
Step 1: You run "cilo up myenv --on big-box"
        │
        ▼
Step 2: CLI checks if big-box is connected (reads ~/.cilo/machines/)
        If not connected → error: "Run 'cilo connect big-box' first"
        │
        ▼
Step 3: CLI creates cilod Client with:
        - Host: big-box's WireGuard IP (10.225.0.1)
        - Token: from state.json
        │
        ▼
Step 4: CLI syncs workspace to remote machine
        (rsync over SSH through WireGuard tunnel)
        │
        ▼
Step 5: CLI calls cilod POST /environments/myenv/up
        │
        ▼
Step 6: cilod on big-box:
        - Creates Docker network
        - Starts containers
        - Updates its local DNS
        - Returns success
        │
        ▼
Step 7: CLI updates local DNS to include remote services
        (so you can curl http://nginx.myenv.test from your Mac)
        │
        ▼
Step 8: Environment is running on big-box, accessible from your Mac!
```

### 4. DNS Resolution Flow

```
Your Mac wants to resolve: nginx.myenv.test
        │
        ▼
Check: Is this a local or remote environment?
        │
        ├─ Local → Use local dnsmasq (port 5354)
        │
        └─ Remote (on big-box) → Route through WireGuard tunnel
                │
                ▼
        Query goes through tunnel to cilod's DNS
                │
                ▼
        cilod responds with container IP (10.225.0.X)
                │
                ▼
        Your Mac connects through tunnel to that IP
```

## The Complete Picture

**Without --on (local):**
```
cilo up myenv
  └─► Runs Docker locally on your Mac
      DNS: nginx.myenv.test → 10.224.X.Y (local Docker IP)
```

**With --on (remote):**
```
cilo up myenv --on big-box
  ├─► Syncs files to big-box (rsync over WireGuard)
  ├─► Calls cilod API on big-box
  ├─► cilod starts containers on big-box
  └─► DNS: nginx.myenv.test → 10.225.X.Y (through tunnel)

Result: Environment runs on big-box, feels local on your Mac
```

**Key Benefits:**
1. **Transparent:** Same commands work local or remote
2. **Secure:** WireGuard encryption for all traffic
3. **DNS Just Works:** .test domains resolve correctly
4. **No Port Forwarding:** Everything through single tunnel
5. **Multiple Machines:** Connect to many remote machines, manage all from one CLI

## Summary

**cilod** = HTTP API server on each machine (manages Docker, WireGuard)
**cilo connect** = Establishes WireGuard tunnel to a machine
**--on flag** = Routes commands through tunnel to cilod on that machine

Together they let you run `cilo up myenv --on gpu-server` and have it feel exactly like running locally, but on a remote machine.
