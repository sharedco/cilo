# Cloud Redesign: Mesh of cilods with Unified CLI

## TL;DR

> **Quick Summary**: Replace the current bifurcated cloud system (separate `cilo cloud *` namespace + central server + agent) with a mesh architecture where each machine runs cilod and the CLI connects directly. Remote becomes a `--on <machine>` flag on existing commands, not a separate command tree.
> 
> **Deliverables**:
> - `cilo connect <host>` / `cilo disconnect` commands with WireGuard tunneling
> - `--on <machine>` flag on `run`, `up`, `down`, `destroy`, `exec`, `logs`
> - Unified `cilo ls` showing local + all connected machines
> - cilod upgraded to serve full API (env CRUD, WG exchange, status, logs, exec)
> - SSH key-based auth between CLI and cilod
> - All `cilo cloud *` commands and central server code deleted
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Task 1 (delete old cloud) → Task 3 (cilod API) → Task 5 (connect) → Task 7 (--on flag) → Task 9 (unified ls)

---

## Context

### Original Request
Redesign cilo's remote/cloud feature based on a re-envisioned PRD. The core idea: simplify what "cloud" means by making it a flag (`--on machine`) rather than a separate command namespace. Remote is about visibility and access — giving another machine the ability to easily view status and quickly SSH/attach to environments.

### Interview Summary
**Key Discussions**:
- Current implementation has two separate command trees (local and cloud) with completely different code paths — objectively worse DX
- The PRD's `--on` flag approach is objectively superior: one mental model, no duplicate commands
- Architecture: mesh of cilods, no central server. Each machine is self-contained.
- Auth: SSH key-based. If you can SSH to a machine, you can connect to its cilod.
- Existing cloud code: tear away completely. Clean break.
- Connect model: explicit `cilo connect <host>` only. Menubar shows connected machines.
- Racing/fan-out: out of scope
- Tests: TDD

**Research Findings**:
- Agent already has HTTP server (:8081) with env lifecycle and WG routes — good foundation for cilod
- WireGuard tunnel daemon (macOS userspace, Linux kernel) is solid — reuse entirely
- DNS system (dnsmasq + split resolver) works — needs extension for remote envs
- Server's WG exchange logic is useful — merge into cilod, but the PostgreSQL dependency must go
- 7 cloud CLI files (~1,200 lines) to delete
- 3 cloud command stubs (status, logs, connect) that were never implemented

### Metis Review
**Identified Gaps** (addressed):
- Auth protocol needs concrete specification (addressed in Task 2)
- IP allocation in mesh without central coordinator (addressed: cilod allocates from its own pool)
- DNS for remote environments (addressed: extend dnsmasq with entries from connected cilods)
- State migration for existing cloud users (addressed: cleanup guide in Task 1)
- WireGuard key persistence vs ephemeral (addressed: persistent keys per connect)
- PTY/streaming for remote exec/logs (addressed: cilod uses WebSocket upgrade for streams)
- Same env name on different machines (addressed: machine-scoped in `cilo ls` output)

---

## Work Objectives

### Core Objective
Replace the bifurcated local/cloud CLI with a unified command set where remote targeting is a `--on <machine>` flag, backed by a mesh of self-contained cilod instances that the CLI connects to directly via WireGuard.

### Concrete Deliverables
- `cilo connect <host>` command — establish WireGuard tunnel + register machine
- `cilo disconnect [host]` command — tear down tunnel + deregister
- `cilo machines` command — list connected machines with status
- `--on <machine>` flag on: `run`, `up`, `down`, `destroy`, `exec`, `logs`
- Unified `cilo ls` — shows local + all connected machine environments
- cilod binary — upgraded agent serving full REST API with SSH key auth
- Deletion of all `cilo cloud *` commands and `internal/server/*` package
- cilod client library for CLI ↔ cilod communication

### Definition of Done
- [ ] `cilo run opencode feat-auth --on devbox` creates env on remote machine (test: e2e_remote_test.go)
- [ ] `cilo ls` shows both local and connected machine environments (test: unified_ls_test.go)
- [ ] `cilo connect devbox.example.com` establishes WireGuard tunnel (test: connect_test.go)
- [ ] All local commands work unchanged without `--on` flag (test: regression_test.go)
- [ ] No `cilo cloud` subcommand exists (test: `cilo cloud` returns "unknown command")
- [ ] `go test ./...` passes with zero failures

### Must Have
- Unified CLI surface (no `cilo cloud` namespace)
- `--on` flag on all lifecycle commands
- `cilo connect/disconnect` for machine management
- SSH key-based auth between CLI and cilod
- Workspace sync (rsync) for `--on` operations
- DNS resolution for remote environment services
- WireGuard tunnel persistence (survives CLI restarts)
- Error messaging: `--on` without connect gives clear "run cilo connect first" error

### Must NOT Have (Guardrails)
- No central server or coordinator process
- No auto-connect (always explicit `cilo connect`)
- No racing/fan-out (multi-agent is out of scope)
- No bidirectional workspace sync (one-way: local → remote only)
- No file watching or live sync
- No environment sharing between users
- No OAuth/JWT/mTLS — SSH keys only for auth
- No health check endpoints beyond `/health` on cilod
- No metrics/prometheus endpoints on cilod
- No changes to local command behavior (regression prevention)
- No implementation of features that were stubs in old cloud (the old cloud_status, cloud_logs, cloud_connect were never implemented — don't port unbuilt features)

---

## Verification Strategy (MANDATORY)

> **UNIVERSAL RULE: ZERO HUMAN INTERVENTION**
>
> ALL tasks in this plan MUST be verifiable WITHOUT any human action.

### Test Decision
- **Infrastructure exists**: YES (24 test files, `go test`)
- **Automated tests**: TDD (RED-GREEN-REFACTOR)
- **Framework**: `go test` (standard library)

### If TDD Enabled

Each TODO follows RED-GREEN-REFACTOR:

**Task Structure:**
1. **RED**: Write failing test first
   - Test file: `[path]_test.go`
   - Test command: `go test ./[package] -run TestName`
   - Expected: FAIL (test exists, implementation doesn't)
2. **GREEN**: Implement minimum code to pass
   - Command: `go test ./[package] -run TestName`
   - Expected: PASS
3. **REFACTOR**: Clean up while keeping green
   - Command: `go test ./[package] -run TestName`
   - Expected: PASS (still)

### Agent-Executed QA Scenarios (MANDATORY — ALL tasks)

**Verification Tool by Deliverable Type:**

| Type | Tool | How Agent Verifies |
|------|------|-------------------|
| **CLI commands** | Bash | Run command, parse output, assert exit code and content |
| **cilod API** | Bash (curl) | Send HTTP requests, assert status + response body |
| **WireGuard** | Bash (wg show) | Check interface, peer, handshake status |
| **DNS** | Bash (dig/nslookup) | Query .test domain, assert resolution |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately):
├── Task 1: Delete old cloud code + cleanup guide
└── Task 2: Design cilod auth protocol + API spec

Wave 2 (After Wave 1):
├── Task 3: Upgrade cilod with full API surface
└── Task 4: Build cilod client library

Wave 3 (After Wave 2):
├── Task 5: Implement cilo connect / disconnect
├── Task 6: Implement cilo machines command
└── Task 7: Add --on flag routing to lifecycle commands

Wave 4 (After Wave 3):
├── Task 8: Implement workspace sync for --on
├── Task 9: Unify cilo ls
├── Task 10: Remote DNS integration
└── Task 11: Remote exec + logs streaming

Wave 5 (After Wave 4):
└── Task 12: Integration tests + regression suite
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 | None | 3, 7 | 2 |
| 2 | None | 3, 4, 5 | 1 |
| 3 | 1, 2 | 5, 6, 7 | 4 |
| 4 | 2 | 5, 7, 8, 9, 11 | 3 |
| 5 | 3, 4 | 7, 8, 9, 10, 11 | 6 |
| 6 | 3, 4 | 9 | 5 |
| 7 | 3, 4, 5 | 8, 11 | 6 |
| 8 | 4, 5, 7 | 12 | 9, 10, 11 |
| 9 | 4, 5, 6 | 12 | 8, 10, 11 |
| 10 | 5 | 12 | 8, 9, 11 |
| 11 | 4, 7 | 12 | 8, 9, 10 |
| 12 | 8, 9, 10, 11 | None | None (final) |

### Agent Dispatch Summary

| Wave | Tasks | Recommended Agents |
|------|-------|-------------------|
| 1 | 1, 2 | quick (deletion), ultrabrain (protocol design) |
| 2 | 3, 4 | deep (cilod API), unspecified-high (client lib) |
| 3 | 5, 6, 7 | deep (connect), quick (machines), unspecified-high (--on) |
| 4 | 8, 9, 10, 11 | all parallel, unspecified-high |
| 5 | 12 | deep (integration) |

---

## TODOs

- [ ] 1. Delete Old Cloud Code + Cleanup Guide

  **What to do**:
  - RED: Write test `TestNoCloudSubcommand` that asserts `cilo cloud` returns "unknown command" error
  - GREEN: Delete all `internal/cli/cloud*.go` files (7 files): `cloud.go`, `cloud_up.go`, `cloud_down.go`, `cloud_destroy.go`, `cloud_login.go`, `cloud_logout.go`, `cloud_status.go`, `cloud_logs.go`, `cloud_connect.go`
  - Remove `cloudCmd` registration from `internal/cli/cloud.go` (the `rootCmd.AddCommand(cloudCmd)` in init())
  - Delete `internal/cloud/client.go` and `internal/cloud/client_test.go` (the cloud API client)
  - Delete `internal/cloud/auth.go` and `internal/cloud/auth_test.go` (cloud auth storage)
  - Delete `internal/cloud/sync.go` and `internal/cloud/sync_test.go` (cloud workspace sync — will be rebuilt as generic sync)
  - Delete entire `internal/server/` package (API server, handlers, store, config, auth, agent client, VM providers)
  - Delete `test/e2e/cloud_auth_test.go`
  - Remove `cloudCmd` from root.go init() if referenced
  - REFACTOR: Verify `go build ./...` succeeds with no broken imports
  - Write cleanup guide as comment block in a new `internal/cli/migrate.go` that prints migration instructions when user tries any former cloud command

  **Must NOT do**:
  - Do NOT delete `internal/cloud/tunnel/` — this is the WireGuard tunnel daemon, it's reused
  - Do NOT delete `internal/agent/` — this becomes the foundation for cilod
  - Do NOT modify any local command behavior

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: This is primarily file deletion with careful import cleanup. No complex logic.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — just deleting files, not history manipulation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 3, 7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/cli/cloud.go:27-29` — Where cloudCmd is registered on rootCmd. This is what to remove.
  - `internal/cli/root.go:38-56` — The init() where top-level commands are added. Verify cloudCmd isn't here.

  **Files to Delete**:
  - `internal/cli/cloud.go` — cloud parent command (30 lines)
  - `internal/cli/cloud_up.go` — cloud up implementation (559 lines)
  - `internal/cli/cloud_down.go` — cloud down (166 lines)
  - `internal/cli/cloud_destroy.go` — cloud destroy (176 lines)
  - `internal/cli/cloud_login.go` — cloud login (100 lines)
  - `internal/cli/cloud_logout.go` — cloud logout (49 lines)
  - `internal/cli/cloud_status.go` — cloud status stub (59 lines)
  - `internal/cli/cloud_logs.go` — cloud logs stub (58 lines)
  - `internal/cli/cloud_connect.go` — cloud connect stub (49 lines)
  - `internal/cloud/client.go` — cloud API client (279 lines)
  - `internal/cloud/client_test.go` — cloud client tests
  - `internal/cloud/auth.go` — cloud auth (credential storage)
  - `internal/cloud/auth_test.go` — cloud auth tests
  - `internal/cloud/sync.go` — cloud workspace sync
  - `internal/cloud/sync_test.go` — sync tests
  - `internal/server/` — entire directory (API, handlers, store, config, auth, VM providers, wireguard exchange, agent client)
  - `test/e2e/cloud_auth_test.go` — cloud e2e auth test

  **Files to KEEP in internal/cloud/**:
  - `internal/cloud/tunnel/tunnel.go` — WireGuard tunnel abstraction
  - `internal/cloud/tunnel/daemon.go` — tunnel daemon
  - `internal/cloud/tunnel/manager.go` — Linux WG management
  - `internal/cloud/tunnel/manager_darwin.go` — macOS WG management
  - `internal/cloud/tunnel/keys.go` — key generation
  - `internal/cloud/tunnel/types.go` — type definitions
  - `internal/cloud/tunnel/interface_darwin.go` — macOS interface ops
  - `internal/cloud/tunnel/interface_linux.go` — Linux interface ops
  - `internal/cloud/tunnel/tunnel_test.go` — tunnel tests
  - `internal/cloud/tunnel/keys_test.go` — key tests

  **Acceptance Criteria**:

  - [ ] `go build ./cmd/cilo` succeeds with no errors
  - [ ] `go test ./...` passes (excluding tests that depended on deleted code)
  - [ ] `go vet ./...` clean
  - [ ] Running `cilo cloud` prints "unknown command" or equivalent error
  - [ ] Running `cilo cloud up test` prints "unknown command" or equivalent error
  - [ ] `internal/server/` directory does not exist
  - [ ] `internal/cloud/client.go` does not exist
  - [ ] `internal/cloud/tunnel/` directory still exists with all tunnel files

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Cloud commands are removed
    Tool: Bash
    Preconditions: cilo binary built from modified source
    Steps:
      1. go build -o /tmp/cilo-test ./cmd/cilo
      2. /tmp/cilo-test cloud 2>&1
      3. Assert: exit code != 0
      4. Assert: output contains "unknown command" or "not found"
      5. /tmp/cilo-test cloud up test 2>&1
      6. Assert: exit code != 0
    Expected Result: All cloud subcommands return errors
    Evidence: Command output captured

  Scenario: Local commands still work
    Tool: Bash
    Preconditions: cilo binary built
    Steps:
      1. /tmp/cilo-test --help
      2. Assert: output contains "run", "up", "down", "destroy", "list"
      3. Assert: output does NOT contain "cloud"
    Expected Result: Local commands present, cloud absent from help
    Evidence: Help output captured

  Scenario: Build and tests pass
    Tool: Bash
    Steps:
      1. go build ./...
      2. Assert: exit code 0
      3. go vet ./...
      4. Assert: exit code 0
      5. go test ./... 2>&1
      6. Assert: no FAIL lines (some tests may be skipped)
    Expected Result: Clean build and test
    Evidence: Build and test output captured
  ```

  **Commit**: YES
  - Message: `refactor(cli): remove cloud command namespace and central server code`
  - Files: all deleted files + modified root.go
  - Pre-commit: `go build ./... && go vet ./...`

---

- [ ] 2. Design cilod Auth Protocol + API Specification

  **What to do**:
  - RED: Write test `TestSSHKeyAuth` in `internal/agent/auth_test.go` that:
    - Creates a mock SSH key pair
    - Tests that a request with valid SSH signature is accepted
    - Tests that a request without auth is rejected (401)
    - Tests that a request with invalid signature is rejected (403)
  - GREEN: Implement SSH key-based auth for cilod:
    - During `cilo connect`, CLI reads user's SSH public key (`~/.ssh/id_ed25519.pub` or `~/.ssh/id_rsa.pub`)
    - CLI signs a challenge/nonce using the SSH private key
    - cilod verifies against its `~/.ssh/authorized_keys`
    - On success, cilod issues a session token (simple bearer token with expiry)
    - Subsequent requests use `Authorization: Bearer <token>`
  - Write API specification as Go interface + types in `internal/agent/api.go`:
    ```
    POST   /auth/connect          — SSH key exchange, returns session token
    DELETE /auth/disconnect        — Invalidate session

    GET    /environments           — List all environments on this machine
    POST   /environments/:name/up  — Create + start environment
    POST   /environments/:name/down — Stop environment
    DELETE /environments/:name     — Destroy environment
    GET    /environments/:name/status — Get env status with services
    GET    /environments/:name/logs  — Stream logs (WebSocket upgrade)
    POST   /environments/:name/exec — Exec into container (WebSocket upgrade)

    POST   /wireguard/exchange     — WireGuard key exchange
    DELETE /wireguard/peers/:key   — Remove peer
    GET    /wireguard/status       — WireGuard interface status

    POST   /sync/:name             — Receive workspace sync (rsync endpoint or tar upload)
    ```
  - Define request/response types as Go structs
  - Define IP allocation strategy: cilod maintains a simple JSON file (`/var/cilo/peers.json`) mapping peer public keys to assigned IPs from a configurable subnet (default: `10.225.0.0/24` per machine)
  - REFACTOR: Ensure the interface is minimal — only what's needed for --on flag support

  **Must NOT do**:
  - Do NOT implement the API yet — this task is DESIGN ONLY (types, interfaces, tests)
  - Do NOT add OAuth, JWT libraries, or complex auth
  - Do NOT add health check, metrics, or discovery endpoints
  - Do NOT design team/multi-user features

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Protocol design requires careful security thinking and API ergonomics
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: Tasks 3, 4, 5
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/agent/server.go:21-40` — Existing agent HTTP server structure. New API extends this.
  - `internal/agent/handlers.go` — Existing handler patterns. Follow same style.
  - `internal/server/api/handlers/wireguard.go:1-50` — WireGuard exchange logic to adapt for cilod
  - `internal/server/wireguard/exchange.go:61-120` — Peer registration logic (replace PostgreSQL with JSON file)

  **API/Type References**:
  - `internal/agent/types.go` — Existing agent types. Extend for new API.
  - `internal/cloud/client.go:46-115` — Request/response types from old cloud client. Use as reference for new cilod types.

  **External References**:
  - SSH agent protocol: `golang.org/x/crypto/ssh` — for SSH key verification
  - SSH agent signing: `golang.org/x/crypto/ssh/agent` — for signing challenges with SSH agent

  **Acceptance Criteria**:

  - [ ] `internal/agent/api.go` exists with interface definition and request/response types
  - [ ] `internal/agent/auth.go` exists with auth types and SSH verification interface
  - [ ] `internal/agent/auth_test.go` exists with tests for SSH key auth (RED state — tests fail because implementation is stub)
  - [ ] `go build ./internal/agent/...` succeeds
  - [ ] API interface covers: auth, environment CRUD, logs, exec, wireguard, sync
  - [ ] IP allocation strategy documented in code comments

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: API types compile and tests exist
    Tool: Bash
    Steps:
      1. go build ./internal/agent/...
      2. Assert: exit code 0
      3. go test ./internal/agent/ -run TestSSHKeyAuth -v 2>&1
      4. Assert: output contains "FAIL" (tests should fail because implementation is stub)
      5. Assert: test names exist in output (proves tests were written)
    Expected Result: Types compile, auth tests exist but fail (RED state)
    Evidence: Build + test output captured

  Scenario: API interface is complete
    Tool: Bash (ast-grep or grep)
    Steps:
      1. grep -c "func.*Handler" internal/agent/api.go
      2. Assert: at least 10 handler definitions
      3. grep "environments" internal/agent/api.go
      4. Assert: contains CRUD operations
      5. grep "wireguard" internal/agent/api.go
      6. Assert: contains exchange endpoint
    Expected Result: API spec covers all required endpoints
    Evidence: Grep output captured
  ```

  **Commit**: YES
  - Message: `feat(agent): design cilod API specification and auth protocol`
  - Files: `internal/agent/api.go`, `internal/agent/auth.go`, `internal/agent/auth_test.go`
  - Pre-commit: `go build ./internal/agent/...`

---

- [ ] 3. Upgrade cilod with Full API Surface

  **What to do**:
  - RED: Write tests in `internal/agent/server_test.go` for each API endpoint:
    - `TestListEnvironments` — GET /environments returns JSON array
    - `TestUpEnvironment` — POST /environments/:name/up creates and starts
    - `TestDownEnvironment` — POST /environments/:name/down stops
    - `TestDestroyEnvironment` — DELETE /environments/:name destroys
    - `TestGetEnvironmentStatus` — GET /environments/:name/status returns services
    - `TestWireGuardExchange` — POST /wireguard/exchange returns peer config
    - `TestAuthMiddleware` — Requests without valid token return 401
  - GREEN: Implement each endpoint:
    - Wire API routes into existing `internal/agent/server.go` chi router
    - Implement auth middleware that validates session tokens
    - Implement SSH key auth endpoint (`/auth/connect`)
    - Adapt environment handlers to use existing `EnvironmentManager`
    - Port WireGuard exchange logic from `internal/server/wireguard/exchange.go` — replace PostgreSQL storage with local JSON file (`/var/cilo/peers.json`)
    - IP allocation: cilod maintains a counter file, assigns 10.225.0.{N}/32 to each new peer
    - Implement WebSocket upgrade for log streaming and exec
  - REFACTOR: Ensure error responses are consistent JSON: `{"error": "message"}`

  **Must NOT do**:
  - Do NOT add endpoints beyond what's in the API spec (Task 2)
  - Do NOT add PostgreSQL or any external database
  - Do NOT add team management or multi-user isolation
  - Do NOT break existing agent functionality (health, existing proxy)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex API implementation requiring careful integration with existing agent code
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 5, 6, 7
  - **Blocked By**: Tasks 1, 2

  **References**:

  **Pattern References**:
  - `internal/agent/server.go:21-80` — Existing server setup with chi router. Add new routes here.
  - `internal/agent/handlers.go` — Existing handler pattern. Follow this style for new handlers.
  - `internal/agent/environment.go:30-180` — EnvironmentManager with Up/Down/Destroy/Status methods. Wire these into API handlers.
  - `internal/agent/wireguard.go:20-60` — WireGuardManager with AddPeer/RemovePeer. Wire into /wireguard/* handlers.
  - `internal/agent/proxy.go:15-80` — EnvProxy that registers routes per environment. Ensure API and proxy don't conflict.

  **API/Type References**:
  - `internal/agent/api.go` (from Task 2) — The API spec defining all endpoints, types, and interfaces
  - `internal/agent/auth.go` (from Task 2) — Auth types and verification interface

  **Code to Port (from deleted server)**:
  - `internal/server/wireguard/exchange.go:61-120` — Peer registration logic. Replace `store.RegisterPeer()` with local JSON file operations.
  - `internal/server/api/handlers/wireguard.go:30-80` — Handler structure for WG exchange. Adapt request/response for cilod.

  **Acceptance Criteria**:

  - [ ] All tests from RED phase pass (GREEN): `go test ./internal/agent/ -v` → all PASS
  - [ ] Auth endpoint works: POST /auth/connect with valid SSH key → 200 with token
  - [ ] Auth rejection works: GET /environments without token → 401
  - [ ] Environment CRUD works: up/down/destroy endpoints function correctly
  - [ ] WireGuard exchange works: POST /wireguard/exchange → returns peer config with assigned IP
  - [ ] IP allocation persists: peers.json written to disk, survives cilod restart
  - [ ] `go build ./cmd/cilod` (or agent binary) succeeds

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: cilod API responds to authenticated requests
    Tool: Bash (curl)
    Preconditions: cilod binary built and running on localhost:8081
    Steps:
      1. curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/environments
      2. Assert: HTTP status is 401 (no auth)
      3. curl -s -X POST http://localhost:8081/auth/connect -d '{"public_key":"test-key"}' -H "Content-Type: application/json"
      4. Extract token from response
      5. curl -s -w "\n%{http_code}" -H "Authorization: Bearer ${TOKEN}" http://localhost:8081/environments
      6. Assert: HTTP status is 200
      7. Assert: response is JSON array (may be empty)
    Expected Result: Auth required, authenticated requests succeed
    Evidence: Response bodies captured

  Scenario: WireGuard exchange returns valid config
    Tool: Bash (curl)
    Preconditions: cilod running with auth token
    Steps:
      1. curl -s -X POST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
           http://localhost:8081/wireguard/exchange \
           -d '{"public_key":"test-wg-pubkey","environment_id":"test-env"}'
      2. Assert: HTTP status 200
      3. Assert: response contains "assigned_ip"
      4. Assert: response contains "server_public_key"
      5. Assert: assigned_ip matches 10.225.0.x pattern
    Expected Result: Valid WG peer config returned
    Evidence: Response body captured
  ```

  **Commit**: YES
  - Message: `feat(agent): implement cilod full API surface with SSH auth`
  - Files: `internal/agent/server.go`, `internal/agent/handlers.go`, `internal/agent/auth.go`, `internal/agent/server_test.go`
  - Pre-commit: `go test ./internal/agent/ -v`

---

- [ ] 4. Build cilod Client Library

  **What to do**:
  - RED: Write tests in `internal/cilod/client_test.go`:
    - `TestClientConnect` — client connects to cilod, gets token
    - `TestClientListEnvironments` — client lists envs
    - `TestClientUpEnvironment` — client creates env
    - `TestClientWireGuardExchange` — client exchanges WG keys
    - `TestClientError` — client handles 4xx/5xx gracefully
  - GREEN: Create `internal/cilod/client.go` — HTTP client for talking to cilod instances:
    - `NewClient(host string, token string) *Client`
    - `Connect(sshPublicKey string) (token string, err error)` — SSH key auth
    - `ListEnvironments() ([]Environment, error)`
    - `UpEnvironment(name string, opts UpOptions) error`
    - `DownEnvironment(name string) error`
    - `DestroyEnvironment(name string) error`
    - `GetStatus(name string) (*EnvironmentStatus, error)`
    - `StreamLogs(name string, service string) (io.ReadCloser, error)`
    - `Exec(name string, service string, cmd []string) error`
    - `WireGuardExchange(publicKey string) (*WGConfig, error)`
    - `SyncWorkspace(name string, localPath string) error` — rsync over SSH
  - Define shared types in `internal/cilod/types.go` (can import from agent/api.go types)
  - REFACTOR: Add timeout handling, retry logic for transient failures

  **Must NOT do**:
  - Do NOT add caching or connection pooling
  - Do NOT add discovery/registry features
  - Do NOT add multi-user support

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Client library with HTTP/WebSocket, solid error handling needed
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: Tasks 5, 7, 8, 9, 11
  - **Blocked By**: Task 2

  **References**:

  **Pattern References**:
  - `internal/cloud/client.go:18-34` (DELETED in Task 1 — read before deletion) — HTTP client pattern with NewClient, get/post/delete helpers. Follow same pattern.
  - `internal/cloud/client.go:214-278` — HTTP helper methods (get, post, delete, do). Reuse this pattern.

  **API/Type References**:
  - `internal/agent/api.go` (from Task 2) — Defines all endpoint contracts. Client must match.

  **Acceptance Criteria**:

  - [ ] `internal/cilod/client.go` exists with all methods matching API spec
  - [ ] `internal/cilod/types.go` exists with shared types
  - [ ] `internal/cilod/client_test.go` passes: `go test ./internal/cilod/ -v` → all PASS
  - [ ] Client handles auth (sends Bearer token), error responses, timeouts
  - [ ] `go build ./internal/cilod/...` succeeds

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Client library compiles and tests pass
    Tool: Bash
    Steps:
      1. go build ./internal/cilod/...
      2. Assert: exit code 0
      3. go test ./internal/cilod/ -v
      4. Assert: all tests PASS
    Expected Result: Client library functional
    Evidence: Test output captured
  ```

  **Commit**: YES
  - Message: `feat(cilod): add client library for CLI-to-cilod communication`
  - Files: `internal/cilod/client.go`, `internal/cilod/types.go`, `internal/cilod/client_test.go`
  - Pre-commit: `go test ./internal/cilod/ -v`

---

- [ ] 5. Implement `cilo connect` / `cilo disconnect`

  **What to do**:
  - RED: Write tests in `internal/cli/connect_test.go`:
    - `TestConnectNewMachine` — connects to a host, registers in machines state
    - `TestConnectAlreadyConnected` — error if already connected
    - `TestConnectUnreachable` — error if host unreachable
    - `TestDisconnectConnected` — disconnects, removes from state
    - `TestDisconnectNotConnected` — error if not connected
  - GREEN: Implement `cilo connect <host>` in `internal/cli/connect.go`:
    1. Resolve host (DNS or IP)
    2. Read user's SSH public key
    3. Call cilod `/auth/connect` with SSH key to get session token
    4. Generate WireGuard key pair (persistent — store in `~/.cilo/machines/<host>/wg-key`)
    5. Call cilod `/wireguard/exchange` with WG public key
    6. Start WireGuard tunnel daemon (reuse `internal/cloud/tunnel/daemon.go`)
    7. Configure split DNS for remote services
    8. Save machine state to `~/.cilo/machines/<host>/state.json` (host, token, WG config, connected_at)
    9. Print success: connected machine + environment count
  - Implement `cilo disconnect [host]` in same file:
    1. Stop WireGuard tunnel for that machine
    2. Remove DNS entries
    3. Remove machine state file
    4. If no host specified, disconnect all
  - Implement machine state storage in `internal/cli/machines.go`:
    - `ListConnectedMachines() []Machine`
    - `GetMachine(host string) *Machine`
    - `SaveMachine(machine Machine) error`
    - `RemoveMachine(host string) error`
  - REFACTOR: Reuse tunnel daemon code from `internal/cloud/tunnel/`

  **Must NOT do**:
  - Do NOT implement auto-reconnect on failure
  - Do NOT add connection health monitoring (future feature)
  - Do NOT support multiple simultaneous tunnels to same machine
  - Do NOT modify the WireGuard tunnel daemon itself

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex networking setup (WireGuard + DNS + state) requiring careful integration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7)
  - **Blocks**: Tasks 7, 8, 9, 10, 11
  - **Blocked By**: Tasks 3, 4

  **References**:

  **Pattern References**:
  - `internal/cli/cloud_up.go:294-396` (DELETED but read before) — WireGuard setup flow. Same sequence for connect, minus env creation.
  - `internal/cli/tunnel.go:223-264` — StartTunnelDaemon helper. Reuse directly.
  - `internal/cloud/tunnel/daemon.go:20-80` — DaemonConfig and DaemonState types. Use for machine connection state.

  **API/Type References**:
  - `internal/cilod/client.go` (from Task 4) — Client methods for Connect and WireGuardExchange
  - `internal/cloud/tunnel/types.go` — DaemonConfig, DaemonState types

  **Acceptance Criteria**:

  - [ ] `cilo connect <host>` establishes WireGuard tunnel and saves machine state
  - [ ] `cilo disconnect <host>` tears down tunnel and removes state
  - [ ] Machine state persisted in `~/.cilo/machines/<host>/`
  - [ ] WireGuard keys persisted (reconnecting reuses same keys)
  - [ ] `go test ./internal/cli/ -run TestConnect -v` → all PASS

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Connect creates machine state directory
    Tool: Bash
    Preconditions: cilod running on localhost
    Steps:
      1. cilo connect localhost
      2. Assert: exit code 0
      3. ls ~/.cilo/machines/localhost/
      4. Assert: state.json exists
      5. Assert: wg-key exists
      6. cilo machines
      7. Assert: output contains "localhost" and "connected"
    Expected Result: Machine registered and visible
    Evidence: State directory and machines output captured

  Scenario: Disconnect cleans up
    Tool: Bash
    Preconditions: Connected to localhost
    Steps:
      1. cilo disconnect localhost
      2. Assert: exit code 0
      3. ls ~/.cilo/machines/localhost/ 2>&1
      4. Assert: directory does not exist (or state.json removed)
      5. cilo machines
      6. Assert: output does NOT contain "localhost"
    Expected Result: Machine deregistered and cleaned up
    Evidence: Command output captured

  Scenario: Connect to unreachable host fails gracefully
    Tool: Bash
    Steps:
      1. cilo connect nonexistent.invalid 2>&1
      2. Assert: exit code != 0
      3. Assert: output contains error message about host unreachable
    Expected Result: Clear error, no partial state
    Evidence: Error output captured
  ```

  **Commit**: YES
  - Message: `feat(cli): implement cilo connect/disconnect for machine-level WireGuard tunnels`
  - Files: `internal/cli/connect.go`, `internal/cli/machines.go`, `internal/cli/connect_test.go`
  - Pre-commit: `go test ./internal/cli/ -run TestConnect -v`

---

- [ ] 6. Implement `cilo machines` Command

  **What to do**:
  - RED: Write test `TestMachinesCommand` — asserts output format with 0, 1, and N connected machines
  - GREEN: Implement `cilo machines` in `internal/cli/machines_cmd.go`:
    - List all connected machines from `~/.cilo/machines/*/state.json`
    - For each, show: hostname, WG IP, connected since, environment count (fetched from cilod)
    - Table format:
      ```
      MACHINE                    STATUS      ENVS   CONNECTED SINCE
      big-box.example.com        connected   3      2h ago
      gpu-server.internal        connected   1      5d ago
      ```
    - Support `--json` flag for machine-readable output
  - Register command in `internal/cli/root.go`
  - REFACTOR: Error handling for machines where cilod is unreachable

  **Must NOT do**:
  - Do NOT add machine health monitoring
  - Do NOT query cilod for detailed status (just env count)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple list command with table formatting
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 5, 7)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 3, 4

  **References**:

  **Pattern References**:
  - `internal/cli/commands.go:17-60` — listCmd implementation. Follow same table formatting pattern.
  - `internal/cli/machines.go` (from Task 5) — Machine state reading functions

  **Acceptance Criteria**:

  - [ ] `cilo machines` shows table of connected machines
  - [ ] `cilo machines --json` outputs JSON array
  - [ ] With 0 machines: prints "No connected machines" or equivalent
  - [ ] `go test ./internal/cli/ -run TestMachines -v` → PASS

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: No machines connected
    Tool: Bash
    Steps:
      1. cilo machines
      2. Assert: output contains "no connected machines" or similar
      3. Assert: exit code 0
    Expected Result: Clean empty state
    Evidence: Output captured
  ```

  **Commit**: YES (groups with Task 5)
  - Message: `feat(cli): add cilo machines command`
  - Files: `internal/cli/machines_cmd.go`
  - Pre-commit: `go build ./cmd/cilo`

---

- [ ] 7. Add `--on` Flag Routing to Lifecycle Commands

  **What to do**:
  - RED: Write tests:
    - `TestUpWithOnFlag` — `cilo up env --on machine` routes to cilod client
    - `TestDownWithOnFlag` — `cilo down env --on machine` routes to cilod
    - `TestDestroyWithOnFlag` — same
    - `TestRunWithOnFlag` — `cilo run cmd env --on machine` routes to cilod
    - `TestOnFlagNotConnected` — `--on unknown` returns "not connected" error
    - `TestCommandsWithoutOnFlag` — existing local behavior unchanged
  - GREEN: Implement `--on` routing layer:
    - Create `internal/cli/routing.go` with helper:
      ```go
      func resolveTarget(cmd *cobra.Command) (target Target, err error)
      // Returns LocalTarget{} or RemoteTarget{machine, client}
      // If --on specified but not connected → error
      ```
    - Add `--on` persistent flag to root command (available on all subcommands)
    - Modify `upCmd.RunE` in `internal/cli/lifecycle.go`:
      - If local target: existing code path (unchanged)
      - If remote target: call `cilodClient.UpEnvironment(name, opts)`
    - Same pattern for `downCmd`, `destroyCmd`
    - Modify `runCmd.RunE` in `internal/cli/run.go`:
      - If remote: sync workspace first, then call cilod `up` + `exec`
    - Add `--on` to `logsCmd`, `execCmd` in `internal/cli/commands.go`
  - REFACTOR: Ensure error messages are clear and actionable

  **Must NOT do**:
  - Do NOT modify any behavior when `--on` is NOT specified (regression prevention)
  - Do NOT add `--on` to commands where it doesn't make sense (init, setup, config, doctor)
  - Do NOT implement workspace sync here (that's Task 8)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Touches many files but with a consistent pattern. Needs care to avoid regressions.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 5, 6)
  - **Blocks**: Tasks 8, 11
  - **Blocked By**: Tasks 3, 4, 5

  **References**:

  **Pattern References**:
  - `internal/cli/lifecycle.go:169-361` — Current upCmd/downCmd/destroyCmd implementations. Add --on branch at top of each RunE.
  - `internal/cli/run.go:36-173` — Current runCmd. Add --on branch.
  - `internal/cli/commands.go:60-160` — logsCmd, execCmd. Add --on branch.

  **API/Type References**:
  - `internal/cilod/client.go` (from Task 4) — Client methods for remote operations
  - `internal/cli/machines.go` (from Task 5) — GetMachine() to resolve --on target

  **Acceptance Criteria**:

  - [ ] `--on` flag available on: run, up, down, destroy, exec, logs
  - [ ] `cilo up env --on machine` calls cilod API (test with mock)
  - [ ] `cilo up env` (no --on) behaves exactly as before
  - [ ] `cilo up env --on unknown` returns "not connected" error with suggestion to run `cilo connect`
  - [ ] `go test ./internal/cli/ -run TestOn -v` → all PASS
  - [ ] `go test ./internal/cli/ -run TestCommandsWithoutOnFlag -v` → all PASS (regression)

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: --on flag with connected machine
    Tool: Bash
    Preconditions: Connected to a cilod instance
    Steps:
      1. cilo up test-env --on localhost
      2. Assert: exit code 0 or expected error (env creation on remote)
      3. cilo ls
      4. Assert: test-env appears under localhost section
    Expected Result: Remote environment created
    Evidence: Output captured

  Scenario: --on flag without connection
    Tool: Bash
    Steps:
      1. cilo up test-env --on not-connected 2>&1
      2. Assert: exit code != 0
      3. Assert: output contains "not connected" and "cilo connect"
    Expected Result: Clear error with guidance
    Evidence: Error output captured

  Scenario: Local commands unchanged
    Tool: Bash
    Steps:
      1. cilo up --help
      2. Assert: output contains "--on"
      3. Assert: output still contains all existing flags (--build, --recreate, etc.)
    Expected Result: New flag added, existing flags unchanged
    Evidence: Help output captured
  ```

  **Commit**: YES
  - Message: `feat(cli): add --on flag for remote environment targeting`
  - Files: `internal/cli/routing.go`, `internal/cli/lifecycle.go`, `internal/cli/run.go`, `internal/cli/commands.go`, `internal/cli/root.go`
  - Pre-commit: `go test ./internal/cli/ -v`

---

- [ ] 8. Implement Workspace Sync for `--on` Operations

  **What to do**:
  - RED: Write tests in `internal/sync/sync_test.go`:
    - `TestRsyncSync` — syncs directory, verifies files arrive
    - `TestSyncExcludes` — .git, node_modules excluded
    - `TestSyncIncremental` — only changed files transferred
  - GREEN: Create `internal/sync/sync.go`:
    - `SyncWorkspace(localPath, remoteHost, remotePath string, opts SyncOptions) error`
    - Uses rsync over SSH (through WireGuard tunnel)
    - Default excludes: `.git/`, `node_modules/`, `.cilo/`, `__pycache__/`, `.venv/`
    - Falls back to tar+ssh if rsync unavailable
    - Reads `.ciloignore` file if present (like .gitignore format)
  - Wire into `--on` flow: before `cilo run --on` or `cilo up --on`, sync workspace to remote
  - Sync uses the WireGuard tunnel (SSH over the WG IP, not the public IP)

  **Must NOT do**:
  - Do NOT implement file watching or bidirectional sync
  - Do NOT implement reverse sync (remote → local)
  - Do NOT add a sync daemon or background process

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: File sync with rsync, exclude patterns, fallback logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 9, 10, 11)
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 4, 5, 7

  **References**:

  **Pattern References**:
  - `internal/cloud/sync.go` (DELETED in Task 1 — read before) — Original SyncWorkspace implementation. Use as starting point but decouple from cloud client.

  **Acceptance Criteria**:

  - [ ] `internal/sync/sync.go` exists with SyncWorkspace function
  - [ ] Excludes .git, node_modules by default
  - [ ] Reads `.ciloignore` if present
  - [ ] Falls back to tar+ssh if rsync unavailable
  - [ ] `go test ./internal/sync/ -v` → all PASS

  **Commit**: YES
  - Message: `feat(sync): implement workspace sync for remote operations`
  - Files: `internal/sync/sync.go`, `internal/sync/sync_test.go`
  - Pre-commit: `go test ./internal/sync/ -v`

---

- [ ] 9. Unify `cilo ls` to Show Local + Remote Environments

  **What to do**:
  - RED: Write tests:
    - `TestListLocalOnly` — no connected machines, shows only local
    - `TestListWithRemote` — connected machine, shows both sections
    - `TestListAllFlag` — `--all` shows all projects
  - GREEN: Modify `internal/cli/commands.go` listCmd:
    - Query local state (existing behavior)
    - For each connected machine, query cilod `/environments`
    - Display unified table:
      ```
      ENVIRONMENT    PROJECT    STATUS    MACHINE         SERVICES
      feat-auth      myapp      running   local           api, redis, postgres
      feat-pay       myapp      running   big-box         api, redis
      test-env       myapp      stopped   gpu-server      api
      ```
    - Machine column shows "local" or the machine hostname
    - If cilod unreachable, show machine as "(unreachable)" with last known envs
  - REFACTOR: Handle case where same env name exists on different machines

  **Must NOT do**:
  - Do NOT add real-time status updates or polling
  - Do NOT cache remote env state

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Aggregation logic, table formatting, error handling for unreachable machines
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 8, 10, 11)
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 4, 5, 6

  **References**:

  **Pattern References**:
  - `internal/cli/commands.go:17-60` — Current listCmd. Extend, don't replace.
  - `internal/cli/machines.go` (from Task 5) — ListConnectedMachines() for iterating remote machines

  **Acceptance Criteria**:

  - [ ] `cilo ls` shows local environments (unchanged when no machines connected)
  - [ ] `cilo ls` shows remote environments with machine column when connected
  - [ ] Unreachable machines show "(unreachable)" not an error
  - [ ] `go test ./internal/cli/ -run TestList -v` → all PASS

  **Commit**: YES
  - Message: `feat(cli): unify cilo ls to show local and remote environments`
  - Files: `internal/cli/commands.go`
  - Pre-commit: `go test ./internal/cli/ -v`

---

- [ ] 10. Remote DNS Integration

  **What to do**:
  - RED: Write tests in `internal/dns/remote_test.go`:
    - `TestAddRemoteDNSEntries` — adds entries for remote services
    - `TestRemoveRemoteDNSEntries` — cleans up on disconnect
    - `TestRemoteDNSResolution` — dig query resolves through WG tunnel
  - GREEN: Extend DNS system:
    - When `cilo connect` succeeds and gets env list from cilod, add DNS entries for all remote services
    - DNS entries point to the WireGuard peer IP (cilod's proxy IP) — reuse existing proxy pattern
    - Format: `{service}.{env}.test → {wg-proxy-ip}` (same as local convention)
    - When `cilo disconnect`, remove all DNS entries for that machine
    - When remote env is created via `--on`, add DNS entries immediately
  - Wire into connect/disconnect flow and --on flow

  **Must NOT do**:
  - Do NOT change DNS convention (keep {service}.{env}.test)
  - Do NOT add DNS polling or refresh daemon
  - Do NOT modify local DNS behavior

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: DNS integration with existing dnsmasq system
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 8, 9, 11)
  - **Blocks**: Task 12
  - **Blocked By**: Task 5

  **References**:

  **Pattern References**:
  - `internal/dns/dns.go:1-80` — SetupDNS, UpdateDNS, RemoveDNS. Extend for remote entries.
  - `internal/dns/render.go:1-60` — DNS config rendering from state. Add remote section.
  - `internal/cli/cloud_up.go:434-507` (DELETED — read before) — configureCloudDNS function. Similar logic needed for connect.

  **Acceptance Criteria**:

  - [ ] After `cilo connect`, remote service DNS entries are resolvable
  - [ ] After `cilo disconnect`, remote DNS entries are removed
  - [ ] `dig api.remote-env.test @127.0.0.1 -p 5354` resolves to WG proxy IP
  - [ ] Local DNS entries unaffected by remote operations
  - [ ] `go test ./internal/dns/ -v` → all PASS

  **Commit**: YES
  - Message: `feat(dns): extend DNS system for remote environment resolution`
  - Files: `internal/dns/remote.go`, `internal/dns/remote_test.go`, `internal/dns/dns.go`
  - Pre-commit: `go test ./internal/dns/ -v`

---

- [ ] 11. Remote Exec + Logs Streaming

  **What to do**:
  - RED: Write tests:
    - `TestRemoteExec` — exec command sent to cilod, output received
    - `TestRemoteLogs` — log stream from cilod service
    - `TestRemoteLogsFollow` — streaming with --follow
  - GREEN:
    - Implement WebSocket client in cilod client for log streaming
    - Implement WebSocket client for exec (bidirectional: stdin/stdout/stderr)
    - Wire into `logsCmd` and `execCmd` when `--on` is specified
    - For logs: `GET /environments/:name/logs?service=X&follow=true` → WebSocket upgrade
    - For exec: `POST /environments/:name/exec` with WebSocket upgrade → bidirectional TTY
    - Handle Ctrl+C signal propagation to remote
  - REFACTOR: Ensure PTY allocation works for interactive exec

  **Must NOT do**:
  - Do NOT implement session persistence/reattach
  - Do NOT add tmux-like features

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: WebSocket streaming, PTY forwarding, signal handling
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 8, 9, 10)
  - **Blocks**: Task 12
  - **Blocked By**: Tasks 4, 7

  **References**:

  **Pattern References**:
  - `internal/cli/commands.go:100-160` — Current logsCmd and execCmd. Add --on branch.
  - `internal/agent/handlers.go` — Agent-side handlers. Add WebSocket upgrade support.

  **External References**:
  - `github.com/gorilla/websocket` — WebSocket library for Go

  **Acceptance Criteria**:

  - [ ] `cilo logs env --on machine` streams logs from remote
  - [ ] `cilo logs env --on machine -f` follows log output via WebSocket
  - [ ] `cilo exec env service --on machine -- bash` opens interactive shell
  - [ ] Ctrl+C in exec terminates remote process
  - [ ] `go test ./internal/cli/ -run TestRemote -v` → all PASS

  **Commit**: YES
  - Message: `feat(cli): implement remote exec and log streaming via WebSocket`
  - Files: `internal/cilod/stream.go`, `internal/cli/commands.go`
  - Pre-commit: `go test ./internal/cli/ -v`

---

- [ ] 12. Integration Tests + Regression Suite

  **What to do**:
  - Write comprehensive integration tests in `test/e2e/`:
    - `test/e2e/regression_test.go` — All local commands work unchanged:
      - `cilo create`, `cilo up`, `cilo down`, `cilo destroy`, `cilo run`, `cilo list`, `cilo logs`, `cilo exec`
      - No `--on` flag needed, behavior identical to before redesign
    - `test/e2e/connect_test.go` — Connect/disconnect flow:
      - Connect to cilod, verify machine appears
      - Disconnect, verify cleanup
      - Connect to multiple machines
    - `test/e2e/remote_workflow_test.go` — Full remote workflow:
      - Connect → run with --on → ls shows remote env → logs → exec → down → disconnect
    - `test/e2e/remote_errors_test.go` — Error cases:
      - --on without connect
      - Connect to unreachable host
      - Disconnect while envs running (should succeed with warning)
  - Verify: `go test ./test/e2e/ -v` passes end-to-end
  - Verify: `go test ./... -v` passes full suite

  **Must NOT do**:
  - Do NOT test deleted cloud commands (they no longer exist)
  - Do NOT test racing/fan-out (out of scope)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Integration tests requiring careful environment setup and teardown
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (final)
  - **Blocks**: None (final task)
  - **Blocked By**: Tasks 8, 9, 10, 11

  **References**:

  **Pattern References**:
  - `test/e2e/workflow_test.go` — Existing e2e test pattern. Follow same structure.
  - `test/e2e/shared_services_test.go` — Another e2e test example.

  **Acceptance Criteria**:

  - [ ] `go test ./test/e2e/ -v` → all PASS
  - [ ] `go test ./... -v` → all PASS (full suite)
  - [ ] Regression tests confirm local commands unchanged
  - [ ] Remote workflow test demonstrates full connect → run → ls → disconnect flow

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Full test suite passes
    Tool: Bash
    Steps:
      1. go test ./... -v -count=1 2>&1
      2. Assert: exit code 0
      3. Assert: no "FAIL" lines in output
      4. Count "PASS" lines
      5. Assert: PASS count > 30 (existing + new tests)
    Expected Result: All tests green
    Evidence: Full test output captured
  ```

  **Commit**: YES
  - Message: `test: add integration and regression tests for unified CLI`
  - Files: `test/e2e/regression_test.go`, `test/e2e/connect_test.go`, `test/e2e/remote_workflow_test.go`, `test/e2e/remote_errors_test.go`
  - Pre-commit: `go test ./... -v`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `refactor(cli): remove cloud command namespace and central server code` | ~20 deleted files | `go build ./...` |
| 2 | `feat(agent): design cilod API specification and auth protocol` | api.go, auth.go, auth_test.go | `go build ./internal/agent/...` |
| 3 | `feat(agent): implement cilod full API surface with SSH auth` | server.go, handlers.go, auth.go | `go test ./internal/agent/` |
| 4 | `feat(cilod): add client library for CLI-to-cilod communication` | client.go, types.go, client_test.go | `go test ./internal/cilod/` |
| 5 | `feat(cli): implement cilo connect/disconnect` | connect.go, machines.go | `go test ./internal/cli/ -run TestConnect` |
| 6 | `feat(cli): add cilo machines command` | machines_cmd.go | `go build ./cmd/cilo` |
| 7 | `feat(cli): add --on flag for remote targeting` | routing.go, lifecycle.go, run.go, commands.go | `go test ./internal/cli/` |
| 8 | `feat(sync): implement workspace sync` | sync.go, sync_test.go | `go test ./internal/sync/` |
| 9 | `feat(cli): unify cilo ls` | commands.go | `go test ./internal/cli/` |
| 10 | `feat(dns): extend DNS for remote resolution` | remote.go, dns.go | `go test ./internal/dns/` |
| 11 | `feat(cli): remote exec and log streaming` | stream.go, commands.go | `go test ./internal/cli/` |
| 12 | `test: integration and regression tests` | test/e2e/*.go | `go test ./...` |

---

## Success Criteria

### Verification Commands
```bash
# Build succeeds
go build ./...                    # Expected: exit 0, no errors

# All tests pass
go test ./... -v                  # Expected: all PASS, 0 FAIL

# Cloud commands gone
cilo cloud 2>&1                   # Expected: "unknown command"

# Local commands work
cilo list                         # Expected: shows local environments

# Connect works
cilo connect <host>               # Expected: WireGuard tunnel established
cilo machines                     # Expected: shows connected machine

# Remote targeting works
cilo up test --on <host>          # Expected: creates env on remote
cilo ls                           # Expected: shows local + remote envs

# DNS resolves
dig api.test-env.test @127.0.0.1 -p 5354  # Expected: resolves to WG IP

# Clean disconnect
cilo disconnect <host>            # Expected: tunnel torn down, DNS cleaned
```

### Final Checklist
- [ ] No `cilo cloud` subcommand exists
- [ ] `--on` flag works on: run, up, down, destroy, exec, logs
- [ ] `cilo connect/disconnect/machines` commands work
- [ ] `cilo ls` shows unified local + remote view
- [ ] DNS resolves remote services
- [ ] Workspace sync works for --on operations
- [ ] All local commands work unchanged (regression)
- [ ] TDD: all tests pass
- [ ] `go vet ./...` clean
