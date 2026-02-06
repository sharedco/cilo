# Examples

This folder contains multiple small Docker Compose projects you can use to test Cilo.

## Quickstart

```bash
cilo init
sudo cilo dns setup

# Pick an example project and create an environment from it
cilo create demo --from ./examples/basic
cilo up demo
cilo status demo

curl http://nginx.demo.test
curl http://api.demo.test:8080

cilo destroy demo --force
```

## Example projects

### `examples/basic`

The original single example project (nginx + api + redis).

### `examples/basic-2`

Same topology but different HTML so you can run two unrelated projects side-by-side.

### `examples/ingress-hostnames`

Shows the "multiple hostnames behind one nginx" paradigm.

Today, Cilo can map DNS names per *service* and per *environment*.
For real-world setups where one nginx serves multiple hostnames (vhosts), the right model is:

`<hostname>.<project>.<env>.test` → (ingress nginx IP)

Cilo does not yet have a first-class "project" concept or wildcard host mappings.
This example exists to make that desired capability concrete.

### `examples/env-render`

Shows config-driven env rendering (`.cilo/config.yml` + `.env` tokens) and `env_files` support.

### `examples/custom-dns-suffix`

Demonstrates how to change the TLD from `.test` to `.localhost` (or any other suffix).
