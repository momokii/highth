-- Migration 005: Incremental Materialized View Refresh
--
-- This migration adds functions to refresh only recent data instead of
-- the entire materialized view, dramatically improving refresh performance.
--
-- Run with: docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/005_incremental_mv_refresh.sql
--
-- Author: Higth Optimization Team
-- Date: 2026-03-25
-- Version: 1.0.0
--
-- Problem: mv_device_hourly_stats refresh takes 8+ minutes with 83M rows
-- Solution: Only refresh last N days of data (default 7 days)
-- Expected Result: ~30 seconds refresh time (down from 8+ minutes)

BEGIN;

-- =============================================================================
-- Incremental Refresh Function for Hourly Stats
-- =============================================================================

CREATE OR REPLACE FUNCTION refresh_hourly_stats_incremental(days_to_refresh INT DEFAULT 7)
RETURNS void AS $$
DECLARE
    deleted_count INT;
    inserted_count INT;
    cutoff_time TIMESTAMP WITH TIME ZONE;
BEGIN
    cutoff_time := NOW() - (days_to_refresh || ' days')::interval;

    RAISE NOTICE 'Starting incremental refresh for mv_device_hourly_stats';
    RAISE NOTICE 'Refreshing last % days of data (since %)', days_to_refresh, cutoff_time;

    -- Delete old data for the period we're refreshing
    DELETE FROM mv_device_hourly_stats
    WHERE hour >= cutoff_time;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % rows from mv_device_hourly_stats', deleted_count;

    -- Insert fresh data for the recent period
    INSERT INTO mv_device_hourly_stats
    SELECT
        device_id,
        date_trunc('hour', timestamp) as hour,
        reading_type,
        count(*) as reading_count,
        round(avg(value)::numeric, 2) as avg_value,
        round(min(value)::numeric, 2) as min_value,
        round(max(value)::numeric, 2) as max_value,
        round(stddev(value)::numeric, 2) as stddev_value,
        min(timestamp) as first_reading,
        max(timestamp) as last_reading
    FROM sensor_readings
    WHERE timestamp >= cutoff_time
    GROUP BY device_id, date_trunc('hour', timestamp), reading_type
    ON CONFLICT (device_id, hour, reading_type) DO NOTHING;

    GET DIAGNOSTICS inserted_count = ROW_COUNT;
    RAISE NOTICE 'Inserted % rows into mv_device_hourly_stats', inserted_count;
    RAISE NOTICE 'Incremental refresh complete for last % days', days_to_refresh;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_hourly_stats_incremental IS
'Incremental refresh for mv_device_hourly_stats. Only refreshes the last N days of data.
Default: 7 days. Call: SELECT refresh_hourly_stats_incremental(7);
Performance: ~30 seconds for 7 days vs 8+ minutes for full refresh.';

-- =============================================================================
-- Incremental Refresh Function for Daily Stats
-- =============================================================================

CREATE OR REPLACE FUNCTION refresh_daily_stats_incremental(days_to_refresh INT DEFAULT 30)
RETURNS void AS $$
DECLARE
    deleted_count INT;
    inserted_count INT;
    cutoff_time TIMESTAMP WITH TIME ZONE;
BEGIN
    cutoff_time := NOW() - (days_to_refresh || ' days')::interval;

    RAISE NOTICE 'Starting incremental refresh for mv_device_daily_stats';
    RAISE NOTICE 'Refreshing last % days of data (since %)', days_to_refresh, cutoff_time;

    -- Delete old data for the period we're refreshing
    DELETE FROM mv_device_daily_stats
    WHERE day >= cutoff_time;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Deleted % rows from mv_device_daily_stats', deleted_count;

    -- Insert fresh data for the recent period
    INSERT INTO mv_device_daily_stats
    SELECT
        device_id,
        date_trunc('day', timestamp) as day,
        reading_type,
        count(*) as reading_count,
        round(avg(value)::numeric, 2) as avg_value,
        round(min(value)::numeric, 2) as min_value,
        round(max(value)::numeric, 2) as max_value,
        round(stddev(value)::numeric, 2) as stddev_value,
        round(percentile_cont(0.5) WITHIN GROUP (ORDER BY value)::numeric, 2) as median_value,
        round(percentile_cont(0.95) WITHIN GROUP (ORDER BY value)::numeric, 2) as p95_value,
        round(percentile_cont(0.99) WITHIN GROUP (ORDER BY value)::numeric, 2) as p99_value,
        min(timestamp) as first_reading,
        max(timestamp) as last_reading
    FROM sensor_readings
    WHERE timestamp >= cutoff_time
    GROUP BY device_id, date_trunc('day', timestamp), reading_type
    ON CONFLICT (device_id, day, reading_type) DO NOTHING;

    GET DIAGNOSTICS inserted_count = ROW_COUNT;
    RAISE NOTICE 'Inserted % rows into mv_device_daily_stats', inserted_count;
    RAISE NOTICE 'Incremental refresh complete for last % days', days_to_refresh;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_daily_stats_incremental IS
'Incremental refresh for mv_device_daily_stats. Only refreshes the last N days of data.
Default: 30 days. Call: SELECT refresh_daily_stats_incremental(30);
Note: Percentiles (median, p95, p99) are computed only for refreshed period.';

-- =============================================================================
-- Wrapper Function for Global Stats
-- =============================================================================

CREATE OR REPLACE FUNCTION refresh_global_stats_incremental()
RETURNS void AS $$
BEGIN
    RAISE NOTICE 'Refreshing global stats (CONCURRENTLY)';
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_global_stats;
    RAISE NOTICE 'Global stats refresh complete';
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_global_stats_incremental IS
'Wrapper to refresh mv_global_stats using CONCURRENTLY.
Global stats only has ~3 rows (one per reading type), so it is very fast.';

-- =============================================================================
-- Verification and Validation
-- =============================================================================

DO $$
DECLARE
    func_count INTEGER;
BEGIN
    -- Count functions created
    SELECT count(*) INTO func_count
    FROM pg_proc
    WHERE proname IN (
        'refresh_hourly_stats_incremental',
        'refresh_daily_stats_incremental',
        'refresh_global_stats_incremental'
    );

    RAISE NOTICE '';
    RAISE NOTICE '╔════════════════════════════════════════════════════════════════╗';
    RAISE NOTICE '║      Incremental MV Refresh Migration (005) Complete          ║';
    RAISE NOTICE '╚════════════════════════════════════════════════════════════════╝';
    RAISE NOTICE '';
    RAISE NOTICE 'Functions Created: %', func_count;
    RAISE NOTICE '';
    RAISE NOTICE 'Available Functions:';
    RAISE NOTICE '  1. refresh_hourly_stats_incremental(days)   - Default 7 days';
    RAISE NOTICE '  2. refresh_daily_stats_incremental(days)    - Default 30 days';
    RAISE NOTICE '  3. refresh_global_stats_incremental()       - Fast full refresh';
    RAISE NOTICE '';
    RAISE NOTICE 'Usage Examples:';
    RAISE NOTICE '  SELECT refresh_hourly_stats_incremental(7);   -- Last 7 days';
    RAISE NOTICE '  SELECT refresh_daily_stats_incremental(30);   -- Last 30 days';
    RAISE NOTICE '  SELECT refresh_global_stats_incremental();';
    RAISE NOTICE '';
    RAISE NOTICE 'Next Steps:';
    RAISE NOTICE '  1. Test: SELECT refresh_hourly_stats_incremental(1);';
    RAISE NOTICE '  2. Update refresh script to use incremental functions';
    RAISE NOTICE '  3. Set up automated refresh with appropriate intervals';
    RAISE NOTICE '';
END $$;

COMMIT;

-- =============================================================================
-- Rollback Instructions (if needed)
-- =============================================================================

-- To rollback this migration, run:
--
-- BEGIN;
-- DROP FUNCTION IF EXISTS refresh_hourly_stats_incremental(INT);
-- DROP FUNCTION IF EXISTS refresh_daily_stats_incremental(INT);
-- DROP FUNCTION IF EXISTS refresh_global_stats_incremental();
-- COMMIT;
--
-- Then verify with:
-- \df refresh_
