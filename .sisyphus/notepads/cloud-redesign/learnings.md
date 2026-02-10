
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
