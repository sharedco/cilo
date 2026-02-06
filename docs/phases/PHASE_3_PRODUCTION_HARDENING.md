# Phase 3: Production Hardening (Draft)

**Status:** Draft (separate product shift)

## Goal

Make Cilo safe and operable in a multi-user setting.

## Why this is a separate initiative

Multi-user support changes the threat model and “who can do what” invariants.
It should not be bolted onto a system that is still stabilizing its local correctness.

## Key questions (must answer before implementation)

1. **Authn/Authz model**
   - What identities exist (users, roles)?
   - What is the authentication mechanism (tokens, OIDC, SSH, etc.)?
   - What is authorized: per-project, per-host, per-environment?

2. **Isolation boundaries**
   - What must be isolated between users (filesystems, networks, logs)?
   - What shared resources are allowed?

3. **Quotas and enforcement**
   - What resources are metered (disk, CPU, memory, number of envs)?
   - Where is enforcement implemented (runtime provider, OS-level, cgroups)?

4. **Audit and observability**
   - Required audit events
   - Operational logs vs user-visible logs

5. **API surface**
   - If an API exists: who uses it, and why isn’t CLI enough?
   - gRPC vs REST: pick based on integration needs.

## Non-goals (initially)

- Full SaaS control plane
- Arbitrary plugin execution
