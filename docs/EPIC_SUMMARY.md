# Cilo Architecture Epic - Summary

**Status:** Summary of the refined epic (see `ARCHITECTURE_EPIC.md` for the plan)

## What changed in the refined epic

The original planning direction was useful for vision, but it mixed near-term foundations with much larger initiatives (remote, multi-user, API). The refined epic:

- Keeps the long-term direction visible.
- Narrows near-term execution to **foundations hardening** (state/DNS/compose/reconciliation/tests).
- Adds explicit **architecture invariants** that prevent painting the system into a corner.
- Introduces a **design gate** for remote operation: choose a concrete routing model before writing code.

## The objective near-term goal

Make local Cilo reliable and non-fragile under real usage:

- state is concurrency-safe and crash-safe
- compose handling uses minimal overrides (no deep rewriting)
- DNS is rendered deterministically and applied atomically
- reconciliation makes runtime the source of truth
- tests cover the core lifecycle end-to-end

## Reading order

1. `docs/ARCHITECTURE_EPIC.md`
2. `docs/architecture/ARCHITECTURE_DESIGN.md`
3. `docs/phases/PHASE_1_FOUNDATIONS.md`
