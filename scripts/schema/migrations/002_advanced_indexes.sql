-- Migration 002: Advanced Indexing for Production Workloads
--
-- This migration adds performance indexes for time-series IoT sensor data.
--
-- Run with: docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/002_advanced_indexes.sql
--
-- IMPORTANT: Run AFTER data generation completes
--
-- Note: Using regular indexes instead of CONCURRENTLY for smaller datasets
-- For large datasets (>10M rows), consider using CONCURRENTLY

-- =============================================================================
-- Index 1: BRIN Index for Time-Series Data
-- =============================================================================
-- BRIN indexes are very compact for time-series data
-- Best for tables larger than 100MB

CREATE INDEX IF NOT EXISTS idx_sensor_readings_timestamp_brin
ON sensor_readings USING BRIN (timestamp);

COMMENT ON INDEX idx_sensor_readings_timestamp_brin IS
'BRIN index for timestamp. Very compact for time-series data.';

-- =============================================================================
-- Index 2: Composite Index for Device Queries
-- =============================================================================

CREATE INDEX IF NOT EXISTS idx_sensor_readings_device_type_timestamp
ON sensor_readings (device_id, reading_type, timestamp DESC);

COMMENT ON INDEX idx_sensor_readings_device_type_timestamp IS
'Covers most sensor query patterns.';

-- =============================================================================
-- Verification
-- =============================================================================

DO $$
DECLARE
    index_count INTEGER;
BEGIN
    SELECT count(*) INTO index_count
    FROM pg_indexes
    WHERE tablename = 'sensor_readings'
      AND schemaname = 'public'
      AND indexname IN (
          'idx_sensor_readings_timestamp_brin',
          'idx_sensor_readings_device_type_timestamp'
      );

    RAISE NOTICE 'Performance indexes created: %', index_count;
    RAISE NOTICE 'Setup complete! Database ready for queries.';
END $$;
