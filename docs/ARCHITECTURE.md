# Cilo Architecture

Cilo is designed around the principle of **Network-Level Isolation**. Instead of the "Port Mapping" model used by standard Docker Compose, Cilo treats each environment as a first-class citizen on the host network.

## 1. The Subnet Model
Every Cilo environment is assigned a unique `/24` subnet (defaulting to the `10.224.0.0/16` range). 
- **Isolation:** Containers in `env-a` cannot communicate with `env-b` unless explicitly linked.
- **Predictability:** Services are assigned stable internal IPs within their subnet (e.g., `10.224.1.2`), which are then mapped to DNS.

## 2. DNS-First Discovery
Cilo manages a local `dnsmasq` instance that acts as the source of truth for the `.test` TLD (or a custom suffix).
- **System Integration:** During `init`, Cilo configures the system resolver (via `systemd-resolved` or `/etc/resolver/`) to forward queries for the chosen suffix to the Cilo DNS daemon.
- **Dynamic Rendering:** When an environment is brought `up`, Cilo reconciles the actual container IPs and regenerates the `dnsmasq` configuration atomically.

## 3. Non-Destructive Overrides
Cilo respects your source code. It never modifies your `docker-compose.yml`.
- **The Override Pattern:** Cilo generates a hidden `.cilo/override.yml` in the environment workspace. 
- **Injected Logic:** This override disables port publishing (`ports: []`) and injects the Cilo-managed network and static IP configuration.
- **Execution:** `docker compose -f base.yml -f .cilo/override.yml up`

## 4. State & Atomicity
To ensure reliability for automated agents:
- **Flock:** Every state mutation is protected by an advisory file lock on `state.json`.
- **Atomic Writes:** State and DNS updates use a "Write-Temp-Then-Rename" pattern to prevent corruption during system crashes or concurrent calls.
- **Reconciliation:** The `doctor` command uses the Docker engine as the source of truth to repair any drift in the file-based state.
