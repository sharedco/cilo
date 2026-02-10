# [ARCHIVED] Draft: Cloud/Remote PRD vs Implementation Analysis
# Plan generated: .sisyphus/plans/cloud-redesign.md
# This draft is no longer needed.

## Investigation Status
- [x] Full CLI command tree mapped
- [x] Cloud client and server API understood
- [x] WireGuard tunnel system understood
- [x] Local lifecycle (create/up/down/destroy/run) understood
- [x] Cloud lifecycle (cloud up/down/destroy/login/logout) understood
- [x] DNS system understood
- [x] Domain models (local vs cloud) understood

---

## KEY FINDING: Two Completely Separate Worlds

### What's Implemented Today (The Bifurcated Model)

The current codebase has **two entirely separate command trees** with **duplicate concepts**:

**Local Commands:**
```
cilo create <name>        → create local env
cilo up <name>            → start local env
cilo down <name>          → stop local env
cilo destroy <name>       → destroy local env
cilo run <cmd> <env>      → create + up + exec locally
cilo list                 → list local envs
cilo status <name>        → local env status
cilo logs <name>          → local logs
cilo exec <name> <svc>    → exec into local container
```

**Cloud Commands (completely separate namespace):**
```
cilo cloud login           → authenticate
cilo cloud logout          → de-authenticate
cilo cloud up <name>       → create remote env (completely different code path)
cilo cloud down <name>     → stop remote env
cilo cloud destroy <name>  → destroy remote env
cilo cloud status          → remote status (STUB - not implemented)
cilo cloud logs            → remote logs (STUB - not implemented)
cilo cloud connect         → connect to existing (STUB - not implemented)
```

### What The PRD Envisions (The Unified Model)

The PRD proposes a fundamentally different approach:

```
cilo run opencode feat/add-auth              → runs locally by default
cilo run opencode feat/add-auth --on big-box → same command, remote target
cilo up agent-1 --spec ./compose.yaml        → unified up, local or remote
cilo connect devbox.example.com              → connect to a machine (not an env)
cilo disconnect                              → disconnect from machine
cilo remote create --provider hetzner        → provision infrastructure
cilo remote ls                               → list machines
cilo remote destroy big-box                  → destroy infrastructure
```

---

## CONFLICT ANALYSIS

### Conflict 1: `cilo run` Signature
- **Implemented**: `cilo run <command> <env-name> [args...]`
  - Example: `cilo run opencode agent-1 "fix the bug"`
- **PRD**: `cilo run <agent> <task>`
  - Example: `cilo run opencode feat/add-auth`
  - Also supports: `cilo run opencode claude codex feat/add-auth` (racing)
  - And: `cilo run opencode feat/auth feat/payments feat/notifications` (fan-out)
- **Key difference**: PRD uses "task" (branch-like concept), impl uses "env-name" (arbitrary name). PRD allows multiple agents AND multiple tasks in a single command.

### Conflict 2: Cloud is a Namespace vs Cloud is a Flag
- **Implemented**: `cilo cloud up <name>` — entirely separate command with different code path
- **PRD**: `cilo run opencode feat/auth --on big-box` — cloud/remote is just a flag on the same command
- **PRD's `connect`**: Connects to a *machine*, not an *environment*. It's about establishing a persistent tunnel to a remote host so all subsequent commands transparently route there.

### Conflict 3: `connect` / `disconnect` Semantics
- **Implemented**: `cilo cloud connect <env-name>` — connects a second user to an existing cloud environment (multi-user access, NOT IMPLEMENTED)
- **PRD**: `cilo connect devbox.example.com` — establishes a WireGuard tunnel to a remote cilod host. This is machine-level, not environment-level. Once connected, `cilo ls` shows remote envs alongside local ones.

### Conflict 4: Machine Management
- **Implemented**: No `cilo remote` commands at all. Machine provisioning happens implicitly server-side when `cilo cloud up` is called (pool-based).
- **PRD**: Explicit `cilo remote create/ls/destroy` commands for managing cloud machines.

### Conflict 5: DNS Convention
- **Implemented**: `{service}.{env-name}.test` (e.g., `api.agent-1.test`)
- **PRD**: `{agent-id}.{service}.test` (e.g., `agent-1.app.test`)
- **Assessment**: The implemented convention is actually better — it groups by environment name first, which maps more naturally to "which task am I looking at."

### Conflict 6: The Server Architecture
- **Implemented**: Three-tier: CLI → Cloud Server (API/DB) → Agent (on VM). The server is a central coordinator with PostgreSQL, auth, team management, API keys.
- **PRD**: Two-tier: CLI ↔ cilod (daemon on each host). No central server. Each machine runs cilod. The developer connects directly to each host.
- **Assessment**: The PRD's model is simpler and more aligned with the stated goal of "invisible infrastructure." The implemented server adds significant operational overhead.

---

## WHAT THE PRD GETS OBJECTIVELY RIGHT

### 1. Cloud-as-a-Flag is Superior DX
The PRD's `--on big-box` approach is objectively better than a separate `cilo cloud` namespace because:
- **One mental model**: The user thinks "run agent on task" not "is this local or cloud?"
- **No command duplication**: One `cilo up`, one `cilo down`, one `cilo ls` — they work everywhere
- **Gradual adoption**: Start local, add `--on remote` when you need it. Zero learning curve for the remote case.
- **Composability**: `cilo ls` shows everything. No `cilo list` + `cilo cloud status` to see the full picture.

### 2. Machine-Level Connect is Superior
The PRD's `cilo connect devbox.example.com` model (connect to a machine, then all commands route there) is superior because:
- **Persistent relationship**: Connect once, use all commands normally
- **Multi-environment visibility**: Once connected, `cilo ls` shows all envs on that machine
- **Simpler mental model**: "I have machines. Machines run environments." vs "I have cloud environments that magically appear on machines I never see."

### 3. `cilo remote` for Machine Lifecycle is Cleaner
Explicit `cilo remote create/destroy` separates concerns:
- **Infrastructure provisioning** (create/destroy VMs) is separate from
- **Environment management** (up/down agents on those VMs)
Currently these are conflated — `cloud up` implicitly provisions infrastructure.

### 4. The Core Bet is Right
> The user thinks in terms of agents and tasks, not containers and networks.

The PRD nails this. The implementation currently forces users to think about "local vs cloud" which is exactly the kind of infrastructure detail the product should hide.

---

## WHAT THE IMPLEMENTATION GETS RIGHT (OR BETTER)

### 1. DNS Convention
`{service}.{env}.test` > `{agent}.{service}.test`
- Groups by environment/task first, which is what you care about
- More natural: "show me everything for this task"
- The implementation's convention is used consistently and works well

### 2. Auth/Team Model
The server-side auth (API keys, teams) enables multi-user scenarios and CI/CD integration that the PRD doesn't address well. The `--ci` mode is a real-world need.

### 3. WireGuard Implementation is Solid
The tunnel daemon, key exchange, and split DNS are well-implemented. The infrastructure for remote connectivity exists and works.

### 4. Shared Services
`--shared postgres` is well-implemented with connect/disconnect lifecycle. The PRD mentions it but the implementation is ahead.

---

## THE CORE IDEA: SIMPLIFICATION

You said: "the core idea is the simplification of what it means to have things on the cloud and consolidate the commands so cloud is more a flag."

And: "the cloud or remote feature is really just a purpose of giving another machine the ability to easily view the status of it and quickly ssh into the machine at the right spot or hop into an opencode session, or attach an opencode session to that."

This reframes the remote feature as:
1. **Visibility**: See what's running on remote machines from your local CLI
2. **Access**: Quickly SSH/attach to the right place
3. **Transparent routing**: `--on` makes any command target a remote machine

This is drastically simpler than what's implemented (a full cloud platform with server, auth, VM provisioning, etc.)

---

## OPTIMAL DX SYNTHESIS

Based on both the PRD and what's implemented well:

### Commands (Proposed)
```
# Core workflow (works local or remote)
cilo run <agent> <task> [--on <machine>]
cilo up <name> [--on <machine>]
cilo down <name>
cilo destroy <name>
cilo ls [--all]                    # shows local AND connected remote envs
cilo logs <name> [service]
cilo exec <name> [service] [cmd]

# Machine connectivity
cilo connect <host>                # establish tunnel to remote cilod
cilo disconnect [<host>]           # tear down tunnel
cilo machines                      # list connected machines + their status

# Machine provisioning (optional, power-user)
cilo remote create --provider hetzner --size cpx51 --name big-box
cilo remote ls
cilo remote destroy big-box

# Auth (needed for remote, but simpler)
cilo login <host>                  # auth with a specific cilod
cilo logout [<host>]
```

### Key Design Decisions
1. **`--on` flag on core commands** — not a separate `cloud` namespace
2. **`cilo connect` is machine-level** — establishes persistent tunnel
3. **`cilo ls` is unified** — shows everything, tagged by machine
4. **`cilo remote` is for infrastructure** — separate from env management
5. **No central server required** — cilod on each machine is the API
6. **Keep DNS convention** — `{service}.{env}.test` (current, it's good)
7. **Keep WireGuard infra** — it works, just wire it differently

## User Decisions
- Racing/fan-out: OUT OF SCOPE for this redesign
- Connect model: EXPLICIT connect only. No auto-connect. Menubar shows connected machines + their environments + lets you open sessions.
- Architecture: MESH OF CILODS. No central server. Each machine is self-contained. CLI connects directly.
- Existing cloud flows: TEAR AWAY. Clean break, not migration. They don't work well.
- Provisioning server: NOT NEEDED. CLI can call cloud APIs directly if needed.
- Plan scope: FULL REDESIGN of remote feature. All phases collapsed into one plan.

## ARCHITECTURE DECISION: Mesh of cilods

```
Mac (cilo CLI + menubar)
  ├── WireGuard → machine-a (cilod) → manages its own envs
  ├── WireGuard → machine-b (cilod) → manages its own envs  
  └── local Docker → manages local envs

cilo ls → aggregates all sources
cilo run opencode feat-auth --on machine-a → routes to machine-a's cilod
```

No central server. No PostgreSQL. No team management layer.
Each cilod is self-contained. CLI connects directly to each.

## DEEP OBJECTIVE ANALYSIS

### The Architecture Question: What Actually Matters

The current 3-tier model (CLI → cloud server → agent) was built to solve:
1. VM provisioning coordination
2. WireGuard key exchange brokering
3. Team/auth management
4. Environment state persistence

But the user's reframed purpose of "remote" is much simpler:
> "The remote feature is really just giving another machine the ability to easily view status and quickly SSH into the machine at the right spot or hop into an opencode session."

This is a VISIBILITY + ACCESS problem, not an ORCHESTRATION problem.

### Recommendation: cilod IS the server

Every machine running environments should run cilod. cilod serves:
- Environment lifecycle (up/down/destroy)
- Status API
- WireGuard peer management
- Log streaming

The CLI connects directly to each cilod. No central coordinator needed for the core use case.

For VM provisioning specifically (Hetzner API etc), that can be a separate concern — either a `cilo remote create` command that provisions + installs cilod, or just documentation on how to set up a machine.

### The Connect Model (Explicit, Persistent, Machine-Level)

`cilo connect devbox.example.com` should work like mounting a remote filesystem:
1. Establish WireGuard tunnel to that machine's cilod
2. Register it as a "connected machine" locally
3. Start receiving status from its cilod
4. All envs on that machine become visible in `cilo ls` and menubar
5. DNS entries route through the tunnel

Once connected, `--on devbox` on any command targets that machine.
If NOT connected, `--on devbox` errors: "Not connected to devbox. Run: cilo connect devbox"

### What Code Survives vs Gets Rewritten

**KEEP (solid infrastructure):**
- WireGuard tunnel daemon (cloud/tunnel/*)
- DNS system (dns/*)
- Compose transform/loader (compose/*)
- Engine/parser detection (engine/*)
- Runtime/Docker provider (runtime/*, runtimes/*)
- State management core (state/*)
- Share manager (share/*)
- Copy-on-write filesystem (filesystem/*)
- Agent environment manager (agent/environment.go)
- Agent WireGuard manager (agent/wireguard.go)

**REWRITE (CLI surface):**
- All cli/cloud_*.go files → merge into core commands with --on flag
- cli/tunnel.go → becomes part of `cilo connect/disconnect`
- cli/commands.go → add remote awareness
- cli/lifecycle.go → add --on routing
- cli/run.go → add --on routing

**REWORK (server → cilod merge):**
- server/api/* → becomes cilod API surface
- cloud/client.go → becomes cilod client (direct to machine, not central server)
