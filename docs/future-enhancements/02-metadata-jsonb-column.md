# Metadata JSONB Column

## Overview

This document describes how to add the `metadata` JSONB column to the `sensor_readings` table, enabling flexible storage of device-specific data such as firmware version, battery level, calibration data, and location information.

## Why Add Metadata JSONB?

The original design specified a `metadata` JSONB column, but it was not implemented in the base schema to keep the initial implementation simple. Adding this column provides:

| Benefit | Description |
|---------|-------------|
| **Flexibility** | Store device-specific data without schema changes |
| **Queryable** | JSONB supports indexing and querying within the JSON structure |
| **Efficient** | Binary format with decompression for fast access |
| **No Migrations** | Add new metadata fields without ALTER TABLE |

## Use Cases

### Device Firmware Information

```json
{
  "firmware_version": "2.1.0",
  "hardware_revision": "v1.2",
  "last_update": "2024-01-15T10:30:00Z"
}
```

### Battery and Power Status

```json
{
  "battery_level": 87,
  "battery_voltage": 3.7,
  "power_source": "battery",
  "low_battery_mode": false
}
```

### Sensor Calibration

```json
{
  "sensor_calibration_date": "2024-01-15",
  "calibration_offset": 0.5,
  "calibration_factor": 1.02,
  "last_calibration_by": "tech-001"
}
```

### Location Information

```json
{
  "location": {
    "building": "Building A",
    "floor": 3,
    "room": "301",
    "coordinates": {
      "latitude": 40.7128,
      "longitude": -74.0060
    }
  }
}
```

## Implementation

### Step 1: Create Migration

Create `scripts/schema/migrations/007_add_metadata_column.sql`:

```sql
-- Migration 007: Add Metadata JSONB Column
--
-- This migration adds a metadata JSONB column to store device-specific data.
--
-- Run with: ./scripts/run_migrations.sh

-- =============================================================================
-- Add metadata column
-- =============================================================================

ALTER TABLE sensor_readings
ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Add comment
COMMENT ON COLUMN sensor_readings.metadata IS
'Device-specific metadata such as firmware version, battery level, location, etc.';

-- =============================================================================
-- Add GIN index for JSONB queries
-- =============================================================================

CREATE INDEX IF NOT EXISTS idx_sensor_readings_metadata_gin
ON sensor_readings USING GIN (metadata);

COMMENT ON INDEX idx_sensor_readings_metadata_gin IS
'GIN index for efficient JSONB queries on metadata column.';

-- =============================================================================
-- Verification
-- =============================================================================

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sensor_readings'
        AND column_name = 'metadata'
    ) THEN
        RAISE NOTICE 'Metadata column added successfully';
    ELSE
        RAISE EXCEPTION 'Failed to add metadata column';
    END IF;
END $$;
```

### Step 2: Update Application Code

#### Update Go Struct

Edit `internal/model/sensor.go`:

```go
type SensorReading struct {
    ID          string      `json:"id" db:"id"`
    DeviceID    string      `json:"device_id" db:"device_id"`
    Timestamp   time.Time   `json:"timestamp" db:"timestamp"`
    ReadingType string      `json:"reading_type" db:"reading_type"`
    Value       float64     `json:"value" db:"value"`
    Unit        string      `json:"unit" db:"unit"`
    Metadata    interface{} `json:"metadata,omitempty" db:"metadata"` // NEW
}
```

#### Update Repository Query

Edit `internal/repository/sensor_repo.go`:

```go
const queryGetSensorReadings = `
    SELECT
        id,
        device_id,
        timestamp,
        reading_type,
        value,
        unit,
        metadata  -- ADD THIS
    FROM sensor_readings
    WHERE device_id = $1
    AND ($2 = '' OR reading_type = $2)
    ORDER BY timestamp DESC
    LIMIT $3
`
```

### Step 3: Update API Tests

Update test data to include metadata:

```bash
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
```

Expected response:
```json
{
  "data": [
    {
      "id": "12345678",
      "device_id": "sensor-001",
      "timestamp": "2025-01-15T10:30:00Z",
      "reading_type": "temperature",
      "value": 23.45,
      "unit": "celsius",
      "metadata": {
        "firmware_version": "2.1.0",
        "battery_level": 87
      }
    }
  ],
  "meta": {
    "count": 1,
    "limit": 10,
    "device_id": "sensor-001",
    "reading_type": null
  }
}
```

## Querying Metadata

### Query All Readings with Specific Firmware

```sql
SELECT * FROM sensor_readings
WHERE metadata->>'firmware_version' = '2.1.0';
```

### Query Low Battery Devices

```sql
SELECT device_id, COUNT(*)
FROM sensor_readings
WHERE (metadata->>'battery_level')::numeric < 20
GROUP BY device_id;
```

### Query by Location

```sql
SELECT * FROM sensor_readings
WHERE metadata->'location'->>'building' = 'Building A';
```

### Check if Metadata Field Exists

```sql
SELECT * FROM sensor_readings
WHERE metadata ? 'firmware_version';
```

## Data Migration Strategy

### Option 1: Start Empty (Recommended)

For new deployments, simply add the column and let new data include metadata:

```sql
-- New readings will include metadata
INSERT INTO sensor_readings (device_id, timestamp, reading_type, value, unit, metadata)
VALUES ('sensor-001', NOW(), 'temperature', 23.5, 'celsius', '{"firmware_version": "2.1.0"}'::jsonb);
```

### Option 2: Backfill Existing Data

For existing deployments, generate synthetic metadata:

```sql
-- Add metadata to existing rows
UPDATE sensor_readings
SET metadata = jsonb_build_object(
    'firmware_version', '2.' || (floor(random() * 10) + 1)::text || '.0',
    'battery_level', (floor(random() * 100) + 1)::int,
    'location', jsonb_build_object(
        'building', 'Building ' || char(65 + (random() * 6)::int)
    )
)
WHERE metadata IS NULL;
```

## Performance Considerations

### Storage Impact

| Dataset | Without Metadata | With Metadata | Increase |
|---------|-----------------|--------------|----------|
| 10M rows | ~2 GB | ~2.5 GB | +25% |
| 50M rows | ~10 GB | ~12.5 GB | +25% |
| 100M rows | ~20 GB | ~25 GB | +25% |

### Query Performance

| Query Type | Without Metadata | With Metadata (GIN) | Impact |
|------------|-----------------|---------------------|--------|
| Standard query | 50ms | 50ms | None |
| Metadata filter | N/A | 100-200ms | New capability |
| Covering index scan | 5-50ms | 5-50ms | None |

### Index Strategy

The GIN index on metadata enables fast JSONB queries but adds storage overhead:

```sql
-- GIN index size at 50M rows: ~5-10 GB
-- Trade-off: Query flexibility vs storage
```

## Backward Compatibility

### API Compatibility

- ✅ Old clients continue working (metadata is optional)
- ✅ New clients can use metadata
- ✅ JSON null if metadata not present

### Database Compatibility

- ✅ Existing queries unaffected (metadata is nullable)
- ✅ Existing indexes unchanged
- ✅ No data migration required

## Testing

### Unit Tests

```go
func TestSensorReadingWithMetadata(t *testing.T) {
    reading := SensorReading{
        DeviceID:    "sensor-001",
        ReadingType: "temperature",
        Value:       23.5,
        Unit:        "celsius",
        Metadata: map[string]interface{}{
            "firmware_version": "2.1.0",
            "battery_level":    87,
        },
    }

    // Test serialization
    data, _ := json.Marshal(reading)
    assert.Contains(t, string(data), "firmware_version")
}
```

### Integration Tests

```bash
# Test metadata is returned
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=1" | jq '.data[0].metadata'

# Test metadata queries
docker exec highth-postgres psql -U sensor_user -d sensor_db -c \
  "SELECT COUNT(*) FROM sensor_readings WHERE metadata ? 'firmware_version';"
```

## Rollback Plan

If needed, rollback is straightforward:

```sql
-- Drop GIN index
DROP INDEX IF EXISTS idx_sensor_readings_metadata_gin;

-- Drop column (data will be lost)
ALTER TABLE sensor_readings DROP COLUMN IF EXISTS metadata;
```

**Warning:** Dropping the column will permanently delete all metadata.

## Related Documentation

- **[../architecture.md](../architecture.md)** - Database schema design
- **[../api-spec.md](../api-spec.md)** - API response formats
- **[PostgreSQL JSONB Documentation](https://www.postgresql.org/docs/current/datatype-json.html)**
