# Future Vision: Remote & Mesh Networking

The ultimate goal of Cilo is to make "Remote" feel "Local." We want to create a transparent network fabric where a container on your laptop can talk to a container on a production-staging server via a standard DNS name.

## The Challenge
Standard Docker networking is host-bound. Accessing a remote container usually requires:
1. Mapping a port to the remote host.
2. Using a VPN to access the host.
3. Managing IP conflicts between local and remote subnets.

## The Cilo Vision: The Mesh Provider
We are implementing a **Mesh Provider Interface** that abstracts the underlying transport (Tailscale, WireGuard, or SSH Tunnels).

### Proposed Routing Models

#### 1. Host-Level Proxy (Near-term)
Cilo handles the DNS resolution locally but points the DNS entry to the **Mesh IP** of the remote host.
- `api.dev.remote.test` → `100.64.0.5` (Tailscale IP of the server).
- The remote host runs a global ingress (managed by Cilo) that routes traffic to the specific container.

#### 2. Subnet Routing (Long-term)
The Mesh Provider advertises the Cilo subnets (`10.224.x.x`) over the mesh.
- Your laptop "sees" the remote subnet.
- `api.dev.remote.test` → `10.224.50.2` (The actual container IP on the remote host).
- This is the highest fidelity model but requires deeper integration with the mesh network's routing table.

## Workspace Synchronization
Remote operation isn't just about networking; it's about the filesystem. 
- Cilo will implement a background sync (using `rsync` or `git-push-to-deploy`) that keeps your local edits in sync with the remote workspace.
- `cilo up --remote` will sync, then execute the compose command on the remote host.

## Multi-Tenancy
As we move to shared remote hosts, Cilo will add:
- **Authentication:** Per-user API keys for the Cilo daemon.
- **Quotas:** Limits on how many subnets/resources a single user can claim on a shared remote host.
