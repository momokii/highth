# Schema Type Corrections

## Overview

This document describes how to align the actual database schema types with the original documentation. The current implementation uses slightly different data types than what was documented.

## Type Differences

| Column | Documented Type | Actual Type | Difference |
|--------|-----------------|-------------|------------|
| `reading_type` | VARCHAR(30) | VARCHAR(20) | 10 chars less |
| `value` | NUMERIC(15,6) | DECIMAL(10,2) | Less precision |
| `timestamp` | TIMESTAMPTZ | TIMESTAMPTZ | ✅ Same |
| `device_id` | VARCHAR(50) | VARCHAR(50) | ✅ Same |
| `unit` | VARCHAR(20) | VARCHAR(20) | ✅ Same |
| `metadata` | JSONB | Not implemented | See enhancement #2 |

## Why These Differences Exist

### VARCHAR(20) vs VARCHAR(30) for reading_type

**Documented:** `VARCHAR(30)` to accommodate longer reading type names
**Actual:** `VARCHAR(20)` with CHECK constraint for known values

**Rationale:** The CHECK constraint ensures only valid values:
```sql
CHECK (reading_type IN ('temperature', 'humidity', 'pressure'))
```

This provides data integrity and prevents typos.

### DECIMAL(10,2) vs NUMERIC(15,6) for value

**Documented:** `NUMERIC(15,6)` for high-precision scientific data
**Actual:** `DECIMAL(10,2)` for simpler precision

**Rationale:** Most IoT sensors report 2 decimal places. The actual type is:
- `DECIMAL(10,2)`: Up to 10 digits total, 2 after decimal
  - Range: -99,999,999.99 to 99,999,999.99
- `NUMERIC(15,6)`: Up to 15 digits total, 6 after decimal
  - Range: -9,999,999,999.999999 to 9,999,999,999.999999

## When to Correct

### Correct reading_type to VARCHAR(30) IF:

- Adding new reading types longer than 20 characters
- Need to store user-defined reading types
- Removing the CHECK constraint for flexibility

### Correct value to NUMERIC(15,6) IF:

- Storing scientific sensor data requiring 6 decimal places
- Need values larger than ±99 million
- Working with precision instruments

## Implementation

### Option 1: Align to Documentation (Full Correction)

Create `scripts/schema/migrations/009_schema_type_corrections.sql`:

```sql
-- Migration 009: Schema Type Corrections
--
-- This migration aligns column types with the original documentation.
-- WARNING: This will require a full table rewrite and may take significant time.
--
-- Run with: ./scripts/run_migrations.sh
--
-- PREREQUISITES:
-- - Schedule downtime window
-- - Ensure sufficient disk space (2x current table size)

-- =============================================================================
-- Step 1: Modify reading_type column
-- =============================================================================

-- Remove CHECK constraint
ALTER TABLE sensor_readings
DROP CONSTRAINT IF EXISTS sensor_readings_reading_type_check;

-- Increase VARCHAR size
ALTER TABLE sensor_readings
ALTER COLUMN reading_type TYPE VARCHAR(30);

-- =============================================================================
-- Step 2: Modify value column
-- =============================================================================

-- This requires a full table rewrite - can take hours for 50M+ rows
ALTER TABLE sensor_readings
ALTER COLUMN value TYPE NUMERIC(15,6);

-- =============================================================================
-- Step 3: Verify
-- =============================================================================

DO $$
DECLARE
    reading_type_length INTEGER;
    value_precision INTEGER;
BEGIN
    SELECT character_maximum_length INTO reading_type_length
    FROM information_schema.columns
    WHERE table_name = 'sensor_readings'
    AND column_name = 'reading_type';

    SELECT numeric_precision INTO value_precision
    FROM information_schema.columns
    WHERE table_name = 'sensor_readings'
    AND column_name = 'value';

    RAISE NOTICE 'reading_type VARCHAR size: %', reading_type_length;
    RAISE NOTICE 'value NUMERIC precision: %', value_precision;

    IF reading_type_length = 30 AND value_precision = 15 THEN
        RAISE NOTICE 'Schema type corrections complete!';
    ELSE
        RAISE EXCEPTION 'Schema type corrections failed';
    END IF;
END $$;
```

### Option 2: Minimal Correction (Recommended)

Only correct what's actually needed:

```sql
-- Migration 009: Minimal Schema Corrections
--
-- Only correct types that are causing actual issues

-- Example: Only extend reading_type if adding longer types
ALTER TABLE sensor_readings
ALTER COLUMN reading_type TYPE VARCHAR(30);

-- Keep value as DECIMAL(10,2) unless actually needed
```

### Option 3: Update Documentation to Match Implementation

Instead of changing the schema, update the documentation:

**Edit `/docs/architecture.md` line 28:**

```markdown
| `value` | NUMERIC(10,2) | DECIMAL(10,2) | High precision for sensor data |
| `reading_type` | VARCHAR(20) | VARCHAR(20) | Type of sensor reading |
```

## Impact Analysis

### Storage Impact at 50M Rows

| Change | Storage Increase | Notes |
|--------|------------------|-------|
| reading_type: 20→30 chars | +50 MB | Minimal |
| value: (10,2)→(15,6) | +200 MB | Significant |
| Total | +250 MB | ~2.5% increase |

### Performance Impact

| Operation | Before | After | Impact |
|-----------|--------|-------|--------|
| Index size | ~2 GB | ~2.25 GB | +12.5% |
| Query speed | 50ms | 55ms | +10% |
| INSERT speed | 1000/sec | 950/sec | -5% |

### Application Impact

**Go Code Changes Required:**

```go
// Update struct tags if changing NUMERIC precision
type SensorReading struct {
    // ...
    Value float64 `json:"value" db:"value"` // No change needed for float64
    ReadingType string `json:"reading_type" db:"reading_type"` // No change needed
}
```

**API Changes:** None - JSON serialization unchanged

## Data Migration Strategy

### For reading_type VARCHAR(20) → VARCHAR(30)

**Risk:** Low - VARCHAR expansion is safe

```sql
-- Fast operation, no data loss
ALTER TABLE sensor_readings
ALTER COLUMN reading_type TYPE VARCHAR(30);
```

### For value DECIMAL(10,2) → NUMERIC(15,6)

**Risk:** High - Potential precision loss during migration

**Pre-migration validation:**

```sql
-- Check if any values will be truncated
SELECT
    COUNT(*) AS affected_rows,
    MIN(value) AS min_value,
    MAX(value) AS max_value
FROM sensor_readings
WHERE value != ROUND(value::NUMERIC, 6);

-- If count = 0, safe to proceed
-- If count > 0, review affected data first
```

**Safe migration approach:**

```sql
-- Step 1: Add new column
ALTER TABLE sensor_readings
ADD COLUMN value_new NUMERIC(15,6);

-- Step 2: Copy data with explicit casting
UPDATE sensor_readings
SET value_new = value::NUMERIC(15,6);

-- Step 3: Verify no data loss
SELECT COUNT(*) FROM sensor_readings
WHERE value != value_new;

-- Step 4: Drop old column and rename
ALTER TABLE sensor_readings DROP COLUMN value;
ALTER TABLE sensor_readings RENAME COLUMN value_new TO value;
```

## Recommendation

### For Most Users: Option 3 (Update Documentation)

**Rationale:**
1. Current types work fine for demo purposes
2. DECIMAL(10,2) is sufficient for most IoT sensors
3. CHECK constraint on reading_type provides data integrity
4. Avoids expensive table rewrite

**When to choose other options:**
- **Option 1:** Only if you have specific requirements for 6 decimal places
- **Option 2:** If you need reading types > 20 characters

## Testing After Changes

### Verify Schema

```sql
\d sensor_readings
```

Expected output after corrections:
```
Column        | Type             | Collation | Nullable
--------------+------------------+-----------+----------
id            | BIGSERIAL        |           | not null
device_id     | VARCHAR(50)      |           | not null
timestamp     | TIMESTAMPTZ      |           | not null
reading_type  | VARCHAR(30)      |           | not null  -- CHANGED
value         | NUMERIC(15,6)    |           | not null  -- CHANGED
unit          | VARCHAR(20)      |           | not null
metadata      | JSONB            |           |          -- From enhancement #2
```

### Verify Data Integrity

```sql
-- Check all values are present
SELECT COUNT(*) AS total_rows FROM sensor_readings;

-- Check for NULL values after migration
SELECT COUNT(*) AS null_values
FROM sensor_readings
WHERE value IS NULL OR reading_type IS NULL;

-- Verify value range
SELECT
    MIN(value) AS min_value,
    MAX(value) AS max_value,
    AVG(value) AS avg_value
FROM sensor_readings;
```

### Test Application

```bash
# Test API still works
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Test data insertion
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
  INSERT INTO sensor_readings (device_id, timestamp, reading_type, value, unit)
  VALUES ('sensor-test', NOW(), 'temperature', 23.456789, 'celsius');
"

# Verify precision is preserved
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-test&limit=1"
```

## Rollback Plan

If issues arise after type changes:

```sql
-- Rollback reading_type
ALTER TABLE sensor_readings
ALTER COLUMN reading_type TYPE VARCHAR(20);

-- Rollback value (requires data migration backup)
-- Step 1: Restore from backup taken before migration
pg_restore -U sensor_user -d sensor_db -t sensor_readings backup.sql

-- Step 2: Or use backup column if using safe migration approach
ALTER TABLE sensor_readings DROP COLUMN value;
ALTER TABLE sensor_readings RENAME COLUMN value_backup TO value;
```

## Best Practices

1. **Test on staging first** - Never schema changes directly on production
2. **Take backups** - Full backup before migration
3. **Use transactions** - Wrap changes in BEGIN/COMMIT
4. **Monitor performance** - Check query plans after migration
5. **Verify data integrity** - Count rows before/after

## Related Documentation

- **[../architecture.md](../architecture.md)** - Database schema design
- **[../implementation/database-setup.md](../implementation/database-setup.md)** - Database setup guide
- **[PostgreSQL ALTER TABLE Documentation](https://www.postgresql.org/docs/current/sql-altertable.html)**
