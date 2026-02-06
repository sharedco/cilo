# Operations & Maintenance

## Initial Setup (`cilo init`)
Running `sudo cilo init` performs a one-time configuration of your machine:
1. **Directory Scaffolding:** Creates `~/.cilo/` for state and workspace data.
2. **DNS Daemon:** Installs and starts a local `dnsmasq` instance listening on `127.0.0.1:5354`.
3. **System Resolver:**
   - **Linux:** Adds a config to `/etc/systemd/resolved.conf.d/` to forward `.test` queries.
   - **macOS:** Creates an entry in `/etc/resolver/test`.

## Health & Repair (`cilo doctor`)
If DNS isn't resolving or environments feel "stuck":
- `cilo doctor`: Scans state for inconsistencies and checks if Docker/dnsmasq are running.
- `cilo doctor --fix`: Attempts to recreate missing networks, restart the DNS daemon, and prune orphaned containers.

## Manual Uninstallation
If you need to remove Cilo completely:
1. **Stop Environments:** `cilo list` then `cilo destroy <env>` for each.
2. **Clean System DNS:**
   - Linux: `sudo rm /etc/systemd/resolved.conf.d/cilo.conf && sudo systemctl restart systemd-resolved`
   - macOS: `sudo rm /etc/resolver/test`
3. **Remove Data:** `rm -rf ~/.cilo`
4. **Remove Binary:** `rm $(which cilo)`

## Port Conflicts
By default, Cilo's DNS daemon uses port `5354`. If this port is taken, you can re-initialize with:
`sudo cilo init --dns-port 5454`
