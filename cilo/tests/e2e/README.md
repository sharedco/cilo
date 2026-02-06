# E2E tests

These tests are **opt-in** and require Docker + a configured cilo DNS setup.

Run:

```bash
export CILO_E2E=1
go test -tags e2e ./tests/e2e -run TestEnvRenderExample
```

The test uses the `examples/env-render` project and verifies env rendering after `cilo up`.
