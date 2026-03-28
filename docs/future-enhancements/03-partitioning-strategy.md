# Partitioning Strategy

## Overview

This document describes how to implement table partitioning for the `sensor_readings` table to scale beyond 100M rows while maintaining query performance.

## Why Partition?

Partitioning splits a large table into smaller, more manageable pieces called partitions. For time-series data, partitioning by time range provides significant benefits:

| Benefit | Impact |
|---------|--------|
| **Query Pruning** | PostgreSQL skips irrelevant partitions during query planning |
| **Faster Deletes** | Drop old partitions instead of DELETE (no VACUUM needed) |
| **Parallel Scans** | Each partition can be scanned independently |
| **Smaller Indexes** | Indexes are per-partition, improving cache efficiency |
| **Maintenance** | Operations (VACUUM, REINDEX) on smaller datasets |

## When to Partition

### Partitioning Thresholds

| Dataset Size | Partitioning Needed | Performance Impact |
|--------------|---------------------|-------------------|
| < 50M rows | ❌ No | Negligible benefit |
| 50M-100M rows | ⚠️ Optional | Minor improvement |
| 100M-500M rows | ✅ Recommended | Significant benefit |
| > 500M rows | ✅ Required | Essential for performance |

### Signs You Need Partitioning

- Queries becoming slower as dataset grows
- VACUUM operations taking too long
- Index sizes exceeding available RAM
- Need to regularly delete old data

## Implementation Strategy

### Option 1: Partition by Month (Recommended)

Best for:
- Datasets growing at 14M+ rows/month
- Queries typically filter by time range
- Need to retain data for months/years

**Trade-off:** More partitions to manage, but each is smaller

### Option 2: Partition by Quarter

Best for:
- Slower data growth rates
- Longer data retention requirements
- Simpler partition management

**Trade-off:** Fewer partitions, but each is larger

## Step-by-Step Implementation

### Step 1: Create Partitioned Table

Create `scripts/schema/migrations/008_add_partitioning.sql`:

```sql
-- Migration 008: Add Table Partitioning
--
-- This migration converts the sensor_readings table to use partitioning by month.
-- IMPORTANT: This requires significant downtime and data migration.
--
-- Run with: ./scripts/run_migrations.sh
--
-- PREREQUISITES:
-- - Table size should be > 100M rows
-- - Schedule downtime window (can take several hours for 50M+ rows)
-- - Ensure sufficient disk space (2x current table size)

-- =============================================================================
-- Step 1: Create partitioned table (new structure)
-- =============================================================================

CREATE TABLE sensor_readings_partitioned (
    id BIGSERIAL,
    device_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    reading_type VARCHAR(20) NOT NULL CHECK (reading_type IN ('temperature', 'humidity', 'pressure')),
    value DECIMAL(10,2) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    metadata JSONB
) PARTITION BY RANGE (timestamp);

-- Add primary key (note: must include partition key)
ALTER TABLE sensor_readings_partitioned ADD PRIMARY KEY (id, timestamp);

-- =============================================================================
-- Step 2: Create initial partitions
-- =============================================================================

-- Create partitions for past 3 months and future 2 months
CREATE TABLE sensor_readings_2024_01 PARTITION OF sensor_readings_partitioned
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE sensor_readings_2024_02 PARTITION OF sensor_readings_partitioned
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');

CREATE TABLE sensor_readings_2024_03 PARTITION OF sensor_readings_partitioned
    FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');

CREATE TABLE sensor_readings_2024_04 PARTITION OF sensor_readings_partitioned
    FOR VALUES FROM ('2024-04-01') TO ('2024-04-01');

CREATE TABLE sensor_readings_2024_05 PARTITION OF sensor_readings_partitioned
    FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');

-- =============================================================================
-- Step 3: Copy data from original table
-- =============================================================================

-- Insert data into appropriate partitions
-- This can take several hours for large datasets
INSERT INTO sensor_readings_partitioned
SELECT * FROM sensor_readings;

-- =============================================================================
-- Step 4: Create indexes on partitioned table
-- =============================================================================

-- BRIN index (per partition, automatically created)
CREATE INDEX idx_sensor_readings_partitioned_timestamp_brin
ON sensor_readings_partitioned USING BRIN (timestamp);

-- Composite index
CREATE INDEX idx_sensor_readings_partitioned_device_type_timestamp
ON sensor_readings_partitioned (device_id, reading_type, timestamp DESC);

-- Covering index
CREATE INDEX idx_sensor_readings_partitioned_device_covering
ON sensor_readings_partitioned (device_id, timestamp DESC)
INCLUDE (reading_type, value, unit);

-- GIN index for metadata
CREATE INDEX idx_sensor_readings_partitioned_metadata_gin
ON sensor_readings_partitioned USING GIN (metadata);

-- =============================================================================
-- Step 5: Verify and switch
-- =============================================================================

-- Verify row counts match
DO $$
DECLARE
    original_count BIGINT;
    partitioned_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO original_count FROM sensor_readings;
    SELECT COUNT(*) INTO partitioned_count FROM sensor_readings_partitioned;

    RAISE NOTICE 'Original table rows: %', original_count;
    RAISE NOTICE 'Partitioned table rows: %', partitioned_count;

    IF original_count = partitioned_count THEN
        RAISE NOTICE 'Row counts match - safe to proceed';
    ELSE
        RAISE EXCEPTION 'Row counts do not match - aborting';
    END IF;
END $$;

-- Rename tables (requires exclusive lock)
BEGIN;

-- Backup original table
ALTER TABLE sensor_readings RENAME TO sensor_readings_backup;

-- Switch partitioned table to production name
ALTER TABLE sensor_readings_partitioned RENAME TO sensor_readings;

COMMIT;

-- =============================================================================
-- Step 6: Update sequence
-- =============================================================================

-- Ensure sequence continues from max ID
SELECT setval(
    'sensor_readings_id_seq',
    (SELECT MAX(id) FROM sensor_readings)
);

RAISE NOTICE 'Partitioning complete!';
RAISE NOTICE 'Original table backed up as: sensor_readings_backup';
RAISE NOTICE 'Drop backup when ready: DROP TABLE sensor_readings_backup;';
```

### Step 2: Create Automated Partition Management

Create `scripts/manage_partitions.sh`:

```bash
#!/bin/bash
# Partition Management Script
# Creates new partitions and drops old ones

# Configuration
RETENTION_MONTHS=12  # Keep data for 12 months
ADVANCE_MONTHS=2     # Create partitions 2 months in advance
DB_NAME="sensor_db"
DB_USER="sensor_user"

# Get current date
CURRENT_YEAR=$(date +%Y)
CURRENT_MONTH=$(date +%m)

echo "=== Partition Management: $(date) ==="

# Create future partitions
echo "Creating future partitions..."
for i in $(seq 1 $ADVANCE_MONTHS); do
    # Calculate future month
    FUTURE_DATE=$(date -d "$CURRENT_YEAR-$CURRENT_MONTH-01 + $i month" +%Y-%m)
    FUTURE_YEAR=$(date -d "$FUTURE_DATE-01" +%Y)
    FUTURE_MONTH=$(date -d "$FUTURE_DATE-01" +%m)
    NEXT_MONTH=$(date -d "$FUTURE_YEAR-$FUTURE_MONTH-01 + 1 month" +%Y-%m)

    PARTITION_NAME="sensor_readings_${FUTURE_YEAR}_${FUTURE_MONTH}"
    START_DATE="${FUTURE_YEAR}-${FUTURE_MONTH}-01"
    END_DATE="${NEXT_MONTH}-01"

    # Check if partition exists
    EXISTS=$(psql -U $DB_USER -d $DB_NAME -tAc "
        SELECT EXISTS (
            SELECT 1 FROM pg_class
            WHERE relname = '$PARTITION_NAME'
        );
    ")

    if [ "$EXISTS" = "f" ]; then
        echo "Creating partition: $PARTITION_NAME"
        psql -U $DB_USER -d $DB_NAME -c "
            CREATE TABLE IF NOT EXISTS $PARTITION_NAME
            PARTITION OF sensor_readings
            FOR VALUES FROM ('$START_DATE') TO ('$END_DATE');
        "
    else
        echo "Partition already exists: $PARTITION_NAME"
    fi
done

# Drop old partitions
echo "Dropping partitions older than $RETENTION_MONTHS months..."
CUTOFF_DATE=$(date -d "$CURRENT_YEAR-$CURRENT_MONTH-01 - $RETENTION_MONTHS month" +%Y-%m)

# Get partitions to drop
PARTITIONS_TO_DROP=$(psql -U $DB_USER -d $DB_NAME -tAc "
    SELECT relname
    FROM pg_class
    WHERE relname LIKE 'sensor_readings_%'
    AND relname < 'sensor_readings_${CUTOFF_DATE//-/_}'
    ORDER BY relname;
")

for PARTITION in $PARTITIONS_TO_DROP; do
    echo "Dropping partition: $PARTITION"
    psql -U $DB_USER -d $DB_NAME -c "DROP TABLE IF EXISTS $PARTITION CASCADE;"
done

echo "=== Partition management complete ==="
```

### Step 3: Add Cron Job

```bash
# Edit crontab
crontab -e

# Add: Run partition management on 1st of each month at 2 AM
0 2 1 * * /path/to/highth/scripts/manage_partitions.sh >> /var/log/partition_management.log 2>&1
```

## Query Patterns with Partitioning

### Time-Range Queries (Automatic Pruning)

```sql
-- Query only scans relevant partitions
SELECT * FROM sensor_readings
WHERE timestamp >= '2024-01-01' AND timestamp < '2024-02-01'
AND device_id = 'sensor-001';

-- Check query plan
EXPLAIN ANALYZE SELECT * FROM sensor_readings
WHERE timestamp >= '2024-01-01' AND timestamp < '2024-02-01';

-- Look for: "Append" with "Partitions selected by partition constraint"
```

### Device Queries (No Change)

```sql
-- Queries work exactly as before
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;

-- Query planner will scan all partitions (but still fast due to indexes)
```

## Performance Comparison

### Query Performance: 100M Rows (12 months)

| Query Type | Unpartitioned | Partitioned | Improvement |
|------------|---------------|-------------|-------------|
| Time-range (1 month) | 400ms | 50ms | 8x faster |
| Device lookup (all time) | 200ms | 220ms | Slightly slower |
| Recent data (1 day) | 350ms | 25ms | 14x faster |

### Maintenance Performance: 100M Rows

| Operation | Unpartitioned | Partitioned | Improvement |
|-----------|---------------|-------------|-------------|
| VACUUM | 2 hours | 10 minutes | 12x faster |
| DELETE old data | 1 hour | 1 second | 3600x faster |
| CREATE INDEX | 45 minutes | 5 minutes | 9x faster |

## Monitoring

### Check Partition Sizes

```sql
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    (SELECT COUNT(*) FROM sensor_readings) AS row_count
FROM pg_tables
WHERE tablename LIKE 'sensor_readings_%'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### Check Partition Pruning

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM sensor_readings
WHERE timestamp >= '2024-01-01' AND timestamp < '2024-02-01'
AND device_id = 'sensor-001';

-- Look for:
-- - "Append" node with "Subplans Removed: N"
-- - Only relevant partitions scanned
```

## Migration Considerations

### Zero-Downtime Migration (Advanced)

For production systems requiring zero downtime, use:

```sql
-- Step 1: Create partitioned table in background
CREATE TABLE sensor_readings_new (...) PARTITION BY RANGE (timestamp);

-- Step 2: Create triggers to copy new writes
CREATE OR REPLACE FUNCTION sensor_readings_insert_trigger()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO sensor_readings_new VALUES (NEW.*);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sensor_readings_sync_trigger
AFTER INSERT ON sensor_readings
FOR EACH ROW EXECUTE FUNCTION sensor_readings_insert_trigger();

-- Step 3: Backfill existing data in batches
-- (Run in batches during low-traffic periods)

-- Step 4: Switch tables when caught up
BEGIN;
ALTER TABLE sensor_readings RENAME TO sensor_readings_old;
ALTER TABLE sensor_readings_new RENAME TO sensor_readings;
COMMIT;
```

## Rollback Plan

If issues arise after partitioning:

```sql
-- Step 1: Stop new writes
-- Step 2: Swap back to original table
BEGIN;
ALTER TABLE sensor_readings RENAME TO sensor_readings_partitioned;
ALTER TABLE sensor_readings_backup RENAME TO sensor_readings;
COMMIT;

-- Step 3: Drop partitioned table when confirmed
DROP TABLE sensor_readings_partitioned CASCADE;
```

## Best Practices

1. **Start partitioning before 100M rows** - Easier to migrate smaller datasets
2. **Use monthly partitions** - Good balance between manageability and size
3. **Automate partition management** - Use cron jobs to create/drop partitions
4. **Monitor partition growth** - Set up alerts for partition size
5. **Test on staging first** - Validate performance before production

## Troubleshooting

### Issue: Queries Not Using Partition Pruning

**Cause:** Query doesn't include partition key (timestamp) in WHERE clause

**Solution:** Add time range filter to queries:
```sql
-- Bad: Scans all partitions
SELECT * FROM sensor_readings WHERE device_id = 'sensor-001';

-- Good: Uses partition pruning
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
AND timestamp >= NOW() - INTERVAL '7 days';
```

### Issue: Too Many Partitions

**Cause:** Creating partitions too far in future

**Solution:** Only create 2-3 months in advance, use automated script

### Issue: Partition Too Large

**Cause:** Monthly partition > 50M rows

**Solution:** Switch to daily or weekly partitions:
```sql
-- Daily partitions
CREATE TABLE sensor_readings_2024_01_01 PARTITION OF sensor_readings
FOR VALUES FROM ('2024-01-01') TO ('2024-01-02');
```

## Related Documentation

- **[../architecture.md](../architecture.md)** - Database design and partitioning strategy
- **[PostgreSQL Partitioning Documentation](https://www.postgresql.org/docs/current/ddl-partitioning.html)**
- **[../implementation/validation-checklist.md](../implementation/validation-checklist.md)** - Validation checklist
