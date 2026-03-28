# PostgreSQL Setup for High-Throughput Workloads

This guide covers PostgreSQL configuration and schema design for achieving ≤500ms median latency on exact-ID queries at scale.

## Overview

Proper PostgreSQL configuration is critical for high-throughput systems. This document covers:

1. **PostgreSQL configuration parameters** for high-throughput workloads
2. **Table design patterns** optimized for exact-ID queries
3. **Indexing strategies** including BRIN, composite, and covering indexes
4. **Connection pooling** with PgBouncer
5. **Materialized views** for analytics queries

## PostgreSQL Configuration

### Key Parameters for High-Throughput

Add these parameters to your `postgresql.conf` or pass them as command-line arguments:

```bash
# Memory Configuration
shared_buffers = 2GB                 # 25% of available RAM
effective_cache_size = 6GB           # 50-75% of RAM
work_mem = 16MB                      # Per-operation memory
maintenance_work_mem = 1GB           # For maintenance operations

# Connection Configuration
max_connections = 200                # Total allowed connections

# Write-Ahead Log
wal_buffers = 16MB
checkpoint_completion_target = 0.9   # Reduce checkpoint spikes

# Query Planner
random_page_cost = 1.1               # For SSD storage
effective_io_concurrency = 200       # For SSD storage

# Parallelism
max_worker_processes = 8
max_parallel_workers_per_gather = 2
max_parallel_workers = 8

# Background Writer
bgwriter_delay = 200ms
bgwriter_lru_maxpages = 100
```

### Docker Compose Configuration

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
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
    environment:
      POSTGRES_DB: app_db
      POSTGRES_USER: app_user
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
```

### Parameter Explanations

| Parameter | Value | Why |
|-----------|-------|-----|
| `shared_buffers` | 2GB | Cached data pages (25% of RAM on 8GB system) |
| `effective_cache_size` | 6GB | Query planner's estimate of available cache |
| `work_mem` | 16MB | Memory for sorting/hashing per operation |
| `random_page_cost` | 1.1 | Optimizes for SSD (default is 4.0 for HDD) |
| `effective_io_concurrency` | 200 | Parallel I/O operations for SSD |
| `max_connections` | 200 | Sufficient for connection pooling |
| `checkpoint_completion_target` | 0.9 | Spreads checkpoint I/O over time |

## Table Design for Exact-ID Queries

### Core Schema Pattern

```sql
CREATE TABLE entity_readings (
    id              BIGSERIAL       PRIMARY KEY,
    entity_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(20)     NOT NULL,
    value           DECIMAL(10,2)   NOT NULL,
    metadata        JSONB
);
```

### Design Principles

#### 1. Primary Key: BIGSERIAL (Not UUID)

```sql
-- GOOD: BIGSERIAL for sequential IDs
id BIGSERIAL PRIMARY KEY

-- AVOID: UUID for time-series data
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

**Why**: BIGSERIAL provides:
- Sequential storage (better for indexes)
- Smaller index size (8 bytes vs 16 bytes)
- Faster inserts and scans
- Natural ordering for time-series data

#### 2. Identifier Column: VARCHAR(50) with Index

```sql
entity_id VARCHAR(50) NOT NULL
```

**Why VARCHAR(50)**:
- Sufficient for most identifiers (user IDs, device IDs, etc.)
- Small enough for efficient indexing
- Large enough for composite keys (e.g., "user-12345-type")

#### 3. Timestamp: TIMESTAMPTZ

```sql
timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

**Why TIMESTAMPTZ**:
- Timezone-aware (converts to client timezone)
- Consistent ordering across timezones
- Index-friendly for time-range queries

#### 4. CHECK Constraints for Validation

```sql
reading_type VARCHAR(20) NOT NULL
CHECK (reading_type IN ('type1', 'type2', 'type3'))
```

**Why**: Data integrity at database level prevents invalid data.

#### 5. Denormalization for Read-Heavy Workloads

```sql
-- Single table with all columns (faster reads)
entity_readings (id, entity_id, type, value, metadata)

-- Instead of normalized schema (slower reads)
entities (id, name)
readings (id, entity_id, type_id, value)
types (id, name)
```

**Trade-off**: Faster reads at cost of storage space and write complexity.

## Indexing Strategy

### Index Selection Decision Tree

```
Is data append-only time-series?
├─ Yes → Use BRIN index (99% smaller)
└─ No
   ├─ Query needs multiple columns?
   │  ├─ Yes → Use composite index (column order matters)
   │  └─ No → Use single-column B-tree
   └─ Query can be satisfied from index alone?
      ├─ Yes → Use covering index with INCLUDE
      └─ No → Use regular B-tree
```

### 1. BRIN Index (Block Range INdex)

**Best for**: Append-only time-series data larger than 100MB

```sql
CREATE INDEX idx_readings_timestamp_brin
ON entity_readings USING BRIN (timestamp);
```

**Characteristics**:
- **Size**: 99% smaller than B-tree (20MB vs 2GB for 50M rows)
- **Performance**: Excellent for time-range queries
- **Trade-off**: Slower for exact timestamp lookups

**When to use**:
- Table size > 100MB
- Data is append-only (inserts, not updates)
- Queries filter by time range
- Sequential correlation in timestamp column

**Example query pattern**:
```sql
-- Excellent for BRIN
SELECT * FROM entity_readings
WHERE timestamp >= NOW() - INTERVAL '7 days';

-- Less ideal for BRIN
SELECT * FROM entity_readings
WHERE timestamp = '2024-01-15 10:30:00';
```

### 2. Composite Index

**Best for**: Queries filtering by multiple columns

```sql
CREATE INDEX idx_readings_entity_type_timestamp
ON entity_readings (entity_id, reading_type, timestamp DESC);
```

**Column Order Matters**:

1. **Equality columns first** (entity_id)
2. **Range columns last** (timestamp DESC)

```sql
-- Uses composite index efficiently
SELECT * FROM entity_readings
WHERE entity_id = 'user-123'
AND reading_type = 'temperature'
ORDER BY timestamp DESC
LIMIT 100;

-- Also uses index (prefix match)
SELECT * FROM entity_readings
WHERE entity_id = 'user-123'
ORDER BY timestamp DESC
LIMIT 100;

-- Does NOT use index efficiently (reading_type not first)
SELECT * FROM entity_readings
WHERE reading_type = 'temperature';
```

**When to use**:
- Queries always filter by entity_id first
- Additional columns (type, category) are commonly filtered
- Time-based ordering is always DESC

### 3. Covering Index (Index-Only Scan)

**Best for**: Queries that can be satisfied without accessing the table

```sql
CREATE INDEX idx_readings_entity_covering
ON entity_readings (entity_id, timestamp DESC)
INCLUDE (reading_type, value, metadata);
```

**How it works**:
- Index contains search columns (entity_id, timestamp)
- Index INCLUDEs non-searched columns (reading_type, value, metadata)
- Query satisfied entirely from index (no heap access)

**Performance improvement**: 2-5x faster than regular index

```sql
-- This query uses ONLY the index (index-only scan)
SELECT entity_id, timestamp, reading_type, value, metadata
FROM entity_readings
WHERE entity_id = 'user-123'
ORDER BY timestamp DESC
LIMIT 100;

-- EXPLAIN ANALYZE output shows:
-- Index Only Scan using idx_readings_entity_covering
```

**When to use**:
- Queries always return same set of columns
- Frequently accessed entities (hot keys)
- Wide table with many columns (index reduces I/O)

**Trade-off**:
- Larger index size
- Slower INSERT/UPDATE (index maintenance)

### Index Size Comparison (50M rows)

| Index Type | Size | Notes |
|------------|------|-------|
| B-tree on timestamp | ~2.5GB | Baseline |
| BRIN on timestamp | ~25MB | 99% smaller |
| Composite (3 columns) | ~4GB | Standard size |
| Covering (2 + 3 columns) | ~5GB | 25% larger than composite |

### Creating Indexes Concurrently

**For production databases with active traffic**:

```sql
-- Create index without blocking writes
CREATE INDEX CONCURRENTLY idx_readings_entity_covering
ON entity_readings (entity_id, timestamp DESC)
INCLUDE (reading_type, value, metadata);
```

**Important**: CONCURRENTLY takes longer but doesn't lock the table.

## Connection Pooling with PgBouncer

### Why Connection Pooling Matters

Creating a new PostgreSQL connection has overhead:
- TCP handshake: ~5-10ms
- Authentication: ~5-15ms
- Backend process startup: ~10-25ms

**Total**: 20-50ms per new connection

Connection pooling reuses existing connections, eliminating this overhead.

### PgBouncer Configuration

#### Install PgBouncer

```bash
# Alpine/Debian
apk add pgbouncer

# Ubuntu/Debian
apt-get install pgbouncer
```

#### Configuration (pgbouncer.ini)

```ini
[databases]
app_db = host=localhost port=5432 dbname=app_db

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

# Pooling mode: transaction (recommended for PostgreSQL)
pool_mode = transaction

# Connection pool sizes
max_client_conn = 1000
default_pool_size = 50
min_pool_size = 10

# Connection lifetime
server_lifetime = 3600
server_idle_timeout = 600

# Logging
log_connections = 1
log_disconnections = 1
log_pooler_errors = 1
```

#### User List (userlist.txt)

```
"app_user" "md5hashed_password"
```

Generate hash:
```bash
echo -n "apppassword" | md5sum
```

#### Pool Mode Selection

| Mode | Description | Use Case |
|------|-------------|----------|
| **session** | One connection per client | Low concurrency, long transactions |
| **transaction** | Return connection after each transaction | **Recommended** for most workloads |
| **statement** | Return connection after each statement | Rarely used, autocommit apps |

**Use transaction mode** for high-throughput web applications.

### Connection Pool Size Calculation

```
Total connections = (number of app instances) × (connections per instance)

Recommended connections per instance:
- Low traffic (< 100 RPS): 10 connections
- Medium traffic (100-1000 RPS): 25 connections
- High traffic (> 1000 RPS): 50 connections

Formula: connections = (CPU cores × 2) + effective_spindle_count
```

Example (4 CPU cores, SSD):
```
connections = (4 × 2) + 1 = 9 → round up to 10
```

## Materialized Views for Analytics

### What are Materialized Views?

Materialized views pre-compute and store query results, refreshed periodically.

### Use Case: Statistics Queries

```sql
-- Slow on base table (50M rows)
SELECT entity_id, COUNT(*), AVG(value)
FROM entity_readings
GROUP BY entity_id;
-- Execution time: 5-10 seconds
```

```sql
-- Fast from materialized view
SELECT * FROM mv_entity_stats;
-- Execution time: 5-10 milliseconds
```

**Performance improvement**: 100-1000x faster

### Creating Materialized Views

```sql
CREATE MATERIALIZED VIEW mv_entity_stats AS
SELECT
    entity_id,
    COUNT(*) as reading_count,
    AVG(value) as avg_value,
    MIN(value) as min_value,
    MAX(value) as max_value,
    STDDEV(value) as stddev_value
FROM entity_readings
GROUP BY entity_id;

-- Create index on materialized view
CREATE INDEX idx_mv_stats_entity_id
ON mv_entity_stats (entity_id);
```

### Refresh Strategies

#### Manual Refresh

```sql
REFRESH MATERIALIZED VIEW mv_entity_stats;
```

**Drawback**: Locks the materialized view during refresh.

#### Concurrent Refresh (PostgreSQL 9.4+)

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_entity_stats;
```

**Requirements**:
- Must have at least one UNIQUE index on the materialized view
- Allows queries during refresh

#### Automated Refresh

```sql
-- Create refresh function
CREATE OR REPLACE FUNCTION refresh_entity_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_entity_stats;
END;
$$ LANGUAGE plpgsql;

-- Schedule with pg_cron (extension)
SELECT cron.schedule('refresh-stats', '*/15 * * * *', 'SELECT refresh_entity_stats()');
```

**Refresh intervals**:
- Real-time: Every 1-5 minutes
- Near real-time: Every 15-30 minutes
- Hourly/daily: For batch analytics

### When to Use Materialized Views

| Use Case | Recommended | Refresh Interval |
|----------|-------------|------------------|
| Dashboard statistics | Yes | 5-15 minutes |
| Real-time monitoring | Maybe | 1-5 minutes |
| Audit reports | Yes | Hourly/daily |
| Transactional queries | No | N/A (use base table) |

## Schema Migration Strategy

### Version Your Schema Changes

```
scripts/schema/migrations/
├── 001_init_schema.sql
├── 002_advanced_indexes.sql
├── 003_materialized_views.sql
└── 004_covering_index.sql
```

### Migration Script Template

```sql
-- Migration 00X: Description
--
-- What this migration does and why.
--
-- Run with: ./scripts/run_migrations.sh
--
-- IMPORTANT: Review before running on production.

-- =============================================================================
-- Step 1: Add new column
-- =============================================================================

ALTER TABLE entity_readings
ADD COLUMN IF NOT EXISTS new_column VARCHAR(100);

-- =============================================================================
-- Step 2: Create index
-- =============================================================================

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_readings_new_column
ON entity_readings (new_column);

-- =============================================================================
-- Verification
-- =============================================================================

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'entity_readings'
        AND column_name = 'new_column'
    ) THEN
        RAISE NOTICE 'Migration 00X completed successfully';
    ELSE
        RAISE EXCEPTION 'Migration 00X failed';
    END IF;
END $$;
```

## Monitoring and Maintenance

### Key Metrics to Monitor

```sql
-- Index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
ORDER BY idx_scan ASC;

-- Table size
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Connection stats
SELECT
    count(*) as total_connections,
    count(*) FILTER (WHERE state = 'active') as active,
    count(*) FILTER (WHERE state = 'idle') as idle
FROM pg_stat_activity;
```

### Regular Maintenance

```sql
-- Analyze tables (update statistics)
ANALYZE entity_readings;

-- Vacuum (reclaim space)
VACUUM ANALYZE entity_readings;

-- Reindex (rebuild indexes)
REINDEX TABLE CONCURRENTLY entity_readings;
```

## Best Practices Summary

1. **Configure PostgreSQL for your hardware**: Adjust `shared_buffers` and `work_mem` based on available RAM
2. **Use BIGSERIAL primary keys**: Not UUIDs for time-series data
3. **Choose the right index type**: BRIN for time-series, composite for multi-column, covering for index-only scans
4. **Implement connection pooling**: Use PgBouncer in transaction mode
5. **Pre-compute aggregations**: Use materialized views for statistics
6. **Monitor index usage**: Remove unused indexes (they slow down writes)
7. **Version your migrations**: Never make manual schema changes in production
8. **Test with production data volume**: Performance at 1K rows != performance at 50M rows

## Next Steps

- [Golang API Setup](./02-golang-api-setup.md) - Configure the application layer
- [Performance Targets](./03-performance-targets.md) - Define and measure performance goals
- [General Setup Guide](./04-general-setup-guide.md) - Step-by-step implementation guide
