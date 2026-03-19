# Phase B1: Advanced Indexing Strategy

## Overview

This phase implements advanced indexing strategies optimized for production workloads with time-series IoT sensor data. The indexes are designed based on common query patterns observed in IoT systems and leverage PostgreSQL's advanced indexing features including partial indexes, BRIN indexes, and expression indexes.

**Business Context:** IoT sensor data typically exhibits the following characteristics:
- **Temporal locality**: Most queries access recent data (last 30 days)
- **Device popularity skew (Zipf distribution)**: A small percentage of devices account for a large percentage of queries
- **Aggregation patterns**: Daily/hourly aggregations are common for dashboards and analytics

**Performance Goals:**
- Reduce query latency for recent data by 30-40%
- Reduce query latency for hot devices by 20-30%
- Reduce query latency for daily aggregations by 50-60%
- Minimize storage overhead through partial and expression indexes

## Implementation

### Files Created
- `scripts/schema/migrations/002_advanced_indexes.sql`: Migration script with CONCURRENTLY index creation
- `scripts/validate_indexes.sh`: Index validation and performance verification script
- `docs/implementation/phase-b1-advanced-indexing.md`: This documentation

### Files Modified
- None (this migration only adds new indexes)

### Database Context

**Table Schema:**
```sql
CREATE TABLE sensor_readings (
    id              BIGSERIAL PRIMARY KEY,
    device_id       VARCHAR(50) NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL,
    reading_type    VARCHAR(50) NOT NULL,
    value           DOUBLE PRECISION NOT NULL,
    unit            VARCHAR(20),
    metadata        JSONB
);
```

**Existing Indexes (from Phase 0):**
- `idx_sensor_readings_device_ts`: B-tree on (device_id, timestamp DESC)
- `idx_sensor_readings_ts`: B-tree on (timestamp DESC)
- `idx_sensor_readings_device_type_ts`: B-tree on (device_id, reading_type, timestamp DESC)

## Index Strategy

### Index 1: Partial BRIN Index for Recent Data

**Rationale:**
- BRIN (Block Range INdex) indexes are extremely space-efficient for time-series data
- They store summary information for ranges of blocks rather than individual rows
- For time-ordered data, BRIN indexes can be 100x smaller than B-tree indexes
- By limiting to recent 30 days, we optimize for the most common query pattern

**Implementation:**
```sql
CREATE INDEX CONCURRENTLY idx_sensor_readings_recent_brin
ON sensor_readings USING BRIN (timestamp)
WHERE timestamp >= NOW() - INTERVAL '30 days';
```

**Expected Impact:**
- **Storage**: ~1-2 MB for 30 days of data vs ~200 MB for full B-tree
- **Query Performance**: 30-40% faster for queries filtering by recent timestamp
- **Maintenance**: Automatically excludes old data, reducing index maintenance overhead

**Use Case:** Queries like:
```sql
SELECT * FROM sensor_readings
WHERE device_id = 'device-123'
  AND timestamp >= NOW() - INTERVAL '7 days'
ORDER BY timestamp DESC LIMIT 100;
```

**Validation Query:**
```sql
-- Check BRIN index size
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size,
    pg_size_pretty(pg_relation_size(indexrelid) /
        (SELECT count(*) FROM sensor_readings WHERE timestamp >= NOW() - INTERVAL '30 days')
    ) as bytes_per_row
FROM pg_indexes
WHERE indexname = 'idx_sensor_readings_recent_brin';
```

### Index 2: Partial Covering Index for Hot Devices

**Rationale:**
- IoT data follows Zipf distribution: top 20% of devices receive ~80% of queries
- Covering indexes include all columns needed for index-only scans (no table access)
- By limiting to top 100 devices, we optimize for the most queried devices
- Partial index reduces storage overhead while maximizing performance benefit

**Implementation:**
```sql
CREATE INDEX CONCURRENTLY idx_sensor_readings_hot_device_covering
ON sensor_readings (device_id, timestamp DESC, reading_type, value, unit)
WHERE device_id IN (
  SELECT device_id
  FROM (
    SELECT device_id, count(*) as cnt
    FROM sensor_readings
    GROUP BY device_id
    ORDER BY cnt DESC
    LIMIT 100
  ) t
);
```

**Expected Impact:**
- **Storage**: ~50-100 MB (only for top 100 devices) vs ~2 GB for full covering index
- **Query Performance**: 20-30% faster for hot device queries
- **Index-Only Scans**: Eliminates table access for most queries

**Use Case:** Queries like:
```sql
SELECT device_id, timestamp, reading_type, value, unit
FROM sensor_readings
WHERE device_id = 'device-1'  -- One of the hot devices
ORDER BY timestamp DESC
LIMIT 100;
```

**Validation Query:**
```sql
-- Check if index-only scans are being used
EXPLAIN (ANALYZE, BUFFERS)
SELECT device_id, timestamp, reading_type, value, unit
FROM sensor_readings
WHERE device_id = 'device-1'
ORDER BY timestamp DESC
LIMIT 100;

-- Look for "Index Only Scan" in the output
```

### Index 3: Expression Index for Daily Aggregations

**Rationale:**
- Dashboard queries frequently aggregate data by day
- Using `date_trunc('day', timestamp)` in WHERE clauses requires full table scans
- Expression indexes pre-compute the function result, enabling efficient filtering
- This index supports both filtering and GROUP BY operations

**Implementation:**
```sql
CREATE INDEX CONCURRENTLY idx_sensor_readings_date_trunc
ON sensor_readings (date_trunc('day', timestamp), device_id, reading_type);
```

**Expected Impact:**
- **Storage**: ~2-3 GB (similar to B-tree on timestamp)
- **Query Performance**: 50-60% faster for daily aggregation queries
- **Optimization**: Enables efficient partition pruning and parallel aggregation

**Use Case:** Queries like:
```sql
SELECT
    date_trunc('day', timestamp) as day,
    device_id,
    reading_type,
    avg(value) as avg_value,
    count(*) as reading_count
FROM sensor_readings
WHERE date_trunc('day', timestamp) >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY date_trunc('day', timestamp), device_id, reading_type
ORDER BY day DESC, device_id;
```

**Validation Query:**
```sql
-- Check if the expression index is used
EXPLAIN (ANALYZE, BUFFERS)
SELECT
    date_trunc('day', timestamp) as day,
    device_id,
    avg(value) as avg_value
FROM sensor_readings
WHERE date_trunc('day', timestamp) >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY date_trunc('day', timestamp), device_id;

-- Look for "Index Scan using idx_sensor_readings_date_trunc"
```

## Configuration

### Environment Variables
None required. This migration uses only database configuration.

### PostgreSQL Requirements
- PostgreSQL 14+ (for BRIN index with WHERE clause support)
- Sufficient disk space for index creation (~5 GB temporary space during creation)
- CONCURRENTLY option prevents table locks during creation

### Migration Parameters
- **Creation Time**: 2-5 minutes on 50M rows (using CONCURRENTLY)
- **Downtime**: Zero (CONCURRENTLY option)
- **Temporary Space**: ~5 GB during creation

## Performance Impact

### Expected Improvements

| Query Pattern | Before | After | Improvement |
|--------------|--------|-------|-------------|
| Recent data (7 days) | ~200ms | ~140ms | 30% faster |
| Hot device query | ~150ms | ~105ms | 30% faster |
| Daily aggregation | ~500ms | ~200ms | 60% faster |
| Full table scan | Unchanged | Unchanged | N/A |

### Storage Impact

| Index | Type | Size | Maintenance |
|-------|------|------|-------------|
| `idx_sensor_readings_recent_brin` | Partial BRIN | ~1-2 MB | Automatic (30-day window) |
| `idx_sensor_readings_hot_device_covering` | Partial B-tree | ~50-100 MB | Static (top 100 devices) |
| `idx_sensor_readings_date_trunc` | Expression B-tree | ~2-3 GB | Standard B-tree maintenance |

**Total Additional Storage**: ~2-3 GB (acceptable for 50M row table)

### Write Performance Impact
- **INSERT overhead**: ~5-10% slower (3 additional indexes to maintain)
- **UPDATE overhead**: Minimal (timestamp updates rare in IoT data)
- **DELETE overhead**: Minimal (soft deletes used)

## Testing Steps

### 1. Pre-Migration Validation

```bash
# Check current index usage
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT
    schemaname,
    indexname,
    idx_scan as scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
WHERE tablename = 'sensor_readings'
ORDER BY idx_scan DESC;
"

# Check current table size
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT
    pg_size_pretty(pg_total_relation_size('sensor_readings')) as total_size,
    pg_size_pretty(pg_relation_size('sensor_readings')) as table_size,
    pg_size_pretty(pg_total_relation_size('sensor_readings') - pg_relation_size('sensor_readings')) as indexes_size;
"
```

### 2. Run Migration

```bash
# Apply migration (wait for data generation to complete first!)
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/002_advanced_indexes.sql

# Monitor progress in another terminal
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT pid, query, wait_event_type, wait_event
FROM pg_stat_activity
WHERE query LIKE '%CREATE INDEX%';
"
```

### 3. Post-Migration Validation

```bash
# Run validation script
./scripts/validate_indexes.sh

# Check all indexes exist
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT indexname, pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_indexes
WHERE tablename = 'sensor_readings'
ORDER BY pg_relation_size(indexrelid) DESC;
"

# Verify BRIN index summary
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT
    brinstartblock,
    brinendblock,
    heapblkscanned,
    heapblksprocessed,
    pagescanned,
    pageprocessed
FROM brin_page_stats(get_raw_page('idx_sensor_readings_recent_brin', 0));
"
```

### 4. Query Performance Testing

```bash
# Test recent data query
time curl "http://localhost:8080/api/v1/sensor-readings?device_id=device-1&limit=100"

# Test daily aggregation (via stats endpoint)
time curl "http://localhost:8080/api/v1/stats?device_id=device-1&days=7"

# Compare EXPLAIN ANALYZE output
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM sensor_readings
WHERE device_id = 'device-1'
  AND timestamp >= NOW() - INTERVAL '7 days'
ORDER BY timestamp DESC
LIMIT 100;
"
```

### 5. Load Testing Comparison

```bash
# Run baseline load test
./scripts/test-runner.sh

# Compare results against baseline from Phase A3
# Look for improvements in p50 and p95 latencies
```

## Rollback Plan

If performance degrades or issues arise:

### Immediate Rollback

```sql
-- Drop all new indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_recent_brin;
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_hot_device_covering;
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_date_trunc;
```

### Partial Rollback (Selective)

```sql
-- Drop only problematic indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_recent_brin;  -- If too slow
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_hot_device_covering;  -- If not beneficial
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_date_trunc;  -- If aggregations rare
```

### Verification After Rollback

```bash
# Verify indexes dropped
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT indexname FROM pg_indexes
WHERE tablename = 'sensor_readings'
  AND indexname LIKE 'idx_sensor_readings_%';
"

# Run load test to confirm baseline performance restored
./scripts/test-runner.sh
```

## Monitoring and Maintenance

### Ongoing Monitoring

```sql
-- Monitor index usage weekly
SELECT
    schemaname,
    indexname,
    idx_scan as scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_stat_user_indexes
WHERE tablename = 'sensor_readings'
ORDER BY idx_scan DESC;

-- Check for index bloat
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size,
    pg_size_pretty(bloat_size) as bloat
FROM (
    SELECT
        schemaname,
        tablename,
        indexname,
        indexrelid,
        pg_relation_size(indexrelid) -
        pg_stat_get_blocks_fetched(indexrelid) *
        (SELECT current_setting('block_size')::int / 1024) as bloat_size
    FROM pg_stat_user_indexes
    WHERE tablename = 'sensor_readings'
) sub
WHERE bloat_size > 0
ORDER BY bloat_size DESC;
```

### Maintenance Tasks

#### Monthly: Update Hot Device List

```sql
-- Rebuild hot device index with updated device list
DROP INDEX CONCURRENTLY IF EXISTS idx_sensor_readings_hot_device_covering;

CREATE INDEX CONCURRENTLY idx_sensor_readings_hot_device_covering
ON sensor_readings (device_id, timestamp DESC, reading_type, value, unit)
WHERE device_id IN (
  SELECT device_id
  FROM (
    SELECT device_id, count(*) as cnt
    FROM sensor_readings
    GROUP BY device_id
    ORDER BY cnt DESC
    LIMIT 100
  ) t
);
```

#### Quarterly: Reindex if Bloat Detected

```sql
-- Reindex individual indexes
REINDEX INDEX CONCURRENTLY idx_sensor_readings_date_trunc;

-- Or reindex entire table (maintenance window required)
-- REINDEX TABLE CONCURRENTLY sensor_readings;
```

## Troubleshooting

### Issue 1: BRIN Index Not Used

**Symptoms**: Queries still use seq scan or other indexes

**Diagnosis**:
```sql
EXPLAIN (ANALYZE) SELECT * FROM sensor_readings
WHERE timestamp >= NOW() - INTERVAL '7 days';
```

**Solution**:
- Check BRIN index parameters: `SELECT * FROM brin_page_stats(get_raw_page('idx_sensor_readings_recent_brin', 0));`
- Increase `pages_per_range` if index is too large: `CREATE INDEX ... USING BRIN (timestamp) WITH (pages_per_range = 128)`

### Issue 2: Hot Device Index Not Used

**Symptoms**: Queries for known hot devices don't use covering index

**Diagnosis**:
```sql
-- Check if device is in hot list
SELECT device_id FROM (
    SELECT device_id, count(*) as cnt
    FROM sensor_readings
    GROUP BY device_id
    ORDER BY cnt DESC
    LIMIT 100
) t
WHERE device_id = 'device-1';
```

**Solution**:
- Device may not be in top 100. Update hot device list or expand limit.
- Check index definition: `SELECT pg_get_indexdef('idx_sensor_readings_hot_device_covering'::regclass);`

### Issue 3: Expression Index Not Used

**Symptoms**: Daily aggregation queries still slow

**Diagnosis**:
```sql
-- Check if query matches index exactly
EXPLAIN (ANALYZE) SELECT
    date_trunc('day', timestamp) as day,
    avg(value)
FROM sensor_readings
WHERE date_trunc('day', timestamp) >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY 1;
```

**Solution**:
- Ensure exact function match: `date_trunc('day', timestamp)` not `date_trunc('day', timestamp) + INTERVAL '1 day'`
- Check for implicit casts that prevent index usage

## References

### PostgreSQL Documentation
- [BRIN Indexes](https://www.postgresql.org/docs/current/brin.html)
- [Partial Indexes](https://www.postgresql.org/docs/current/indexes-partial.html)
- [Expression Indexes](https://www.postgresql.org/docs/current/indexes-expressional.html)
- [Index-Only Scans](https://www.postgresql.org/docs/current/indexes-index-only-scans.html)

### Best Practices
- PostgreSQL Index Design for Time-Series: https://www.citus.io/blog/when-to-use-brin-index
- IoT Database Design: https://www.timescale.com/blog/understanding-time-series-data-characteristics-for-better-performance/
- Index Maintenance: https://wiki.postgresql.org/wiki/Index_Maintenance

### Industry Standards
- For 50M+ row tables: Always use CONCURRENTLY for index creation
- For time-series data: Prefer BRIN for recent data, B-tree for historical
- For skewed access: Use partial indexes for hot data
- For aggregations: Use expression indexes for common GROUP BY clauses

## Success Criteria

- [ ] All three indexes created successfully
- [ ] No table locks during migration (CONCURRENTLY)
- [ ] BRIN index size < 5 MB
- [ ] Hot device covering index size < 150 MB
- [ ] Expression index used for daily aggregations
- [ ] Recent data queries 30%+ faster
- [ ] Hot device queries 20%+ faster
- [ ] Daily aggregation queries 50%+ faster
- [ ] Index validation script passes all checks
- [ ] Load tests show improvement over baseline

## Changelog

**2026-03-15**: Initial Phase B1 documentation created
- Defined three advanced indexes for production workloads
- Created comprehensive testing and validation procedures
- Documented rollback plans and troubleshooting steps
