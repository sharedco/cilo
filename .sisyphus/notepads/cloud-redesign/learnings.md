
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

## Task 8: Implement Workspace Sync for --on Operations

### Completed: 2026-02-10

### Files Created:
- `internal/sync/sync.go` - Workspace sync implementation
- `internal/sync/sync_test.go` - Comprehensive test suite

### Files Modified:
- `internal/cli/lifecycle.go` - Added sync before upRemote
- `internal/cli/run.go` - Added sync before runRemote

### Sync Package Design:

#### SyncOptions
```go
type SyncOptions struct {
    RemoteHost     string   // Target machine (WireGuard IP)
    RemotePath     string   // Destination path on remote
    UseRsync       bool     // Prefer rsync over tar+ssh
    ProgressWriter io.Writer // Optional progress output
    SSHKeyPath     string   // SSH key for authentication
    SSHPort        int      // SSH port (default 22)
}
```

#### SyncWorkspace Function
- Primary entry point for workspace synchronization
- Validates inputs (local path exists, remote host/path provided)
- Reads `.ciloignore` file if present (gitignore format)
- Combines default excludes with .ciloignore patterns
- Auto-detects rsync availability
- Falls back to tar+ssh if rsync unavailable

### Default Excludes:
```go
[]string{
    ".git/",           // Git repository
    "node_modules/",   // Node dependencies
    ".cilo/",          // Cilo metadata
    "__pycache__/",    // Python cache
    ".venv/",          // Python virtualenv
    ".env.local",      // Local env files
    "*.log",           // Log files
    ".DS_Store",       // macOS metadata
    "Thumbs.db",       // Windows metadata
    ".ciloignore",     // Ignore file itself
}
```

### Sync Methods:

#### Rsync (Preferred)
- Uses `rsync -avz --delete --checksum`
- SSH over WireGuard tunnel (uses WG assigned IP)
- Incremental sync (only changed files)
- Efficient delta transfer
- Supports exclude patterns natively

#### Tar+SSH (Fallback)
- Creates compressed tar archive locally
- Streams through SSH to remote
- Extracts on remote side
- Full sync every time (no incremental)
- Used when rsync not available

### .ciloignore Support:
- Gitignore-style pattern matching
- Comments (#) and empty lines ignored
- Supports negation patterns (!important.log)
- Directory patterns (build/)
- Wildcard patterns (*.log)

### WireGuard Integration:
- Uses machine's `WGAssignedIP` for SSH connectivity
- Falls back to machine hostname if WG IP unavailable
- SSH through established WireGuard tunnel
- No public IP exposure

### Test Coverage:

#### sync_test.go
- `TestRsyncSync` - Full directory sync verification
- `TestSyncExcludes` - Default excludes respected
- `TestSyncIncremental` - Only changed files transferred
- `TestSyncCiloignore` - .ciloignore patterns respected
- `TestSyncFallback` - tar+ssh fallback works
- `TestSyncOptions` - Options validation
- `TestParseCiloignore` - Ignore file parsing
- `TestDefaultExcludes` - Default patterns correct
- `TestIsRsyncAvailable` - Rsync detection
- `TestBuildRsyncArgs` - Rsync argument building
- `TestSyncWorkspaceValidation` - Input validation

### Command Integration:

#### cilo up --on <machine>
1. Resolve target machine
2. Get WireGuard IP from machine state
3. Sync workspace: `SyncWorkspace(cwd, wgIP, remoteWorkspace, opts)`
4. Call cilod.UpEnvironment with WorkspacePath

#### cilo run --on <machine>
1. Resolve target machine
2. Get WireGuard IP from machine state
3. If creating environment: sync workspace first
4. If starting environment: sync workspace first
5. Execute command via cilod.Exec

### Remote Workspace Path:
```
/var/cilo/envs/<project>/<environment>/
```

### Build Verification:
- `go build ./internal/sync/...` - Success
- `go test ./internal/sync/ -v` - All 14 tests PASS
- `go build ./cmd/cilo` - Success

### Key Implementation Notes:

1. **Efficient Sync**: Rsync with --checksum for accurate incremental sync
2. **Smart Excludes**: Combines defaults + .ciloignore for minimal transfer
3. **Graceful Fallback**: tar+ssh works when rsync unavailable
4. **WireGuard Native**: Uses WG tunnel IP, not public IP
5. **Progress Support**: Optional ProgressWriter for user feedback
6. **Validation**: Comprehensive input validation before sync
7. **Security**: SSH key auth through WireGuard tunnel

## Task 10: Remote DNS Integration

### Completed: 2026-02-10

### Files Created:
- `internal/dns/remote.go` - Remote DNS management functions
- `internal/dns/remote_test.go` - Comprehensive test suite

### Files Modified:
- `internal/dns/dns.go` - Changed getDNSDir to variable for testability
- `internal/cli/connect.go` - Wired AddRemoteMachine on connect, RemoveRemoteMachine on disconnect

### Remote DNS Design:

#### RemoteMachine Struct
```go
type RemoteMachine struct {
    Host         string
    WGAssignedIP string
}
```
Minimal struct to avoid import cycles with internal/cli.

#### DNS Entry Format
- Same convention as local: `{service}.{env}.test`
- Points to WireGuard proxy IP (cilod's WGAssignedIP)
- Example: `api.remote-env.test → 10.225.1.5`

#### Config File Markers
```
# Remote machine: remote.example.com
address=/api.env1.test/10.225.1.5
address=/db.env1.test/10.225.1.5
# End remote machine: remote.example.com
```

### Functions Implemented:

#### AddRemoteMachine(machine *RemoteMachine, envs []cilod.Environment)
- Adds DNS entries for all services on all environments
- Idempotent: removes existing entries first
- Appends to existing dnsmasq.conf
- Reloads dnsmasq gracefully (SIGHUP)

#### RemoveRemoteMachine(host string)
- Removes all DNS entries for a machine by host
- Uses marker-based section removal
- Preserves all local and other remote entries
- Reloads dnsmasq gracefully

#### UpdateRemoteDNSEntries(machine *RemoteMachine, envs []cilod.Environment)
- Convenience function: Remove + Add
- Used when environments change on remote machine

### Integration Points:

#### cilo connect
1. Authenticate with cilod
2. Exchange WireGuard keys
3. Save machine state
4. **NEW**: Fetch environments from cilod
5. **NEW**: Call AddRemoteMachine() with WG IP and envs
6. Print connection summary

#### cilo disconnect
1. **NEW**: Call RemoveRemoteMachine() to clean up DNS
2. Stop tunnel daemon
3. Remove machine state

### Test Coverage:

#### remote_test.go
- `TestAddRemoteDNSEntries` - Adds entries for remote services
- `TestRemoveRemoteDNSEntries` - Cleans up on disconnect
- `TestRemoteDNSResolution` - Verifies WG IP resolution
- `TestUpdateRemoteDNSEntries` - Updates when envs change
- `TestAddRemoteMachine_MultipleEnvironments` - Multiple envs support
- `TestLocalDNSEntriesUnaffected` - Local entries preserved

### Key Implementation Notes:

1. **Same DNS Convention**: Uses {service}.{env}.test format (identical to local)
2. **WireGuard IP**: All entries point to cilod's WG proxy IP
3. **Marker-Based**: Uses # Remote machine: headers for easy removal
4. **Idempotent**: Can safely call AddRemoteMachine multiple times
5. **Local Preservation**: Local DNS entries completely unaffected
6. **No Polling**: No refresh daemon - explicit add/remove only
7. **Import Cycle Avoidance**: RemoteMachine defined in dns package

### Build Verification:
- `go build ./internal/dns/...` - Success
- `go test ./internal/dns/ -v` - All 8 tests PASS
- `go build ./cmd/cilo` - Success
