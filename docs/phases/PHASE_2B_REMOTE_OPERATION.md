# Phase 2B: Remote Operation (Draft)

**Status:** Draft (design gate; not an implementation guide yet)

## Goal

Enable creating and operating environments on remote hosts, while preserving the Phase A invariants (determinism, crash safety, reconciliation).

## Design gate (must decide before coding)

Remote DNS + connectivity requires an explicit routing model. Choose exactly one primary model:

1. **Subnet routing over mesh**
   - Route Docker subnets over the mesh (e.g. Tailscale/WireGuard).
   - Requires route advertisement/acceptance, ACLs, firewall, MTU considerations.

2. **Host-level proxy/ingress**
   - DNS resolves to a host-reachable IP; host terminates and forwards to containers.
   - Requires choosing L4/L7 strategy and consistent port/hostname mapping.

3. **Other (must be explicit)**

**Exit criteria for this section:** A concrete end-to-end story for: name resolution → routable IP → connectivity to the correct container/service.

## Non-goals (for the first remote iteration)

- Multi-user authentication/authorization
- Full mesh-provider plugin ecosystem
- Kubernetes provider

## Expected deliverables (after the routing model is chosen)

- Remote host configuration model (how to add/list/remove hosts)
- Remote execution strategy (how commands run on the remote)
- Workspace sync strategy (rsync vs git vs other)
- Reconciliation strategy for remote (source of truth and failure modes)
- DNS strategy that matches the chosen routing model
