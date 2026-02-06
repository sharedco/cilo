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

## 5. Environment Variable Management
Cilo provides a sophisticated, config-driven mechanism for managing environment variables across different isolated workspaces.

### Workspace Isolation
Each environment maintains its own set of `.env` files within its dedicated workspace directory (`~/.cilo/envs/<project>/<env>/`). This allows agents and developers to modify environment variables for one instance without affecting others.

### The Env Lifecycle
When an environment is created or updated:
1.  **Init Hook:** Cilo can execute a shell command (e.g., `infisical export` or `aws ssm get-parameter`) to pull secrets or generate base `.env` files.
2.  **Copy Policy:** Cilo filters which `.env` files from the source project are copied into the workspace (modes: `all`, `none`, or `allowlist`).
3.  **Token Rendering:** Cilo scans the `.env` files and replaces specific tokens with environment-specific values:
    - `${CILO_PROJECT}`: The project name.
    - `${CILO_ENV}`: The environment name.
    - `${CILO_DNS_SUFFIX}`: The TLD (e.g., `.test`).
    - `${CILO_BASE_URL}`: The fully qualified URL of the project apex (e.g., `http://myapp.dev.test`).

This ensures that services can automatically discover their own external URLs and sibling services within the same isolated namespace.
