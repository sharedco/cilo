
## Task 2: Design cilod Auth Protocol + API Specification

### Completed: 2026-02-10

### Files Created:
- `internal/agent/auth.go` - SSH authentication types and interfaces
- `internal/agent/auth_test.go` - TestSSHKeyAuth (RED state)
- `internal/agent/api.go` - API specification with all endpoint types

### Auth Protocol Design:
- SSH challenge-response authentication
- Client sends: public_key, challenge, signature
- Server returns: bearer token with 24h expiry
- Simple file-based session storage (no external DB)

### API Endpoints Defined:
```
POST   /auth/connect          — SSH key exchange, returns session token
DELETE /auth/disconnect        — Invalidate session
GET    /environments           — List all environments
POST   /environments/:name/up  — Create + start environment
POST   /environments/:name/down — Stop environment
DELETE /environments/:name     — Destroy environment
GET    /environments/:name/status — Get env status
GET    /environments/:name/logs  — Stream logs (WebSocket upgrade)
POST   /environments/:name/exec — Exec into container (WebSocket upgrade)
POST   /wireguard/exchange     — WireGuard key exchange
DELETE /wireguard/peers/:key   — Remove peer
GET    /wireguard/status       — WireGuard interface status
POST   /sync/:name             — Receive workspace sync
```

### IP Allocation Strategy:
- Each cilod manages its own /24 subnet independently
- Default: 10.225.0.0/24 (configurable)
- Storage: `/var/cilo/peers.json` - simple JSON mapping
- Format: `{ "peers": { "pubkey": "ip" }, "next_ip": "..." }`
- No central coordinator needed - each cilod operates independently

### TDD Status: RED
- Tests written and failing as expected
- Build succeeds
- Implementation stubs in place
- Ready for GREEN phase (actual implementation)

### Key Types Defined:
- `SSHAuthVerifier` interface for signature verification
- `AuthHandler` with middleware for protected routes
- `Session` with token expiry tracking
- `APIHandler` interface with 16 handler methods
- All request/response types for endpoints
- `IPAllocator` interface for peer IP management

## Task 3: Implement cilod Full API Surface (GREEN Phase)

### Completed Implementation

#### SSH Authentication (auth.go)
- Implemented `DefaultSSHVerifier.Verify()` using golang.org/x/crypto/ssh
- Uses proper SSH signature format with algorithm detection
- Signs raw challenge bytes (not pre-hashed)
- Supports RSA keys with SHA1 (ssh-rsa algorithm)
- Token generation with 24-hour expiry
- In-memory session storage with map[string]*Session

#### Auth Middleware
- `AuthMiddleware` validates Bearer tokens from Authorization header
- Returns 401 for missing/invalid tokens
- Returns 401 for expired tokens
- Stores session in context for handlers

#### Environment CRUD (handlers.go)
- `GET /environments` - List all environments using EnvironmentManager.List()
- `POST /environments/:name/up` - Create and start environment
- `POST /environments/:name/down` - Stop environment  
- `DELETE /environments/:name` - Destroy environment
- `GET /environments/:name/status` - Get detailed status with services
- `GET /environments/:name/logs` - Get logs (WebSocket stub)
- `POST /environments/:name/exec` - Exec into container (WebSocket stub)

#### WireGuard Exchange (wireguard_exchange.go)
- Created `JSONPeerStore` for IP allocation persistence
- Stores allocations in `/var/cilo/peers.json`
- IP allocation: 10.225.0.2, 10.225.0.3, etc. (increments last octet)
- Atomic file writes (temp file + rename)
- `POST /wireguard/exchange` - Returns peer config with assigned IP
- `DELETE /wireguard/peers/:key` - Remove peer from WG and store
- `GET /wireguard/status` - Returns WG interface status with peer list

#### Server Wiring (server.go)
- Added authHandler and peerStore to Server struct
- Protected routes use AuthMiddleware
- Legacy routes preserved for backward compatibility
- All API endpoints wired into chi router

#### Error Responses
All errors return consistent JSON: `{"error": "message"}`

### Test Results
- All auth tests pass (TestSSHKeyAuth)
- SSH signature verification works correctly
- Invalid signatures rejected with 403
- Missing auth rejected with 401
- Expired tokens rejected with 401

### Files Modified
- internal/agent/auth.go - SSH verification implementation
- internal/agent/auth_test.go - Updated stub verifier
- internal/agent/handlers.go - Full API handlers
- internal/agent/server.go - Route wiring
- internal/agent/environment.go - Added List() method
- internal/agent/wireguard_exchange.go - New file for peer store

### Build Verification
- go build ./internal/agent/... - Success
- go build ./cmd/cilo-agent - Success
- go test ./internal/agent/ - All tests pass

## Task 7: Add --on Flag Routing to Lifecycle Commands

### Completed: 2026-02-10

### Files Created:
- `internal/cli/routing.go` - Routing layer with Target interface
- `internal/cli/routing_test.go` - Comprehensive test suite

### Files Modified:
- `internal/cli/root.go` - Added --on persistent flag
- `internal/cli/lifecycle.go` - up/down/destroy routing
- `internal/cli/run.go` - run command routing
- `internal/cli/commands.go` - logs/exec routing
- `internal/config/paths.go` - Added GetMachinesDir()

### Routing Layer Design:

#### Target Interface
```go
type Target interface {
    IsRemote() bool
    GetMachine() string
    GetClient() *cilod.Client
}
```

#### LocalTarget
- Returns false for IsRemote()
- Used when --on flag is not specified
- Preserves all existing local behavior (regression prevention)

#### RemoteTarget
- Returns true for IsRemote()
- Contains machine name and cilod client
- Routes commands to remote cilod via HTTP API

#### resolveTarget() Function
- Checks --on flag from command
- If empty: returns LocalTarget (local execution)
- If specified: looks up machine in ~/.cilo/machines/
- If machine not found: returns clear error with "cilo connect" suggestion
- If machine found: creates cilod client and returns RemoteTarget

### Command Modifications:

#### upCmd
- Added routing check at start of RunE
- Calls upRemote() for remote targets
- upRemote() uses cilod.UpEnvironment() API

#### downCmd
- Added routing check at start of RunE
- Calls downRemote() for remote targets
- downRemote() uses cilod.DownEnvironment() API

#### destroyCmd
- Added routing check at start of RunE
- Calls destroyRemote() for remote targets
- destroyRemote() uses cilod.DestroyEnvironment() API
- Preserves --force flag behavior for remote

#### runCmd
- Added routing check at start of runRun()
- Calls runRemote() for remote targets
- runRemote() handles create/up/exec sequence
- Stubs for workspace sync (Task 8)

#### logsCmd
- Added routing check at start of RunE
- Calls logsRemote() for remote targets
- logsRemote() uses cilod.StreamLogs() API
- Preserves --follow flag

#### execCmd
- Added routing check at start of RunE
- Calls execRemote() for remote targets
- execRemote() uses cilod.Exec() API
- Stubs for WebSocket TTY (Task 11)

### Error Handling:

#### Machine Not Connected
```
Error: machine 'unknown-machine' is not connected. Run 'cilo connect unknown-machine' first
```

#### Clear Actionable Guidance
- Error message explicitly mentions "not connected"
- Suggests running "cilo connect <machine>"
- Exit code 1 for proper error handling

### Test Coverage:

#### Routing Tests (routing_test.go)
- TestUpWithOnFlag - up command routes to cilod
- TestDownWithOnFlag - down command routes to cilod
- TestDestroyWithOnFlag - destroy command routes to cilod
- TestRunWithOnFlag - run command routes to cilod
- TestLogsWithOnFlag - logs command routes to cilod
- TestExecWithOnFlag - exec command routes to cilod
- TestOnFlagNotConnected - error for unknown machine
- TestCommandsWithoutOnFlag - regression test for local behavior
- TestLocalTarget - LocalTarget interface methods
- TestRemoteTarget - RemoteTarget interface methods
- TestGetMachine - machine state lookup
- TestListConnectedMachines - list all connected machines

#### Regression Prevention
- All tests verify local behavior unchanged when --on not specified
- No modifications to existing local code paths
- LocalTarget returns nil client (existing code unaffected)

### Build Verification:
- go build ./cmd/cilo - Success
- go test ./internal/cli/ -run TestOn - All PASS
- go test ./internal/cli/ -run TestCommandsWithoutOnFlag - All PASS

### Flag Availability:
All lifecycle commands now have --on flag in Global Flags:
- cilo up <name> --on <machine>
- cilo down <name> --on <machine>
- cilo destroy <name> --on <machine>
- cilo run <cmd> <env> --on <machine>
- cilo logs <name> --on <machine>
- cilo exec <name> <svc> --on <machine>

### Key Implementation Notes:

1. **Routing is Transparent**: Commands don't need to know if they're local or remote
2. **Error Messages are Clear**: Always suggest "cilo connect" when machine not found
3. **Regression Prevention**: Local behavior completely unchanged (no --on = local)
4. **Consistent Pattern**: All commands use same routing pattern
5. **Stub Implementations**: Remote exec/logs use stubs - full WebSocket in Task 11
6. **Workspace Sync**: Remote run stubs for sync - full rsync in Task 8
