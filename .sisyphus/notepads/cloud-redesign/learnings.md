
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
