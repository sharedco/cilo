# env-render

This example demonstrates cilo's config-driven env rendering and `env_files` support.

## Usage

```bash
cilo create dev --from ./examples/env-render
cilo up dev
cilo status dev

curl http://nginx.dev.test/env.txt
curl http://node-api.dev.test:3000
curl http://python-api.dev.test:8000
```

You should see `APP_NAME=env-render-dev` and per-service env output with `BASE_URL` values that include the env name.
