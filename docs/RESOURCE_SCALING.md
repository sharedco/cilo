# Resource Scaling Guide

Planning resources for Cilo environments on your machine.

## Understanding the Cost Model

Each Cilo environment runs your full Docker Compose stack in isolation:

```
1 Environment = 1 Copy of Your Stack
```

**Example Calculation:**
```
Your docker-compose.yml has:
  - 1 PostgreSQL container
  - 1 Redis container  
  - 1 API service container
  - 1 Web frontend container
  = 4 containers per environment

Running 10 environments:
  = 10 × 4 = 40 containers
```

## Resource Requirements by Stack Type

### Light Stack (2-3 containers)
Example: Simple API + database

| Resource | Per Environment | 10 Environments |
|----------|----------------|-----------------|
| RAM | 256-512 MB | 2.5-5 GB |
| CPU | 0.25-0.5 cores | 2.5-5 cores |
| Disk | ~50 MB | ~500 MB |

**Good for:** Microservices, simple backends, static sites

### Typical Stack (4-6 containers)
Example: API + DB + Cache + Worker + Frontend

| Resource | Per Environment | 10 Environments |
|----------|----------------|-----------------|
| RAM | 1-2 GB | 10-20 GB |
| CPU | 0.5-1 cores | 5-10 cores |
| Disk | ~200 MB | ~2 GB |

**Good for:** Full-stack apps, e-commerce, SaaS platforms

### Heavy Stack (7+ containers)
Example: Multi-service architecture with search, queues, monitoring

| Resource | Per Environment | 5 Environments |
|----------|----------------|----------------|
| RAM | 3-6 GB | 15-30 GB |
| CPU | 1-2 cores | 5-10 cores |
| Disk | ~1 GB | ~5 GB |

**Good for:** Enterprise apps, complex microservices

## Realistic Limits

### By Machine Type

**Developer Laptop (16GB RAM, 4-core):**
- Light stacks: 8-12 environments
- Typical stacks: 3-5 environments
- Heavy stacks: 1-2 environments

**Developer Workstation (32GB RAM, 8-core):**
- Light stacks: 20-25 environments
- Typical stacks: 8-12 environments
- Heavy stacks: 3-5 environments

**CI/CD Runner (64GB+ RAM, 16-core):**
- Light stacks: 40+ environments
- Typical stacks: 20-30 environments
- Heavy stacks: 8-12 environments

### Practical Recommendations

**Start Conservative:**
```bash
# Begin with 3-5 environments
cilo run opencode agent-1 "task 1" &
cilo run opencode agent-2 "task 2" &
cilo run opencode agent-3 "task 3" &
wait
```

**Monitor and Scale:**
```bash
# Watch resource usage
docker stats

# Check environment list
cilo list

# See which are running
```

**Aggressive Cleanup:**
```bash
# Destroy when done—environments are cheap to recreate
cilo destroy agent-1 --force

# Or destroy all at once
cilo destroy --all --force
```

## Optimizing Resource Usage

### 1. Limit Container Resources

Add resource constraints to your `docker-compose.yml`:

```yaml
services:
  db:
    image: postgres:15
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
  
  api:
    image: myapp-api
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
```

### 2. Use Lightweight Images

```yaml
# Good: Alpine-based images
services:
  db:
    image: postgres:15-alpine  # ~80MB vs ~400MB
  
  redis:
    image: redis:7-alpine      # ~30MB vs ~100MB
```

### 3. Share External Services

For development, consider sharing heavy services:

```yaml
# Use external database instead of per-environment
cilo run --env DB_HOST=shared-db.internal myapp agent-1
```

### 4. Disk Efficiency

Cilo uses copy-on-write (CoW) for workspace copies:
- **With CoW (APFS, btrfs, XFS):** ~0 extra disk per environment
- **Without CoW:** Linear growth (~50-200MB per environment)

Check your filesystem:
```bash
# macOS
diskutil info /

# Linux
findmnt -n -o FSTYPE /
```

## Monitoring Resources

### Docker Stats

```bash
# Real-time container stats
docker stats

# Or specific containers
docker stats cilo-myapp-agent-1-db-1 cilo-myapp-agent-1-api-1
```

### System Monitoring

```bash
# macOS
vm_stat  # Memory
iostat   # Disk

# Linux
free -h      # Memory
df -h        # Disk
top/htop     # CPU
```

### Cilo Doctor

```bash
# Check for orphaned resources
cilo doctor

# Auto-cleanup
cilo doctor --fix
```

## Troubleshooting Resource Issues

### "Out of memory" errors

1. **Reduce parallel environments:**
   ```bash
   # Instead of 10 parallel, run 3 at a time
   for batch in {1..3}; do
     cilo run opencode agent-$batch "task $batch" &
   done
   wait
   ```

2. **Add container memory limits** (see above)

3. **Use swap** (temporary fix):
   ```bash
   # Linux
   sudo fallocate -l 4G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   ```

### "No space left on device"

1. **Clean up old environments:**
   ```bash
   cilo list
cilo destroy old-env --force
   ```

2. **Prune Docker:**
   ```bash
   docker system prune -a
   ```

3. **Check CoW is working:**
   ```bash
   # Apparent vs actual size
du -sh ~/.cilo/envs/myapp/*
du -sh --apparent-size ~/.cilo/envs/myapp/*
   ```

### High CPU usage

1. **Add CPU limits** to compose file
2. **Reduce concurrent environments**
3. **Check for runaway processes:**
   ```bash
   docker top cilo-myapp-agent-1-api-1
   ```

## CI/CD Resource Planning

### GitHub Actions Example

```yaml
jobs:
  parallel-tests:
    runs-on: ubuntu-latest-8-cores  # Larger runner
    strategy:
      matrix:
        shard: [1, 2, 3, 4]
    steps:
      - uses: actions/checkout@v3
      - name: Setup Cilo
        run: |
          sudo cilo init
      - name: Run tests in isolated environment
        run: |
          cilo run npm test shard-${{ matrix.shard }} -- --shard=${{ matrix.shard }}/4
      - name: Cleanup
        if: always()
        run: |
          cilo destroy shard-${{ matrix.shard }} --force
```

### Resource Planning Formula

```
Max Parallel Environments = 
  MIN(
    floor(Total RAM / RAM per environment),
    floor(Total CPU cores / CPU per environment),
    floor(Docker daemon limits / containers per environment)
  )
```

**Example:**
```
Machine: 32GB RAM, 8 cores
Stack: 4 containers, 1GB RAM, 0.5 CPU per env

Max Environments = MIN(
  floor(32GB / 1GB) = 32,
  floor(8 cores / 0.5) = 16,
  100 (Docker default)
) = 16 environments
```

## Summary Checklist

- [ ] Measured baseline resources for one environment
- [ ] Added resource limits to docker-compose.yml
- [ ] Monitoring with `docker stats` or similar
- [ ] Cleanup strategy (manual or automated)
- [ ] Tested on representative hardware
- [ ] Documented limits for team
