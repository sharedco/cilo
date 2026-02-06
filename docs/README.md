# Cilo Documentation

This directory contains architectural and implementation documentation for Cilo.

---

## Quick Links

### Planning Documents
- **[Architecture Epic](./ARCHITECTURE_EPIC.md)** - Refined plan, invariants, phased scope
- **[Architecture Design](./architecture/ARCHITECTURE_DESIGN.md)** - Deep technical design

### Phase Implementation Guides
- **[Phase 0: Runtime Abstraction](./phases/PHASE_0_RUNTIME_ABSTRACTION.md)** ✅ Complete - Provider interface
- **[Phase 1: Foundations](./phases/PHASE_1_FOUNDATIONS.md)** ✅ Complete (v0.1.20) - File locking, atomic writes, DNS render
- **[Phase 1B: `cilo run` Command](./phases/PHASE_1B_CILO_RUN.md)** ✅ Complete - Agent-first workflow
- **[Phase 2A: Shared Resources](./phases/PHASE_2A_SHARED_RESOURCES.md)** 📝 Optional - Not yet implemented
- **[Phase 2B: Remote Operation](./phases/PHASE_2B_REMOTE_OPERATION.md)** 📝 Draft - Requires routing model decision
- **[Phase 3: Production Hardening](./phases/PHASE_3_PRODUCTION_HARDENING.md)** 📝 Draft - Future multi-user/API work

### Technical Specifications
- **[Mesh Networking Spec](./specs/MESH_NETWORKING_SPEC.md)** - Draft (for Phase 2B)

*Note: Provider Interface, State Schema, and DNS Architecture specs have been implemented and are now documented in code.*

### Operations
- **[Init and Uninstall Runbook](./ops/INIT_AND_UNINSTALL.md)** - What init creates, how to uninstall

---

## Document Purpose

| Document | Audience | Purpose |
|----------|----------|---------|
| Architecture Epic | Tech leads | Invariants + scoped roadmap + success criteria |
| Architecture Design | Engineers | System design, concepts, flows |
| Phase Guides | Implementers | Task breakdowns and implementation notes |
| Technical Specs | Integrators | Draft contracts/interfaces (as needed) |

---

## Reading Path

### For Implementers (Start to Finish)
1. Read [Architecture Epic](./ARCHITECTURE_EPIC.md) - understand the why
2. Read [Architecture Design](./architecture/ARCHITECTURE_DESIGN.md) - understand the what
3. Execute phases in order:
   - Phase 0 → Phase 1 → Phase 2A → Phase 2B → Phase 3
4. Reference technical specs as needed during implementation

### For Reviewers
1. [Architecture Epic](./ARCHITECTURE_EPIC.md) - strategic direction
2. [Architecture Design](./architecture/ARCHITECTURE_DESIGN.md) - system design
3. Relevant phase docs for implementation details

### For Integrators (Adding Providers/Extensions)
1. [Provider Interface Spec](./specs/PROVIDER_INTERFACE_SPEC.md)
2. [Mesh Networking Spec](./specs/MESH_NETWORKING_SPEC.md)
3. [Architecture Design](./architecture/ARCHITECTURE_DESIGN.md) - extensibility points

---

## Phase Dependencies

```
Phase 0: Runtime Abstraction
    ↓
Phase 1: Foundations ←──────────┐
    │                           │
    ├─→ Phase 1B: cilo run ─────┤  (agent-first DX)
    │                           │
    ├─→ Phase 2A: Shared ───────┤
    │                           │
    └─→ Phase 2B: Remote ───────┤
                ↓               │
            Phase 3: Production ─┘
```

**Key:** Phase 1B, 2A, and 2B can be done in parallel (all depend on Phase 1)

---

## Current Status

- [x] Phase 0: Runtime Abstraction ✅
- [x] Phase 1: Foundations ✅ (v0.1.20 - stable foundation)
- [x] Phase 1B: `cilo run` Command ✅
- [ ] Phase 2A: Shared Resources 📝 Optional
- [ ] Phase 2B: Remote Operation 📝 Draft
- [ ] Phase 3: Production Hardening 📝 Draft

---

## Contributing to Docs

When adding new documentation:

1. **Phases:** Add to `phases/PHASE_N_NAME.md`
2. **Specs:** Add to `specs/SPEC_NAME.md`
3. **Architecture:** Update `architecture/ARCHITECTURE_DESIGN.md`
4. **Update this README** with links

### Documentation Standards

- Use markdown
- Include code examples for all APIs
- Add diagrams for complex flows (ASCII art or mermaid)
- Keep "Success Criteria" measurable and testable
- Include migration guides for breaking changes

---

## Questions?

For clarification on any phase or spec:
1. Check the relevant document first
2. Review [Architecture Design](./architecture/ARCHITECTURE_DESIGN.md) for context
3. Open an issue with `[docs]` prefix
