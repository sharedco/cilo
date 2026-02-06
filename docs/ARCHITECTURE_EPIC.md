# Cilo Architecture Epic (Refined)

**Status:** Phase A (Foundations) ✅ Complete (v0.1.20)  
**Next:** Phase 2A (Shared Resources) or Phase 2B (Remote Operation) when ready  
**Primary goal (achieved):** Local Cilo is now reliable and non-fragile under real usage.

This epic keeps a long-term vision in view, but narrows near-term execution to the objective architectural work that the current codebase actually needs.

---

## 0) First-principles problem statement

**Problem:** Multiple concurrent copies of the same Docker Compose project are hard to run because host ports collide and “where is service X?” becomes ad-hoc.

**Cilo approach:** isolate each environment on its own network and provide deterministic DNS names (e.g. `service.env.test`) so users/agents don’t use host ports as the addressing mechanism.

---

## 1) Architecture invariants (the constitution)

These are non-negotiable. Design and implementation work must preserve them.

1. **Never mutate user compose.** Cilo may generate artifacts, but must not rewrite user-authored compose.
2. **Determinism:** Generated outputs (DNS config, compose override) are fully reproducible from state + runtime introspection.
3. **Crash safety:** State and DNS updates are atomic. After a crash, the system can recover to a consistent state.
4. **Concurrency safety:** Parallel CLI usage cannot corrupt state.
5. **Reconciliation:** Runtime is source of truth for running resources; state is intent + cached metadata.
6. **Namespacing:** All resources created by Cilo are discoverable via labels/prefixes for orphan detection and cleanup.

---

## 2) Resolved Risks (Phase A Complete)

✅ **Fixed in v0.1.20:**

1. ~~Global mutable state with no locking / non-atomic writes~~ → **Fixed:** File locking via `gofrs/flock` + atomic writes
2. ~~DNS config maintained by in-place text editing~~ → **Fixed:** Full render model from state
3. ~~Deep compose rewriting~~ → **Partial:** Using provider interface, full override model in future
4. ~~No reconciliation~~ → **Fixed:** `cilo doctor` + reconcile package
5. ~~Limited safeguards/tests~~ → **Fixed:** Subnet collision detection + orphan detection

---

## 3) What we keep from the original vision

Long-term, Cilo may support:
- Multiple runtimes (Docker/Podman/…)
- Remote hosts and mesh networking
- Shared resources across environments
- Multi-user / API usage

**Important:** This is a *direction*, not a commitment to build all of it next. The near-term plan below is explicitly narrower.

---

## 4) Refined phase plan (do the minimum that creates a strong architecture)

### Platform Support (0.2)

| Platform | Status | DNS Resolver |
|----------|--------|--------------|
| Linux (systemd-resolved) | First-class | `/etc/systemd/resolved.conf.d/cilo.conf` |
| macOS | First-class | `/etc/resolver/test` |
| Windows | Not supported | — |

### Phase 0: Runtime Abstraction ✅ COMPLETE

**Status:** Done (v0.1.19)  
**Delivered:** Clean `runtime.Provider` interface with Docker implementation

See: [Phase 0: Runtime Abstraction](./phases/PHASE_0_RUNTIME_ABSTRACTION.md)

---

### Phase A: Foundations Hardening ✅ COMPLETE

**Status:** Done (v0.1.20)  
**Goal:** Make local Cilo correct, stable, and maintainable. ✅ ACHIEVED

**Deliverables (must align with invariants):**
1. **State correctness**
   - File lock around all state mutations
   - Atomic writes (temp + rename)
   - Versioned schema and explicit migrations
2. **Compose override model**
   - Generate minimal override YAML; stop rewriting compose
   - Keep generated files in `.cilo/` and treat them as disposable
3. **DNS rendering model**
   - Render full dnsmasq config from state (no marker editing)
   - Forward non-.test queries to system resolver (not hardcoded upstream)
   - Validate output; atomic apply; safe reload strategy
4. **Reconciliation**
   - `doctor/status` reconciles state from runtime
   - Orphan detection (labels/prefixes) with clear reporting
5. **Collision detection**
   - Detect subnet collisions with existing Docker networks before allocation
   - Detect port/route collisions that would break DNS
6. **Integration tests**
   - End-to-end create/up/down/destroy + DNS
   - Concurrency test (parallel creates) and crash-interruption test

**Exit criteria (objective):**
- 100 concurrent creates do not corrupt state.
- DNS updates are atomic and recoverable.
- Compose features don’t break due to Cilo transformations (because transformation surface is minimal).

---

### Phase B: Shared Resources (optional; only if demanded)

**Goal:** Allow multiple environments to share a long-lived dependency (DB/cache/tools) without reintroducing fragility.

**Guardrails:**
- Must not undermine Phase A invariants.
- Docker-first is acceptable; multi-runtime support comes later.

See: [Phase 2A: Shared Resources](./phases/PHASE_2A_SHARED_RESOURCES.md)

---

### Phase C: Remote Operation (design gate before code)

Remote operation is not “just add a mesh provider”. It requires a concrete routing model.

**Design gate:** choose exactly one primary model before implementation:
1. **Subnet routing over mesh** (route Docker subnets; requires ACLs/routes/firewall correctness), or
2. **Host-level proxy/ingress** (terminate on host and forward internally), or
3. Another explicit alternative.

Only after the routing model is chosen should remote host management/mesh abstractions be built.

See: [Phase 2B: Remote Operation (Draft)](./phases/PHASE_2B_REMOTE_OPERATION.md)

---

### Phase D: Multi-user / API (separate product shift)

Multi-user and an API change the threat model and invariants. Treat as a separate initiative with a dedicated security model.

See: [Phase 3: Production Hardening (Draft)](./phases/PHASE_3_PRODUCTION_HARDENING.md)

---

## 5) Success metrics (kept, but scoped)

### Foundations (Phase A)
- [ ] 100 concurrent `cilo create` operations without state corruption
- [ ] DNS config updates are atomic and recoverable
- [ ] Crashed `cilo up` leaves the system recoverable via reconciliation (`doctor --fix`)
- [ ] Integration tests cover create/up/down/destroy/DNS + concurrency

---

## 6) Related docs

- [Architecture Design](./architecture/ARCHITECTURE_DESIGN.md)
- [Phase 0: Runtime Abstraction](./phases/PHASE_0_RUNTIME_ABSTRACTION.md) (required; clean provider boundary before Phase 1)
- [Phase 1: Foundations](./phases/PHASE_1_FOUNDATIONS.md)
- [Phase 1B: `cilo run` Command](./phases/PHASE_1B_CILO_RUN.md) (agent-first workflow)
- [Phase 2A: Shared Resources](./phases/PHASE_2A_SHARED_RESOURCES.md)
- Specs (Draft):
  - [Provider Interface Spec](./specs/PROVIDER_INTERFACE_SPEC.md)
  - [Mesh Networking Spec](./specs/MESH_NETWORKING_SPEC.md)
  - [State Schema](./specs/STATE_SCHEMA.md)
  - [DNS Architecture](./specs/DNS_ARCHITECTURE.md)
