-- =============================================================================
-- High-Throughput PostgreSQL Schema Example
-- =============================================================================
-- This is a use-case-agnostic schema template for exact-ID query patterns.
-- Adapt the table name and columns to your specific use case.
--
-- Performance Target: p95 ≤ 500ms for exact-ID queries
-- Data Volume: Tested with 50M+ rows
-- =============================================================================

-- =============================================================================
-- Step 1: Create the main table
-- =============================================================================

CREATE TABLE IF NOT EXISTS entity_readings (
    -- Primary key: BIGSERIAL for sequential, compact IDs
    id              BIGSERIAL       PRIMARY KEY,

    -- Entity identifier: The primary lookup column
    entity_id       VARCHAR(50)     NOT NULL,

    -- Timestamp: Time-series data ordering
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- Type/Category: For filtering and aggregation
    reading_type    VARCHAR(20)     NOT NULL,

    -- Value: Numeric measurement or data point
    value           DECIMAL(10,2)   NOT NULL,

    -- Unit: Measurement unit or metadata
    unit            VARCHAR(20)     NOT NULL,

    -- Optional: Flexible metadata storage
    metadata        JSONB
);

COMMENT ON TABLE entity_readings IS
'High-throughput table for exact-ID queries on time-series data';

-- =============================================================================
-- Step 2: Basic indexes (create immediately after table creation)
-- =============================================================================

-- Composite index for primary query pattern
-- Matches: "Get recent N readings for entity X"
CREATE INDEX IF NOT EXISTS idx_entity_readings_entity_timestamp
ON entity_readings (entity_id, timestamp DESC);

COMMENT ON INDEX idx_entity_readings_entity_timestamp IS
'Supports primary query pattern: exact-ID lookup with recent-first ordering';

-- Index on reading_type for filtering
CREATE INDEX IF NOT EXISTS idx_entity_readings_reading_type
ON entity_readings (reading_type);

COMMENT ON INDEX idx_entity_readings_reading_type IS
'Supports queries filtering by reading type';

-- =============================================================================
-- Step 3: Advanced indexes (create AFTER data generation)
-- =============================================================================
-- IMPORTANT: Run these after generating your test data
-- For production, use CREATE INDEX CONCURRENTLY to avoid blocking

-- BRIN index for time-series data (very compact)
-- Best for: Append-only data > 100MB
CREATE INDEX IF NOT EXISTS idx_entity_readings_timestamp_brin
ON entity_readings USING BRIN (timestamp);

COMMENT ON INDEX idx_entity_readings_timestamp_brin IS
'BRIN index for time-range queries. 99% smaller than B-tree for time-series data';

-- Covering index for index-only scans
-- Eliminates heap access for SELECT queries
CREATE INDEX IF NOT EXISTS idx_entity_readings_entity_covering
ON entity_readings (entity_id, timestamp DESC)
INCLUDE (reading_type, value, unit, metadata);

COMMENT ON INDEX idx_entity_readings_entity_covering IS
'Covering index for index-only scans. 2-5x faster than regular index';

-- =============================================================================
-- Step 4: Materialized view for statistics
-- =============================================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_entity_stats AS
SELECT
    entity_id,
    reading_type,
    COUNT(*) as reading_count,
    AVG(value) as avg_value,
    MIN(value) as min_value,
    MAX(value) as max_value,
    STDDEV(value) as stddev_value
FROM entity_readings
GROUP BY entity_id, reading_type;

-- Unique index for concurrent refresh
CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_entity_stats_unique
ON mv_entity_stats (entity_id, reading_type);

-- Index for looking up stats by entity
CREATE INDEX IF NOT EXISTS idx_mv_entity_stats_entity_id
ON mv_entity_stats (entity_id);

COMMENT ON MATERIALIZED VIEW mv_entity_stats IS
'Pre-computed statistics for fast dashboard queries';

COMMENT ON INDEX idx_mv_entity_stats_entity_id IS
'Supports fast lookups of entity statistics';

-- =============================================================================
-- Step 5: Refresh function for materialized view
-- =============================================================================

CREATE OR REPLACE FUNCTION refresh_entity_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_entity_stats;
    RAISE NOTICE 'Entity statistics refreshed successfully';
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_entity_stats() IS
'Refreshes the mv_entity_stats materialized view concurrently';

-- =============================================================================
-- Step 6: Verification queries
-- =============================================================================

-- Check table size
SELECT
    pg_size_pretty(pg_total_relation_size('entity_readings')) as total_size,
    pg_size_pretty(pg_relation_size('entity_readings')) as table_size,
    pg_size_pretty(pg_total_relation_size('entity_readings') - pg_relation_size('entity_readings')) as index_size;

-- Check index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
WHERE tablename = 'entity_readings'
ORDER BY idx_scan DESC;

-- Test query with EXPLAIN ANALYZE
EXPLAIN (ANALYZE, BUFFERS)
SELECT entity_id, timestamp, reading_type, value, unit
FROM entity_readings
WHERE entity_id = 'entity-000001'
ORDER BY timestamp DESC
LIMIT 100;

-- =============================================================================
-- Adaptation Guide
-- =============================================================================
--
-- To adapt this schema to your use case:
--
-- 1. Rename the table:
--    - entity_readings → your_table_name
--
-- 2. Rename entity_id:
--    - entity_id → user_id, device_id, account_id, etc.
--
-- 3. Adjust reading_type:
--    - reading_type → activity_type, transaction_type, event_type, etc.
--    - Adjust VARCHAR length as needed
--
-- 4. Adjust value precision:
--    - DECIMAL(10,2) → DECIMAL(15,2) for currency
--    - DECIMAL(15,6) for scientific data
--
-- 5. Add columns specific to your use case:
--    - status VARCHAR(20)
--    - source VARCHAR(50)
--    - flags JSONB
--
-- 6. Update index names to match your table name:
--    - idx_entity_readings_* → idx_your_table_*
--
-- 7. Update materialized view name:
--    - mv_entity_stats → mv_your_table_stats
--
-- =============================================================================
