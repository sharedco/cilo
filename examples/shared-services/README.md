# Shared Services Example

This example demonstrates cilo's shared services feature. Services can be marked as "shared" so they run once and are connected to multiple environments, saving resources.

## Setup

The `docker-compose.yml` defines:
- **elasticsearch**: A shared service (marked with `cilo.share: "true"`)
- **app**: An isolated service (one per environment)

## Usage

### Create multiple environments

```bash
# Create first environment
cilo create env1 --from .

# Create second environment  
cilo create env2 --from .
```

### Start environments with shared services

```bash
# Start env1 - creates shared elasticsearch
cilo up env1

# Start env2 - reuses the same elasticsearch
cilo up env2
```

Both environments will use the **same** elasticsearch container, but each will have its own `app` container.

### Override with flags

Force a service to be isolated:
```bash
cilo up env3 --isolate elasticsearch
```

Force a service to be shared:
```bash
cilo up env4 --share app
```

### Check status

```bash
cilo status env1
```

The output will show which services are "shared" vs "isolated".

### Cleanup

```bash
cilo down env1
cilo down env2
```

The shared elasticsearch container will be stopped 60 seconds after the last environment disconnects (grace period).

## How It Works

1. **Label Detection**: Services with `cilo.share: "true"` are automatically shared
2. **Container Reuse**: First environment creates the shared container
3. **Network Attachment**: Subsequent environments connect to the existing container
4. **DNS Transparency**: Each environment sees the shared service at `elasticsearch.env-name.test`
5. **Reference Counting**: Container is stopped when no environments are using it
6. **Grace Period**: 60-second delay before stopping unused shared services

## Doctor Command

Check for shared service issues:
```bash
cilo doctor
```

Fix orphaned or misconfigured shared services:
```bash
cilo doctor --fix
```

