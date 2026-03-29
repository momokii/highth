# Architecture Design

This document covers the complete system architecture including database schema design, indexing strategy, caching layer, and infrastructure decisions.

## Table of Contents

- [Database Schema Design](#database-schema-design)
- [Indexing Strategy](#indexing-strategy)
- [Caching Layer](#caching-layer)
- [Connection Pooling](#connection-pooling)
- [Partitioning Strategy](#partitioning-strategy)
- [Infrastructure Components](#infrastructure-components)
- [Generalizability](#generalizability)

---

## Database Schema Design

### Core Table: `sensor_readings`

```sql
CREATE TABLE sensor_readings (
    id              BIGSERIAL       PRIMARY KEY,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(20)     NOT NULL,
    value           DECIMAL(10,2)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL
);
```

> **Note:** The original design specified `reading_type VARCHAR(30)`, `value NUMERIC(15,6)`, and `metadata JSONB`. The current implementation uses `VARCHAR(20)` with a CHECK constraint for known reading types, and `DECIMAL(10,2)` which is sufficient for IoT sensor data. See [Future Enhancements](../future-enhancements/04-schema-type-corrections.md) for alignment options.

#### Column Descriptions

| Column | Type | Purpose |
|--------|------|---------|
| `id` | BIGSERIAL | Primary key; unique identifier for each reading |
| `device_id` | VARCHAR(50) | The repeating identifier; groups readings by device |
| `timestamp` | TIMESTAMPTZ | When the reading was taken; supports time-series queries |
| `reading_type` | VARCHAR(20) | Type of sensor reading (temperature, humidity, pressure) with CHECK constraint |
| `value` | DECIMAL(10,2) | The actual sensor value; precision sufficient for IoT data |
| `unit` | VARCHAR(20) | Unit of measurement (celsius, percent, pascals, etc.) |

### Schema Design Rationale

**Why DECIMAL(10,2) for value?**

- **10 total digits** — Supports values up to ±99,999,999.99
- **2 decimal places** — Precision sufficient for IoT sensor data (temperature, humidity, pressure)
- **Exact decimal arithmetic** — No floating-point rounding errors

> **Note:** For scientific applications requiring 6 decimal places, see [Future Enhancements](../future-enhancements/04-schema-type-corrections.md).

**CHECK constraint on reading_type:**

The schema includes a CHECK constraint to ensure data integrity:
```sql
CHECK (reading_type IN ('temperature', 'humidity', 'pressure'))
```

This prevents typos and ensures only valid reading types are stored.

---

## Indexing Strategy

Proper indexing is critical to meeting the ≤500ms performance target at 50M rows. Our strategy uses multiple complementary indexes.

### Index 1: BRIN Index for Time-Series Queries

```sql
CREATE INDEX idx_sensor_readings_timestamp_brin
    ON sensor_readings
    USING BRIN (timestamp);
```

**What is BRIN?**

BRIN (Block Range INdex) is a space-efficient index type designed for append-only, sequentially-ordered data. Instead of storing a row pointer for every entry, BRIN stores summary information for ranges of consecutive pages.

**Why BRIN for timestamp?**

| Aspect | B-tree | BRIN |
|--------|--------|------|
| Index size at 50M rows | ~2-3 GB | ~20-30 MB |
| Build time | Slow (minutes) | Fast (seconds) |
| Query pattern | Good for random access | Excellent for sequential data |
| Best for | OLTP workloads | Time-series/append-only |

For time-series data where readings are appended in roughly chronological order, BRIN provides:

- **100x smaller index size** — More memory for cache
- **Faster index maintenance** — Less overhead on inserts
- **Sufficient query performance** — Time ranges naturally map to page ranges

### Index 2: Composite B-tree for Device Lookups (Actual Implementation)

```sql
CREATE INDEX idx_sensor_readings_device_type_timestamp
    ON sensor_readings (device_id, reading_type, timestamp DESC);
```

**Why include reading_type in the composite index?**

The actual implementation includes `reading_type` as the second column, which provides additional optimization for queries filtering by reading type:

```sql
-- Query pattern with reading_type filter
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
  AND reading_type = 'temperature'
ORDER BY timestamp DESC
LIMIT 10;
```

This index efficiently serves both:
- Queries with device_id only (first column is used)
- Queries with device_id + reading_type (first two columns are used)

> **Note:** The original design specified a simpler composite index on `(device_id, timestamp DESC)`. The actual implementation adds `reading_type` for additional filtering capability. See [future-enhancements/04-schema-type-corrections.md](../future-enhancements/04-schema-type-corrections.md) for alignment options.

### Index 3: Covering Index for Index-Only Scans (Added in Migration 006)

```sql
CREATE INDEX idx_sensor_readings_device_covering
    ON sensor_readings (device_id, timestamp DESC)
    INCLUDE (reading_type, value, unit);
```

**What is a covering index?**

A covering index includes non-key columns that are frequently accessed, allowing PostgreSQL to satisfy queries directly from the index without accessing the heap (main table storage).

**Index-Only Scan benefit:**

For our typical query, PostgreSQL only needs:
- `device_id` (key column)
- `timestamp` (key column)
- `reading_type` (included column)
- `value` (included column)
- `unit` (included column)

All of these are in the covering index — **no heap access required**.

**Performance impact:**
- Eliminates random I/O to heap pages
- Reduces cache misses
- Typically 2-5x faster than queries requiring heap access
- Before: Index Scan + Heap Access (50-200ms on 50M rows)
- After: Index-Only Scan (5-50ms on 50M rows)

---

## Caching Layer

### Redis Configuration

**Cache key pattern:** `sensor:{device_id}:readings:{limit}[:{reading_type}]`

Examples:
- `sensor:sensor-001:readings:10` — Last 10 readings for sensor-001
- `sensor:sensor-002:readings:50:temperature` — Last 50 temperature readings

**TTL strategy:** 30 seconds

### Why 30-Second TTL?

| TTL Option | Benefit | Drawback |
|------------|---------|----------|
| 5 seconds | Near-real-time freshness | High cache miss rate (more DB load) |
| 30 seconds | **Balanced** — Good hit rate, reasonable freshness | **Optimal for this use case** |
| 5 minutes | Maximum cache hit rate | Stale data for critical alerts |

For IoT monitoring dashboards:
- Most users don't need sub-second freshness
- 30 seconds is acceptable for environmental monitoring
- Dramatically reduces database load under concurrent access

### Cache Population Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API Request Received                              │
└────────────────────────────────────┬────────────────────────────────────────┘
                                      │
                                      ▼
                         ┌─────────────────────────────┐
                         │  Check Redis Cache          │
                         │  Key: sensor:{id}:readings  │
                         └─────────────┬───────────────┘
                                       │
                         ┌─────────────┴───────────────┐
                         │ Cache Hit?                  │
                         └─────────────┬───────────────┘
                                       │
                    ┌──────────────────┴──────────────────┐
                    │ YES                                 │ NO
                    ▼                                     ▼
         ┌──────────────────────┐           ┌──────────────────────────────┐
         │ Return Cached Data   │           │ Query PostgreSQL             │
         │ (~5-15ms)            │           │ (~200-400ms)                 │
         └──────────────────────┘           └──────────────┬───────────────┘
                                                     │
                                                     ▼
                                        ┌──────────────────────────────┐
                                        │ Populate Redis Cache         │
                                        │ Set with 30s TTL             │
                                        └──────────────┬───────────────┘
                                                       │
                                                       ▼
                                        ┌──────────────────────────────┐
                                        │ Return Data to Client        │
                                        └──────────────────────────────┘
```

### Cache Invalidation Approach

**Strategy: Time-based TTL only**

No active invalidation is needed because:
1. Sensor data is append-only — new readings don't invalidate old readings
2. Stale data is simply "slightly older" data — not incorrect data
3. TTL expiration naturally refreshes the cache

**Alternative considered but rejected:** Write-through cache on new readings
- **Rejected because:** High write volume (10k sensors × 1/min = 14.4M/day)
- **Better approach:** Let cache expire naturally; fresh data fetched on next query

### Cold vs Warm Cache Performance

| State | Expected Latency | Reason |
|-------|------------------|--------|
| Cold (cache miss) | 200-400ms | Direct DB query; B-tree index scan |
| Warm (cache hit) | 5-15ms | In-memory Redis retrieval |
| Cold (device not in DB) | ~50ms | Fast path after index lookup returns empty |

At 50M rows with proper indexing, even cold queries should meet the ≤500ms target.

---

## Connection Pooling

### pgx Connection Pool Configuration

```go
poolConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
poolConfig.MaxConns = 50           // Maximum connections
poolConfig.MinConns = 10           // Minimum idle connections
poolConfig.MaxConnLifetime = 1 * time.Hour
poolConfig.MaxConnIdleTime = 10 * time.Minute
poolConfig.HealthCheckPeriod = 30 * time.Second
```

> **Note:** The actual implementation uses higher connection limits (50 max, 10 min) than originally documented (25 max, 5 min) to support higher concurrency. The idle timeout is also shorter (10 min vs 30 min) for more aggressive connection cleanup.

### Connection Pool Sizing

**Formula:** `connections = (cores × 2) + effective_spindle_count`

For typical deployment (4 cores, SSD):
- **Recommended:** 10-25 connections
- **Our setting:** 25 max, 5 min

**Why not more connections?**

PostgreSQL process-per-connection model means:
- Each connection consumes memory (~2-10 MB depending on workload)
- Too many connections cause context switching overhead
- Connection contention increases beyond CPU core count

**Connection pool benefits:**
1. **Reduced connection overhead** — Reusing existing connections
2. **Prepared statement caching** — Query planning cached per connection
3. **Controlled concurrency** — Prevents database overload

### Prepared Statements

pgx automatically uses prepared statements for repeated queries:

```go
-- Prepared on first use, cached for connection lifetime
SELECT * FROM sensor_readings
WHERE device_id = $1
ORDER BY timestamp DESC
LIMIT $2;
```

**Benefits:**
- Query planning done once, executed many times
- Parameter binding prevents SQL injection
- Binary protocol for faster data transfer

---

## Partitioning Strategy

### Declarative Partitioning (Optional for >100M scale)

For 50M rows, partitioning is optional. For 100M+ rows, consider:

```sql
-- Create partitioned table
CREATE TABLE sensor_readings (
    id              BIGSERIAL,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(30)     NOT NULL,
    value           NUMERIC(15,6)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL,
    metadata        JSONB
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions
CREATE TABLE sensor_readings_2024_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE sensor_readings_2024_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
```

### Partitioning Benefits

| Benefit | Impact |
|---------|--------|
| **Query pruning** | PostgreSQL skips irrelevant partitions during query planning |
| **Faster deletes** | Drop old partitions instead of DELETE (no VACUUM needed) |
| **Parallel scans** | Each partition can be scanned independently |
| **Smaller indexes** | Indexes are per-partition, improving cache efficiency |

### When to Enable Partitioning

| Row Count | Partitioning Recommendation |
|-----------|----------------------------|
| < 50M | Not necessary; current indexes suffice |
| 50M-100M | Optional; consider if data retention is needed |
| > 100M | Recommended for optimal performance |

**Key point:** The schema is designed to be partition-ready. Adding partitioning later requires:
1. Creating a new partitioned table
2. Migrating data (can be done online)
3. Switching application to new table

No schema redesign is required.

---

## Infrastructure Components

### Nginx Reverse Proxy

```nginx
upstream api_backend {
    least_conn;
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    # SSL termination
    ssl_certificate /etc/ssl/certs/api.crt;
    ssl_certificate_key /etc/ssl/private/api.key;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
    limit_req zone=api_limit burst=20 nodelay;

    # Proxy settings
    location /api/ {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Why include Nginx?**

1. **SSL termination** — Offloads TLS from Go application
2. **Rate limiting** — Protects against abusive clients
3. **Connection pooling** — Fewer connections to Go app
4. **Static file serving** — Efficient for API documentation
5. **HTTP/2 support** — Better multiplexing for concurrent requests

### Docker Compose Configuration

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: sensor_db
      POSTGRES_USER: sensor_user
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "5432:5432"
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
      -c max_worker_processes=8
      -c max_parallel_workers_per_gather=2
      -c max_parallel_workers=8
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=100

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru

  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://sensor_user:${DB_PASSWORD}@postgres:5432/sensor_db
      REDIS_URL: redis://redis:6379
    depends_on:
      - postgres
      - redis

  nginx:
    image: nginx:alpine
    ports:
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/ssl:ro
    depends_on:
      - api

volumes:
  postgres_data:
```

> **Note:** The PostgreSQL command above includes 14 performance-tuned parameters for high-throughput time-series workloads on an 8GB RAM system. For detailed explanation of each parameter and why these values were chosen, see [high-throughput-guide/01-postgresql-setup.md](../high-throughput-guide/01-postgresql-setup.md#parameter-explanations).


---

## Generalizability

### Schema-Agnostic Design Principles

This architecture's core design decisions apply regardless of specific table structure:

#### 1. Composite Index Pattern

For any table with a repeating identifier and timestamp:

```sql
-- Pattern: (identifier, timestamp DESC)
CREATE INDEX idx_table_identifier_ts
    ON any_table (identifier_column, timestamp_column DESC);

-- Covering index variant
CREATE INDEX idx_table_identifier_covering
    ON any_table (identifier_column, timestamp_column DESC)
    INCLUDE (frequently_accessed_column1, frequently_accessed_column2);
```

**Applies to:**
- User activity logs (`user_id`, `created_at`)
- Transaction histories (`account_id`, `transaction_time`)
- Error tracking (`session_id`, `error_timestamp`)
- Gaming events (`player_id`, `event_time`)

#### 2. BRIN for Append-Only Data

For any time-series or append-only data:

```sql
CREATE INDEX idx_table_timestamp_brin
    ON any_table
    USING BRIN (timestamp_column);
```

**Characteristics that make BRIN suitable:**
- Data is appended in roughly chronological order
- Time-range queries are common
- Index size matters more than single-row lookup speed

#### 3. API-First Caching

Cache key pattern generalizes to any identifier-based lookup:

```
{entity}:{identifier}:{query_type}[:{filters}]

Examples:
- sensor:sensor-001:readings:10
- user:user-123:activities:50
- account:acct-456:transactions:20
```

### What Changes, What Stays the Same

| Changes with Schema | Stays the Same |
|---------------------|----------------|
| Table name | Composite index pattern |
| Column names | BRIN index for time-series |
| Number of columns | Cache key structure |
| Business domain | API-first architecture |
| | Connection pooling strategy |
| | Performance tuning methodology |

### Hard Limits

Regardless of schema, these limits are unavoidable:

| Limit | Approximate Floor |
|-------|-------------------|
| Network latency (local) | 0.5-2ms |
| Network latency (remote) | 50-100ms |
| Disk sequential read | 100-500 MB/s |
| PostgreSQL tuple lookup | ~0.01-0.1ms (in cache) |
| TCP connection setup | ~10-50ms |

**Implication:** The ≤500ms target requires:
- Minimizing round-trips (batch queries when possible)
- Maximizing cache hits (Redis + PostgreSQL buffer pool)
- Optimizing indexes (index-only scans)

---

## Related Documentation

- [stack.md](stack.md) — Complete technology stack with justifications
- [api-spec.md](api-spec.md) — API contract and endpoint specifications
- [testing.md](testing.md) — Performance testing methodology
