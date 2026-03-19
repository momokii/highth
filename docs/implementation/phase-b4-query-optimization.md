# Phase B4: Query Optimization with Materialized Views

## Overview

This phase implements query optimization strategies using PostgreSQL materialized views and advanced query techniques. Materialized views pre-compute and store expensive query results, dramatically improving performance for aggregation and analytics queries.

**Business Context:** IoT sensor data analytics typically involve:
- Aggregating large datasets (AVG, MIN, MAX, COUNT)
- Time-based grouping (hourly, daily, weekly summaries)
- Dashboard queries that run frequently
- Statistical analysis across multiple devices

**Performance Goals:**
- Reduce aggregation query latency by 70-80%
- Enable sub-second response times for dashboard queries
- Minimize database CPU usage for complex aggregations
- Support real-time analytics with periodic refresh

## Implementation

### Files Created
- `scripts/schema/migrations/004_materialized_views.sql`: Materialized view definitions
- `scripts/refresh_materialized_views.sh`: Automated refresh script
- `docs/implementation/phase-b4-query-optimization.md`: This documentation

### Files Modified
- `internal/repository/sensor_repo.go`: Add methods to query materialized views
- `internal/handler/sensor_handler.go`: Add endpoints for pre-aggregated data

## Materialized Views

### View 1: Hourly Device Statistics

**Purpose**: Pre-compute hourly statistics for each device and reading type

**Query Pattern**:
```sql
SELECT
    device_id,
    date_trunc('hour', timestamp) as hour,
    reading_type,
    count(*) as reading_count,
    avg(value) as avg_value,
    min(value) as min_value,
    max(value) as max_value,
    stddev(value) as stddev_value
FROM sensor_readings
GROUP BY device_id, date_trunc('hour', timestamp), reading_type;
```

**Benefits**:
- **Speed**: 100x faster than raw table scan (100ms → 1ms)
- **Storage**: ~500 MB for 50M rows (1% of original data)
- **Maintenance**: Refresh every 15 minutes for near-real-time data

**Use Cases**:
- Dashboard hourly trends
- Device performance monitoring
- Anomaly detection (compare current hour to historical)
- Capacity planning

### View 2: Daily Device Statistics

**Purpose**: Pre-compute daily statistics for each device and reading type

**Query Pattern**:
```sql
SELECT
    device_id,
    date_trunc('day', timestamp) as day,
    reading_type,
    count(*) as reading_count,
    avg(value) as avg_value,
    min(value) as min_value,
    max(value) as max_value,
    percentile_cont(0.5) WITHIN GROUP (ORDER BY value) as median_value,
    percentile_cont(0.95) WITHIN GROUP (ORDER BY value) as p95_value,
    percentile_cont(0.99) WITHIN GROUP (ORDER BY value) as p99_value
FROM sensor_readings
GROUP BY device_id, date_trunc('day', timestamp), reading_type;
```

**Benefits**:
- **Speed**: 200x faster than raw table scan (500ms → 2.5ms)
- **Storage**: ~100 MB for 50M rows (0.2% of original data)
- **Maintenance**: Refresh once per day (batch update)

**Use Cases**:
- Daily performance reports
- Historical trend analysis
- SLA compliance monitoring
- Long-term capacity planning

### View 3: Global Statistics Summary

**Purpose**: Pre-compute system-wide statistics

**Query Pattern**:
```sql
SELECT
    reading_type,
    count(*) as total_readings,
    count(DISTINCT device_id) as active_devices,
    avg(value) as global_avg,
    min(value) as global_min,
    max(value) as global_max,
    min(timestamp) as first_reading,
    max(timestamp) as last_reading
FROM sensor_readings
GROUP BY reading_type;
```

**Benefits**:
- **Speed**: Instant (< 1ms) for system-wide stats
- **Storage**: < 1 KB (one row per reading type)
- **Maintenance**: Refresh every 5 minutes

**Use Cases**:
- System health dashboard
- Overview statistics
- Quick system capacity checks

## Query Optimization Techniques

### 1. Partition Pruning

**Technique**: Use WHERE clauses that enable PostgreSQL to skip irrelevant partitions

**Before**:
```sql
SELECT * FROM sensor_readings
WHERE timestamp >= NOW() - INTERVAL '7 days';
```

**After** (with partitioning):
```sql
SELECT * FROM sensor_readings
WHERE timestamp >= NOW() - INTERVAL '7 days'
  AND timestamp < NOW();
```

**Impact**: 80% faster by scanning only recent partitions

### 2. Index-Only Scans

**Technique**: Design queries to use only indexed columns

**Before**:
```sql
SELECT id, device_id, timestamp, value
FROM sensor_readings
WHERE device_id = 'device-1'
ORDER BY timestamp DESC LIMIT 100;
```

**After** (with covering index):
```sql
SELECT device_id, timestamp, value
FROM sensor_readings
WHERE device_id = 'device-1'
ORDER BY timestamp DESC LIMIT 100;
```

**Impact**: 30% faster by avoiding table access

### 3. Prepared Statements

**Technique**: Use prepared statements for repeated queries

**Implementation**:
```go
const getReadingsQuery = `
    SELECT * FROM sensor_readings
    WHERE device_id = $1
      AND timestamp >= $2
    ORDER BY timestamp DESC
    LIMIT $3
`

stmt, err := r.db.Prepare(ctx, getReadingsQuery)
```

**Impact**: 10-20% faster for repeated queries

### 4. Query Result Caching

**Technique**: Cache expensive query results in Redis

**Implementation**:
```go
cacheKey := fmt.Sprintf("stats:%s:%s", deviceID, timeRange)
if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
    return cached, nil
}
// ... execute query ...
r.cache.Set(ctx, cacheKey, result, 5*time.Minute)
```

**Impact**: 95% cache hit rate = 95% latency reduction

## Configuration

### Materialized View Refresh Strategy

| View | Refresh Interval | Concurrency | Impact |
|------|-----------------|-------------|--------|
| Hourly Stats | Every 15 minutes | CONCURRENTLY | Low impact |
| Daily Stats | Once daily (02:00 AM) | Full | Minimal impact |
| Global Stats | Every 5 minutes | CONCURRENTLY | Minimal impact |

### Environment Variables

```bash
# Enable materialized views
MATERIALIZED_VIEWS_ENABLED=true

# Refresh intervals
MV_HOURLY_REFRESH_MINUTES=15
MV_DAILY_REFRESH_HOUR=2
MV_GLOBAL_REFRESH_MINUTES=5
```

### PostgreSQL Configuration

```sql
-- Enable better materialized view performance
ALTER SYSTEM SET
  work_mem = '64MB',           -- More memory for sorting
  maintenance_work_mem = '1GB', -- For REFRESH MATERIALIZED VIEW
  max_parallel_workers_per_gather = 4; -- Parallel refresh

-- Reload config
SELECT pg_reload_conf();
```

## Performance Impact

### Expected Improvements

| Query Type | Before | After | Improvement |
|------------|--------|-------|-------------|
| Hourly aggregation | 100ms | 1ms | 99% faster |
| Daily aggregation | 500ms | 2.5ms | 99.5% faster |
| Global stats | 200ms | <1ms | 99.5% faster |
| Device dashboard | 800ms | 50ms | 94% faster |

### Storage Impact

| View | Rows | Size | Refresh Time |
|------|------|------|--------------|
| Hourly stats | ~1.2M | ~500 MB | ~30s |
| Daily stats | ~50K | ~100 MB | ~60s |
| Global stats | ~10 | ~1 KB | ~5s |

**Total Storage**: ~600 MB (1.2% of 50 GB table)

## Testing Steps

### 1. Create Materialized Views

```bash
# Run migration
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/004_materialized_views.sql

# Verify views created
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dmv"
```

### 2. Query Materialized Views

```bash
# Test hourly stats query
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT * FROM mv_device_hourly_stats
WHERE device_id = 'device-1'
  AND hour >= NOW() - INTERVAL '24 hours'
ORDER BY hour DESC
LIMIT 10;
"

# Test daily stats query
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT * FROM mv_device_daily_stats
WHERE device_id = 'device-1'
  AND day >= CURRENT_DATE - INTERVAL '7 days'
ORDER BY day DESC;
"

# Test global stats
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT * FROM mv_global_stats;
"
```

### 3. Performance Comparison

```bash
# Compare raw query vs materialized view
time docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT
    device_id,
    date_trunc('hour', timestamp) as hour,
    avg(value) as avg_value
FROM sensor_readings
WHERE device_id = 'device-1'
  AND timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY device_id, date_trunc('hour', timestamp);
"

time docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT device_id, hour, avg_value
FROM mv_device_hourly_stats
WHERE device_id = 'device-1'
  AND hour >= NOW() - INTERVAL '24 hours';
"
```

### 4. Refresh Materialized Views

```bash
# Test manual refresh
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_device_hourly_stats;
"

# Check refresh time
time docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_device_daily_stats;
"
```

### 5. Automated Refresh

```bash
# Run automated refresh script
./scripts/refresh_materialized_views.sh

# Schedule with cron
crontab -e
# Add: */15 * * * * /path/to/scripts/refresh_materialized_views.sh hourly
```

## Rollback Plan

If materialized views cause issues:

### Disable Queries to Materialized Views

```sql
-- Drop views
DROP MATERIALIZED VIEW IF EXISTS mv_device_hourly_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS mv_device_daily_stats CASCADE;
DROP MATERIALIZED VIEW IF EXISTS mv_global_stats CASCADE;
```

### Revert Application Code

```go
// In internal/repository/sensor_repo.go
// Comment out methods that query materialized views
// Use original queries instead
```

### Verify Rollback

```bash
# Verify views dropped
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dmv"

# Run load test to confirm baseline performance
./scripts/test-runner.sh
```

## Monitoring and Maintenance

### Daily Checks

```sql
-- Check materialized view refresh lag
SELECT
    schemaname,
    matviewname,
    pg_size_pretty(pg_relation_size(matviewname::regclass)) as size,
    (SELECT MAX(timestamp) FROM sensor_readings) as latest_data,
    (SELECT MAX(hour) FROM mv_device_hourly_stats) as latest_hourly_mv
FROM pg_matviews
WHERE matviewname LIKE 'mv_%';
```

### Weekly Maintenance

```sql
-- Analyze materialized views for optimal query plans
ANALYZE mv_device_hourly_stats;
ANALYZE mv_device_daily_stats;
ANALYZE mv_global_stats;

-- Check for bloat
SELECT
    matviewname,
    pg_size_pretty(pg_relation_size(matviewname::regclass)) as size,
    pg_size_pretty(bloat_size) as bloat
FROM (
    SELECT
        matviewname,
        matviewname::regclass as matview_oid,
        pg_relation_size(matviewname::regclass) -
        (SELECT count(*) * (SELECT avg_tuple_len FROM pg_stats WHERE tablename = matviewname))
    FROM pg_matviews
    WHERE matviewname LIKE 'mv_%'
) sub;
```

### Monthly Tasks

```sql
-- Rebuild materialized views if bloat detected
DROP MATERIALIZED VIEW IF EXISTS mv_device_hourly_stats;
-- Recreate from migration script

-- Review refresh intervals
-- Adjust based on data volume and query patterns
```

## Troubleshooting

### Issue 1: Materialized View Refresh Takes Too Long

**Symptoms**: REFRESH CONCURRENTLY takes > 5 minutes

**Diagnosis**:
```sql
-- Check refresh progress
SELECT pid, query, wait_event_type, wait_event
FROM pg_stat_activity
WHERE query LIKE '%REFRESH MATERIALIZED VIEW%';

-- Check for locks
SELECT * FROM pg_locks
WHERE relation = 'mv_device_hourly_stats'::regclass;
```

**Solution**:
- Increase `maintenance_work_mem` for faster sorts
- Use REFRESH CONCURRENTLY to avoid locks
- Schedule refresh during low-traffic periods
- Consider reducing refresh frequency

### Issue 2: Stale Data in Materialized Views

**Symptoms**: Materialized view data doesn't match base table

**Diagnosis**:
```sql
-- Compare row counts
SELECT
    (SELECT count(*) FROM sensor_readings) as base_count,
    (SELECT sum(reading_count) FROM mv_device_hourly_stats) as mv_count;
```

**Solution**:
- Check cron jobs are running
- Verify automated refresh script
- Manually refresh: `REFRESH MATERIALIZED VIEW CONCURRENTLY mv_device_hourly_stats`

### Issue 3: Materialized View Not Used by Query Planner

**Symptoms**: Query plan doesn't use materialized view

**Diagnosis**:
```sql
EXPLAIN ANALYZE
SELECT * FROM mv_device_hourly_stats
WHERE device_id = 'device-1';
```

**Solution**:
- Run `ANALYZE mv_device_hourly_stats` to update statistics
- Check for missing indexes on materialized view
- Verify query conditions match materialized view definition

## References

### PostgreSQL Documentation
- [Materialized Views](https://www.postgresql.org/docs/current/sql-creatematerializedview.html)
- [REFRESH MATERIALIZED VIEW](https://www.postgresql.org/docs/current/sql-refreshmaterializedview.html)
- [Query Planning](https://www.postgresql.org/docs/current/sql-explain.html)

### Best Practices
- Materialized View Design: https://www.postgresql.org/docs/current/rules-materializedviews.html
- Refresh Strategies: https://wiki.postgresql.org/wiki/Materialized_views
- Partitioning with Materialized Views: https://www.citus.io/blog/timeseries-data-why-you-need-a-time-series-database

### Industry Standards
- For real-time analytics: Refresh every 5-15 minutes
- For daily reports: Refresh once daily (off-peak hours)
- For historical data: Refresh weekly or monthly
- Always use CONCURRENTLY for production refreshes

## Success Criteria

- [ ] All materialized views created successfully
- [ ] Queries using materialized views 99% faster
- [ ] Automated refresh script working
- [ ] Storage overhead < 2% of base table
- [ ] No significant performance degradation during refresh
- [ ] Application code updated to use materialized views
- [ ] Load tests show improvement over baseline
- [ ] Monitoring in place for refresh lag
- [ ] Rollback plan tested and documented

## Changelog

**2026-03-15**: Initial Phase B4 documentation created
- Defined 3 materialized views for query optimization
- Created refresh strategies and automation procedures
- Documented query optimization techniques
- Added troubleshooting and maintenance procedures
