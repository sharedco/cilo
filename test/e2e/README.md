# Cilo E2E Tests

These tests verify the complete Cilo workflow end-to-end.

## Prerequisites

1. Docker Engine running
2. cilo binary built (`go build -o cilo ./cmd/cilo`)
3. DNS configured (run `sudo cilo init` once)

## Running Tests

### All E2E tests

```bash
export CILO_E2E_ENABLED=true
export CILO_BINARY=./cilo
go test -tags e2e ./test/e2e/...
```

### Specific test

```bash
go test -tags e2e ./test/e2e -run TestFullWorkflow -v
```

### With verbose output

```bash
go test -tags e2e ./test/e2e/... -v
```

## Test Coverage

- `TestFullWorkflow` - Complete lifecycle (up, list, down, destroy)
- `TestParallelEnvironments` - Multiple simultaneous environments
- `TestDetectProject` - Project auto-detection
- `TestEnvRenderExample` - Environment variable rendering
- `TestCiloBasicExample` - Basic cilo functionality
- `TestGitSync` - Git synchronization
- `TestNestedGitSync` - Nested repository sync
- `TestSharedServicesBasic` - Shared services functionality
- `TestSharedServicesCLIFlags` - CLI flag handling
- `TestSharedServicesDoctor` - Doctor command tests

## Troubleshooting

**"cilo binary not found"**
- Set `CILO_BINARY` to the full path
- Or ensure `cilo` is in your PATH

**"Docker not available"**
- Start Docker Desktop or Docker Engine
- Verify with `docker info`

**DNS resolution fails**
- Run `sudo cilo init` to configure DNS
- Verify with `cilo dns status`

## License

MIT License - See LICENSES/MIT.txt
