# Cilo Development Roadmap

**Version:** 2.0  
**Status:** Phase 1 Complete ✅ (v0.1.20)

---

## Quick Start

1. **Read the epic:** [`docs/ARCHITECTURE_EPIC.md`](./docs/ARCHITECTURE_EPIC.md)
2. **Understand the design:** [`docs/architecture/ARCHITECTURE_DESIGN.md`](./docs/architecture/ARCHITECTURE_DESIGN.md)
3. **Implement foundations first:** [`docs/phases/PHASE_1_FOUNDATIONS.md`](./docs/phases/PHASE_1_FOUNDATIONS.md)

---

## Overview

Make Cilo reliable locally, while keeping the long-term direction visible.

### What We're Building

- **Foundations hardening (now):** correctness, determinism, tests
- **Possible later directions (vision):** multi-runtime, remote, shared resources, multi-user/API

---

## Phases

| Phase | Duration | Status | Notes |
|-------|----------|--------|-------|
| **Phase 0: Runtime Abstraction** | 0.5 day | ✅ Complete | Provider interface for Docker operations |
| **Phase 1: Foundations** | 2-3 days | ✅ Complete | v0.1.20 - File locking, atomic writes, DNS render, collision detection |
| **Phase 1B: cilo run** | 0.5 day | ✅ Complete | Agent-first workflow command |
| **Phase 2A: Shared Resources** | 1–3 days | 📝 Optional | Only if demanded |
| **Phase 2B: Remote Operation** | TBD | 📝 Draft | Requires routing-model decision gate |
| **Phase 3: Production Hardening** | TBD | 📝 Draft | Separate product shift (auth/tenancy/API) |

---

## Phase Details

### Phase 1: Foundations (2-3 days)
**Goal:** Fix all critical reliability issues

**Deliverables:**
- Concurrent-safe state operations (file locking, atomic writes)
- Compose override model (stop rewriting user files)
- Rendered DNS configs (no text editing)
- Reconciliation system
- Integration test suite
- Collision detection

**Success:** 100 concurrent creates without corruption

**[→ Implementation Guide](./docs/phases/PHASE_1_FOUNDATIONS.md)**

---

### Phase 2A: Shared Resources (1-2 days)
**Goal:** Enable resource sharing between environments

**Deliverables:**
- External network support (`--shared-network`)
- Dependency tracking in state
- Lifecycle protection (prevent destroying in-use resources)
- DNS for shared services

**Use Case:** 10 frontend envs sharing 1 database

**[→ Implementation Guide](./docs/phases/PHASE_2A_SHARED_RESOURCES.md)**

---

### Phase 2B: Remote Operation (3-4 days)
**Goal:** Run environments on remote hosts (draft)

**Deliverables:**
- Remote host management
- Mesh provider abstraction (Tailscale, WireGuard, SSH)
- Docker context support
- Workspace sync (rsync/git)
- DNS resolution to remote IPs

**Use Case:** Create environment on remote server via Tailscale

**[→ Draft](./docs/phases/PHASE_2B_REMOTE_OPERATION.md)**

---

### Phase 3: Production Hardening (2-3 days)
**Goal:** Multi-user, production-ready (draft)

**Deliverables:**
- Authentication layer (API keys, tokens)
- Per-user quotas (disk, CPU, memory)
- Audit logging
- gRPC/HTTP API
- Garbage collection
- Resource limits

**Use Case:** Multi-user shared server with quotas

**[→ Draft](./docs/phases/PHASE_3_PRODUCTION_HARDENING.md)**

---

## Technical Architecture

### Before (Current)
```
CLI → Docker (hardcoded) → State (JSON, no locking) → DNS (text editing)
```

### After (Post-Epic)
```
┌─────── CLI ───────┐
        ↓
┌──── Core Layer ───┐
│ - State (locked)  │
│ - DNS (rendered)  │
│ - Workspace       │
└───────┬───────────┘
        ↓
┌─── Providers ─────┐
│ - Docker          │
│ - Podman          │
│ - Kubernetes*     │
└───────┬───────────┘
        ↓
┌── Mesh Layer ─────┐
│ - Tailscale       │
│ - WireGuard       │
│ - SSH             │
└───────────────────┘
```

*Kubernetes: Future possibility

---

## Success Metrics

### Phase 1 Complete When:
- [ ] 100 concurrent `cilo create` without state corruption
- [ ] DNS updates are atomic and crash-safe
- [ ] Works with complex compose (extends, profiles, multiple files)
- [ ] Integration tests cover all workflows

### Phase 2A Complete When:
- [ ] 10 frontend envs share 1 database env
- [ ] Destroying shared resource blocked when in use
- [ ] Shared services visible via DNS

### Phase 2B Complete When:
- [ ] Create env on remote host via Tailscale
- [ ] Access remote services via DNS (`service.env.remote.test`)
- [ ] Workspace syncs to remote (rsync or git)

### Phase 3 Complete When:
- [ ] Multi-user access with auth
- [ ] Per-user disk/memory quotas enforced
- [ ] gRPC API for programmatic use
- [ ] Auto-cleanup of old environments

---

## Documentation

### Documentation

- 📁 **docs/**
  - [`ARCHITECTURE_EPIC.md`](./docs/ARCHITECTURE_EPIC.md) - Refined plan + invariants
  - [`README.md`](./docs/README.md) - Documentation index
  - [`EPIC_SUMMARY.md`](./docs/EPIC_SUMMARY.md) - Short summary
- 📁 **docs/architecture/**
  - [`ARCHITECTURE_DESIGN.md`](./docs/architecture/ARCHITECTURE_DESIGN.md) - System design
- 📁 **docs/phases/**
  - [`PHASE_1_FOUNDATIONS.md`](./docs/phases/PHASE_1_FOUNDATIONS.md)
  - [`PHASE_2A_SHARED_RESOURCES.md`](./docs/phases/PHASE_2A_SHARED_RESOURCES.md)
  - Drafts: `PHASE_2B_REMOTE_OPERATION.md`, `PHASE_3_PRODUCTION_HARDENING.md`
- 📁 **docs/specs/**
  - Draft specs (added as needed)

---

## Next Steps

### Immediate Actions

1. **Review**
   - [Architecture Epic](./docs/ARCHITECTURE_EPIC.md)
   - [Architecture Design](./docs/architecture/ARCHITECTURE_DESIGN.md)
2. **Start Phase 1**
   - Follow [Phase 1 guide](./docs/phases/PHASE_1_FOUNDATIONS.md)

---

## Questions to Resolve

### Before Starting
- [ ] Confirm target Go version (1.21.6 or newer?)
- [ ] Confirm test environment setup (Docker in CI?)
- [ ] Confirm backwards compatibility requirements

### During Implementation
- [ ] Override file naming preference
- [ ] State migration: automatic or manual command?
- [ ] Reconciliation: on every command or just doctor/status?

See [Architecture Epic - Open Questions](./docs/ARCHITECTURE_EPIC.md#open-questions-to-be-resolved-per-phase)

---

## Risk Management

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Migration breaks existing setups | Medium | High | Automatic backup, rollback docs |
| DNS on macOS quirks | High | Medium | Extensive testing, fallback options |
| State corruption under concurrency | Low | Critical | File locking + atomic writes (Phase 1) |
| Tailscale API changes | Low | Medium | Abstract behind mesh provider interface |

---

## Communication Plan

### Weekly Updates
- Progress against phase milestones
- Blockers and dependencies
- Updated timeline if needed

### Phase Completion Reviews
- Demo of new capabilities
- Review success criteria
- Plan next phase

---

## References

- Original oracle-analysis doc: not present in this repo snapshot (link removed to avoid dead reference)
- [Docker Compose Spec](https://github.com/compose-spec/compose-spec)
- [Tailscale API Docs](https://tailscale.com/kb/1101/api/)
- [Coder.com Architecture](https://coder.com/docs/v2/latest/about)

---

## Contributors

- **Architecture:** Oracle (AI architectural review)
- **Planning:** Comprehensive phase-by-phase breakdown
- **Implementation:** *Your team here*

---

**Last Updated:** 2026-02-05  
**Status:** ✅ Phase 0-1 Complete, Stable Foundation Ready
