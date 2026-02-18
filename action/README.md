# Cilo GitHub Action

Create isolated preview environments for pull requests with Cilo Cloud.

## Features

- 🚀 Automatic PR preview environments
- 🔒 Isolated network per environment
- 🧹 Automatic cleanup on PR close
- 📊 Environment status in PR comments

## Quick Start

### Prerequisites

1. A running Cilo server (self-hosted or cilocloud.dev)
2. An API key with `ci` scope
3. A `docker-compose.yml` in your repository

### Basic Usage

```yaml
name: Preview Environment

on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

jobs:
  preview:
    if: github.event.action != 'closed'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Create Preview
        id: preview
        uses: sharedco/cilo-action@v1
        with:
          server: ${{ vars.CILO_SERVER }}
          api-key: ${{ secrets.CILO_API_KEY }}
      
      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `🚀 Preview ready!\n\n${{ steps.preview.outputs.environment-url }}`
            })

  cleanup:
    if: github.event.action == 'closed'
    runs-on: ubuntu-latest
    steps:
      - uses: sharedco/cilo-action@v1
        with:
          server: ${{ vars.CILO_SERVER }}
          api-key: ${{ secrets.CILO_API_KEY }}
          action: destroy
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `server` | Yes | - | Cilo server URL |
| `api-key` | Yes | - | Cilo API key with `ci` scope |
| `environment-name` | No | `pr-<number>` | Custom environment name |
| `project-path` | No | `.` | Path to project directory |
| `action` | No | `create` | Action: `create`, `destroy`, `status` |
| `timeout` | No | `60` | Auto-destroy timeout in minutes |
| `wait` | No | `true` | Wait for environment to be ready |

## Outputs

| Output | Description |
|--------|-------------|
| `environment-url` | URL to access the environment |
| `environment-id` | Unique environment ID |
| `environment-status` | Current status |
| `services` | JSON object of service URLs |

## Examples

### Custom Environment Name

```yaml
- uses: sharedco/cilo-action@v1
  with:
    server: ${{ vars.CILO_SERVER }}
    api-key: ${{ secrets.CILO_API_KEY }}
    environment-name: feature-${{ github.event.pull_request.number }}
```

### Matrix Builds

```yaml
strategy:
  matrix:
    test-suite: [unit, integration, e2e]

steps:
  - uses: sharedco/cilo-action@v1
    with:
      server: ${{ vars.CILO_SERVER }}
      api-key: ${{ secrets.CILO_API_KEY }}
      environment-name: pr-${{ github.event.pull_request.number }}-${{ matrix.test-suite }}
```

### Custom Timeout

```yaml
- uses: sharedco/cilo-action@v1
  with:
    server: ${{ vars.CILO_SERVER }}
    api-key: ${{ secrets.CILO_API_KEY }}
    timeout: 120  # 2 hours
```

## Security

- Store your API key in GitHub Secrets
- Use a `ci` scoped API key (not `admin`)
- The action runs in CI mode (no WireGuard tunnel)

## Troubleshooting

### Environment Not Creating

1. Check that your `docker-compose.yml` is valid
2. Verify API key has correct permissions
3. Check server logs for errors

### Cleanup Not Working

The cleanup job only runs when the PR is closed. Make sure:
1. The `closed` event is in your workflow trigger
2. The `cleanup` job has the correct condition

## License

MIT
