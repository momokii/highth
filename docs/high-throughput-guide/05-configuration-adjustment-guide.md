# Configuration Adjustment Guide

This guide helps you adapt the PostgreSQL configuration to your specific hardware, schema size, and workload patterns. The default configuration is tuned for **8GB RAM, SSD storage, 8 CPU cores** — adjust if your system differs.

## Table of Contents

- [Recommendation Boundary](#recommendation-boundary)
- [Adjusting for Your Hardware](#adjusting-for-your-hardware)
- [Hardware Presets](#hardware-presets)
- [Adjusting for Your Schema & Workload](#adjusting-for-your-schema--workload)
- [Adjusting Redis Cache](#adjusting-redis-cache)
- [Quick Reference Table](#quick-reference-table)

---

## Recommendation Boundary

### Use This Configuration As-Is

The default configuration (8GB RAM preset) works well for:

- **Read-heavy / read-mostly workloads** where queries filter by a known identifier and order by timestamp
- **Append-only time-series data** (IoT, logs, events, metrics)
- **Single application instance** connecting directly to PostgreSQL
- **8GB RAM + SSD + 8 CPU cores** hardware (or larger)
- **Point queries with small result sets** (LIMIT 10-100)
- **Development and staging environments** as a reasonable production-like baseline

### Modify Before Using

| Condition | Recommended Changes |
|-----------|-------------------|
| **HDD storage** | Set `random_page_cost=4.0`, `effective_io_concurrency=2` |
| **< 8GB RAM** | Scale `shared_buffers` to 25% of actual RAM, scale `effective_cache_size` to 75% |
| **< 4GB RAM** | Reduce `shared_buffers` to 512MB-1GB, reduce `work_mem` to 4MB |
| **Write-heavy OLTP** (many UPDATEs/DELETEs) | Increase `wal_buffers` to 64MB (or let PG auto-calculate), increase `bgwriter_lru_maxpages` to 500, consider `max_wal_size` tuning |
| **Analytical / reporting queries** | Increase `work_mem` to 64-256MB (accepting fewer concurrent queries), increase `max_parallel_workers_per_gather` to 4 |
| **Multiple API instances** | Add PgBouncer in transaction mode between app and PG |
| **Production monitoring** | Add `shared_preload_libraries='pg_stat_statements'`, enable `auto_explain` for slow queries |
| **NVMe storage** | Can lower `random_page_cost` to 1.0, increase `effective_io_concurrency` to 200-500 |
| **High write throughput** (>50k inserts/sec) | Increase `wal_buffers` to 64-256MB, increase `checkpoint_completion_target` to 0.9 (already set), consider `wal_compression=on` |

### Do NOT Use This Configuration For

- **Shared multi-tenant databases** where one tenant's analytical query could monopolize resources (use resource groups / cgroups instead)
- **Systems with < 2GB RAM** (shared_buffers alone requires 2GB minimum)
- **HDD-based storage** without changing `random_page_cost` and `effective_io_concurrency`
- **Regulatory/compliance environments** requiring at-rest encryption, row-level security, or audit logging (no such extensions configured)
- **High-write OLTP** with frequent UPDATEs/DELETEs on the same rows (no HOT-chain optimization tuning, no autovacuum tuning)

---

## Adjusting for Your Hardware

### How to Determine Your Hardware Specs

**Linux:**
```bash
# RAM
free -h

# CPU cores
nproc

# Storage type (look for "rotational" - 0 means SSD/NVMe, 1 means HDD)
cat /sys/block/sda/queue/rotational

# Disk model
lsblk -d -o name,rota,model
```

**macOS:**
```bash
# RAM
system_profiler SPHardwareDataType | grep Memory

# CPU cores
sysctl -n hw.ncpu

# Storage type
diskutil info disk0 | grep "Solid State"
```

**Docker containers (limited resources):**
Check your Docker Desktop or compose resource limits:
- Docker Desktop: Settings → Resources
- docker-compose: `services.postgres.deploy.resources.limits`

### Memory Parameters

#### shared_buffers

**Formula:** `shared_buffers = 25% of RAM`

| RAM | shared_buffers |
|-----|----------------|
| 4GB | 1GB |
| 8GB | 2GB |
| 16GB | 4GB |
| 32GB | 8GB |
| 64GB | 16GB |

**Rationale:** PostgreSQL's shared disk cache. Traditional OLTP baseline. Too low = excessive disk I/O. Too high = OS has less memory for file system cache.

**Exception:** On systems with < 4GB RAM, use 512MB-1GB maximum to avoid starving the OS.

#### effective_cache_size

**Formula:** `effective_cache_size = 75% of RAM`

| RAM | effective_cache_size |
|-----|----------------------|
| 4GB | 3GB |
| 8GB | 6GB |
| 16GB | 12GB |
| 32GB | 24GB |
| 64GB | 48GB |

**Rationale:** Planner's estimate of total cache (PostgreSQL shared buffers + OS file system cache). This is a **hint only** — it does NOT allocate memory. Setting it higher on modern systems is generally safe.

#### work_mem

**Formula:** `work_mem = (RAM - shared_buffers) / (max_connections × 4)`

Default: **16MB** for 8GB RAM system

| RAM | Recommended work_mem |
|-----|---------------------|
| 4GB | 4-8MB |
| 8GB | 16MB |
| 16GB | 32-64MB |
| 32GB | 64-128MB |
| 64GB | 128-256MB |

**Critical:** `work_mem` is **per query execution node**, not per query. A query with 4 sort/hash operations can use up to 4× work_mem.

**Example calculation for 8GB RAM:**
- shared_buffers = 2GB
- Remaining = 6GB
- max_connections = 200
- 6GB / (200 × 4) = 7.5MB → round up to 16MB for safety margin

**For analytical queries:** Increase to 64-256MB, but reduce `max_connections` accordingly to avoid OOM.

#### maintenance_work_mem

**Formula:** `maintenance_work_mem = 1-2GB` (for systems with 8GB+ RAM)

| RAM | maintenance_work_mem |
|-----|----------------------|
| 4GB | 256MB-512MB |
| 8GB | 1GB |
| 16GB | 2GB |
| 32GB | 4GB |
| 64GB | 8GB |

**Rationale:** Memory for VACUUM, CREATE INDEX, ALTER TABLE. Maintenance operations are infrequent and single-threaded, so larger values dramatically speed up index creation without hurting query performance.

### SSD/HDD Planner Parameters

#### random_page_cost

**Storage-based selection:**

| Storage Type | random_page_cost |
|--------------|------------------|
| HDD | 4.0 (PostgreSQL default) |
| SSD | 1.1 |
| NVMe | 1.0 |

**Rationale:** Cost estimate for non-sequential disk page fetch. On HDD, random access is much slower than sequential. On SSD/NVMe, they're nearly equal.

#### effective_io_concurrency

**Storage-based selection:**

| Storage Type | effective_io_concurrency |
|--------------|-------------------------|
| HDD | 1-2 |
| SATA SSD | 100-200 |
| NVMe SSD | 200-500 |

**Rationale:** Estimated number of concurrent I/O operations the storage can handle. Modern SSDs can handle many parallel operations efficiently.

### CPU/Parallelism Parameters

#### max_worker_processes

**Formula:** `max_worker_processes = number of CPU cores`

| CPU Cores | max_worker_processes |
|-----------|---------------------|
| 2 | 2 |
| 4 | 4 |
| 8 | 8 |
| 16 | 16 |
| 32+ | 32 (cap at reasonable limit) |

**Rationale:** Maximum background worker processes for parallel queries, autovacuum, etc. Should match CPU core count.

#### max_parallel_workers_per_gather

**Formula:** `max_parallel_workers_per_gather = min(CPU cores / 4, 4)`

| CPU Cores | Recommended |
|-----------|-------------|
| 2 | 1 |
| 4 | 2 |
| 8 | 2-4 |
| 16 | 4 |
| 32+ | 4-6 |

**Rationale:** Maximum parallel workers for a single query operation. Setting too high causes CPU contention under concurrent queries. Conservative settings (2-4) work best for mixed workloads.

#### max_parallel_workers

**Formula:** `max_parallel_workers = max_worker_processes - 2` (reserve 2 for autovacuum)

| CPU Cores | max_parallel_workers |
|-----------|---------------------|
| 2 | 2 (with reservation) |
| 4 | 4-6 |
| 8 | 8-12 |
| 16 | 16-20 |

**Rationale:** Maximum total parallel workers across all operations. Should allow multiple queries to use parallelism simultaneously.

### Connection Parameters

#### max_connections

**Formula:** `max_connections = (expected app instances × pool size) + 20% buffer`

| Deployment | max_connections |
|------------|-----------------|
| Single app instance, pool size 25-50 | 100 |
| Multiple app instances | 200-300 |
| With PgBouncer | 100 (PgBouncer multiplexes connections) |

**Rationale:** Each connection consumes ~2-10MB RAM. Too many connections cause memory overhead and connection contention. For high concurrency, use PgBouncer.

#### pgxpool Settings (Application-side)

**Formula:** `MaxConns = (CPU cores × 2) + effective_spindle_count`

| CPU Cores | DB_MAX_CONNECTIONS (MaxConns) | DB_MIN_CONNECTIONS (MinConns) |
|-----------|------------------------------|----------------------------|
| 2 | 10-15 | 3-5 |
| 4 | 25-30 | 5-10 |
| 8 | 40-50 | 10-15 |
| 16 | 60-80 | 15-25 |

**Additional settings (same for all configs):**
- `DB_MAX_CONN_LIFETIME`: 1h
- `DB_MAX_CONN_IDLE_TIME`: 10m
- `DB_HEALTH_CHECK_PERIOD`: 30s

### Background Writer Parameters

#### bgwriter_delay

**Value:** 200ms (default for most workloads)

**Exception:** For very high write throughput, reduce to 100-150ms.

#### bgwriter_lru_maxpages

**Write throughput based:**

| Write Rate | bgwriter_lru_maxpages |
|-----------|----------------------|
| Low (< 1k inserts/sec) | 100 (default) |
| Medium (1k-10k inserts/sec) | 500 |
| High (> 10k inserts/sec) | 1000 |

**Rationale:** Maximum dirty buffers flushed per background writer round. Higher values allow the background writer to keep up with high insert rates, reducing checkpoint burden.

---

## Hardware Presets

Copy-paste these directly into your `docker-compose.yml`.

### Preset: 4GB RAM / 2 Cores / SSD

**Use for:** Development, small datasets (< 10M rows)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=100
      -c shared_buffers=1GB
      -c effective_cache_size=3GB
      -c work_mem=4MB
      -c maintenance_work_mem=512MB
      -c random_page_cost=1.1
      -c effective_io_concurrency=100
      -c wal_buffers=16MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=2
      -c max_parallel_workers_per_gather=1
      -c max_parallel_workers=2
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=100
    environment:
      DB_MAX_CONNECTIONS: 10
      DB_MIN_CONNECTIONS: 3

  redis:
    command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
```

### Preset: 8GB RAM / 4 Cores / SSD (Current Default)

**Use for:** Production with 10-50M rows

```yaml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=2GB
      -c effective_cache_size=6GB
      -c work_mem=16MB
      -c maintenance_work_mem=1GB
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
      -c wal_buffers=16MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=4
      -c max_parallel_workers_per_gather=2
      -c max_parallel_workers=4
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=100
    environment:
      DB_MAX_CONNECTIONS: 25
      DB_MIN_CONNECTIONS: 5

  redis:
    command: redis-server --appendonly yes --maxmemory 512mb --maxmemory-policy allkeys-lru
```

### Preset: 16GB RAM / 8 Cores / SSD

**Use for:** Production with 50-200M rows

```yaml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=4GB
      -c effective_cache_size=12GB
      -c work_mem=32MB
      -c maintenance_work_mem=2GB
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
      -c wal_buffers=32MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=8
      -c max_parallel_workers_per_gather=4
      -c max_parallel_workers=8
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=500
    environment:
      DB_MAX_CONNECTIONS: 50
      DB_MIN_CONNECTIONS: 10

  redis:
    command: redis-server --appendonly yes --maxmemory 1gb --maxmemory-policy allkeys-lru
```

### Preset: 32GB RAM / 16 Cores / NVMe

**Use for:** Production with 200-500M rows, analytical queries

```yaml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=8GB
      -c effective_cache_size=24GB
      -c work_mem=64MB
      -c maintenance_work_mem=4GB
      -c random_page_cost=1.0
      -c effective_io_concurrency=200
      -c wal_buffers=64MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=16
      -c max_parallel_workers_per_gather=4
      -c max_parallel_workers=16
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=500
    environment:
      DB_MAX_CONNECTIONS: 50
      DB_MIN_CONNECTIONS: 10

  redis:
    command: redis-server --appendonly yes --maxmemory 2gb --maxmemory-policy allkeys-lru
```

### Preset: 64GB RAM / 16 Cores / NVMe

**Use for:** Production with 500M+ rows, heavy analytical workload

```yaml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=300
      -c shared_buffers=16GB
      -c effective_cache_size=48GB
      -c work_mem=128MB
      -c maintenance_work_mem=8GB
      -c random_page_cost=1.0
      -c effective_io_concurrency=300
      -c wal_buffers=128MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=16
      -c max_parallel_workers_per_gather=6
      -c max_parallel_workers=16
      -c bgwriter_delay=150ms
      -c bgwriter_lru_maxpages=1000
    environment:
      DB_MAX_CONNECTIONS: 80
      DB_MIN_CONNECTIONS: 20

  redis:
    command: redis-server --appendonly yes --maxmemory 4gb --maxmemory-policy allkeys-lru
```

---

## Adjusting for Your Schema & Workload

### Schema Size Impact

| Row Count | Shared Buffers | Work Mem | Notes |
|-----------|---------------|----------|-------|
| < 10M | Use default | Use default | Indexes fit in memory easily |
| 10M-50M | Use default | Use default | Covering index recommended |
| 50M-200M | Consider +50% | Use default | Consider partitioning |
| 200M-1B | +50-100% | +50% | Partitioning recommended |

**Index maintenance at scale:**
- > 50M rows: Use `CREATE INDEX CONCURRENTLY` to avoid table locks
- > 100M rows: Increase `maintenance_work_mem` for faster index builds
- > 500M rows: Consider partitioning by time (monthly partitions)

### Read-Heavy vs Write-Heavy Workloads

**Read-heavy (default config optimized for this):**
- Higher `shared_buffers` (25% of RAM)
- Lower `wal_buffers` (16MB is sufficient)
- Covering indexes for index-only scans
- Redis caching with 30s TTL

**Write-heavy adjustments:**
```yaml
# Increase WAL buffers for high write throughput
-c wal_buffers=64MB

# More aggressive background writer
-c bgwriter_delay=150ms
-c bgwriter_lru_maxpages=500

# Consider enabling WAL compression
-c wal_compression=on
```

**Mixed read/write:**
- Use default settings
- Monitor `pg_stat_bgwriter` to tune `bgwriter_lru_maxpages`
- Monitor checkpoint duration with `pg_stat_bgwriter`

### Multi-Tenant Considerations

For multi-tenant databases where one tenant's queries could monopolize resources:

1. **Use resource groups** (PostgreSQL 16+): CREATE RESOURCE GROUP
2. **Or use row-level limits**: Set `statement_timeout` per connection
3. **Or use connection pooling with limits**: PgBouncer with per-tenant pools

### When to Enable Partitioning

**Enable partitioning when:**
- Table size > 100M rows
- Data has natural time-based partitioning key
- You frequently drop old data
- Query patterns always include partition key in WHERE clause

**Partitioning example (monthly):**
```sql
-- Create partitioned table
CREATE TABLE sensor_readings (
    id              BIGSERIAL,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(20)     NOT NULL,
    value           DECIMAL(10,2)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions
CREATE TABLE sensor_readings_2024_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE sensor_readings_2024_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
```

---

## Adjusting Redis Cache

### Cache TTL by Freshness Requirement

| Freshness Need | TTL | Use Case |
|---------------|-----|----------|
| Real-time (< 5 seconds) | 5s | Live dashboards, monitoring alerts |
| Near real-time (< 1 minute) | 30s | **Default** for IoT monitoring |
| Acceptable delay (< 5 minutes) | 60s | Analytics, reporting |
| Mostly static | 300s | Reference data |

**How to change:** Set `REDIS_TTL` environment variable:
```bash
REDIS_TTL=60s  # 60 seconds
```

### Cache Memory by Dataset Size

| Dataset Size | Redis maxmemory | Notes |
|-------------|-----------------|-------|
| < 1M rows | 128MB | Small datasets |
| 1M-10M rows | 256-512MB | **Default**: 512MB |
| 10M-100M rows | 1-2GB | Large datasets |
| > 100M rows | 4GB+ | Enterprise scale |

**How to change:** Modify redis command in docker-compose.yml:
```yaml
redis:
  command: redis-server --appendonly yes --maxmemory 1gb --maxmemory-policy allkeys-lru
```

---

## Quick Reference Table

### All 14 PostgreSQL Parameters

| Parameter | Formula/Range | Minimum | Maximum | Notes |
|-----------|--------------|---------|----------|-------|
| `shared_buffers` | 25% of RAM | 128MB | 32GB | PostgreSQL shared cache |
| `effective_cache_size` | 75% of RAM | 1GB | System RAM | Planner hint only |
| `work_mem` | (RAM - shared_buffers) / (max_connections × 4) | 4MB | 1GB | Per sort/hash operation |
| `maintenance_work_mem` | 1-2GB (8GB+ RAM) | 128MB | 16GB | VACUUM, CREATE INDEX |
| `max_connections` | (instances × pool_size) + 20% | 20 | 1000+ | Connection limit |
| `wal_buffers` | 16MB (default), 64MB+ (write-heavy) | 1MB | 1GB | WAL buffer |
| `checkpoint_completion_target` | 0.9 (default) | 0.5 | 0.95 | Spread checkpoint I/O |
| `random_page_cost` | 4.0 (HDD), 1.1 (SSD), 1.0 (NVMe) | 1.0 | 4.0 | Planner cost for random I/O |
| `effective_io_concurrency` | 1-2 (HDD), 100-200 (SSD), 200-500 (NVMe) | 1 | 1000+ | Concurrent I/O ops |
| `max_worker_processes` | CPU cores | 1 | 64 | Background workers |
| `max_parallel_workers_per_gather` | min(CPU cores / 4, 4) | 0 | 8 | Parallel workers per query |
| `max_parallel_workers` | max_worker_processes - 2 | 1 | 64 | Total parallel workers |
| `bgwriter_delay` | 150-200ms | 10ms | 1s | Background writer interval |
| `bgwriter_lru_maxpages` | 100 (low write), 500-1000 (high write) | 10 | 10000+ | Pages flushed per round |

---

## Next Steps

- [PostgreSQL Setup](./01-postgresql-setup.md) — Detailed parameter explanations
- [General Setup Guide](./04-general-setup-guide.md) — Step-by-step implementation
- [Main README](../../README.md) — Project overview
