# Data Generation Strategy

This guide covers generating 50M rows of realistic IoT sensor test data with non-uniform identifier distribution.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Distribution Model](#distribution-model)
- [Generation Approaches](#generation-approaches)
- [Go Implementation](#go-implementation)
- [SQL Implementation](#sql-implementation)
- [Batch Strategy](#batch-strategy)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before generating data, ensure:

- [ ] Phase 1 (Database Setup) complete
- [ ] Database `sensor_db` exists and accessible
- [ ] Table `sensor_readings` created with schema
- [ ] All 3 indexes created
- [ ] At least 20GB free disk space

---

## Distribution Model

### Zipf-Like Distribution (Realistic Hot Keys)

In real IoT deployments, some devices are more active than others. We model this with a Zipf distribution where a small percentage of devices account for a large percentage of readings.

### Distribution Table

| Device Percentile | Devices | Readings/Device | Total Readings | % of Data |
|-------------------|---------|-----------------|----------------|-----------|
| Top 1% | 10 | 200,000 | 2,000,000 | 4% |
| Top 5% | 50 | 150,000 | 7,500,000 | 15% |
| Top 20% | 200 | 75,000 | 15,000,000 | 30% |
| Middle 40% | 400 | 40,000 | 16,000,000 | 32% |
| Bottom 40% | 400 | 12,500 | 5,000,000 | 10% |
| **Total** | **1,000** | **50,000 avg** | **50,000,000** | **100%** |

### Why This Distribution?

**Hot devices** (top 1-5%) represent:
- Critical infrastructure sensors (monitored more frequently)
- High-traffic area sensors (more activity to report)
- Faulty sensors (reporting errors more frequently)

**Cold devices** (bottom 40%) represent:
- Low-traffic area sensors
- Battery-powered sensors (reporting infrequently to save power)
- Decommissioned or testing devices

---

## Generation Approaches

### Approach 1: Go Script (Recommended)

**Pros:** Fast, fine-grained control, can monitor progress

**Cons:** Requires writing code

### Approach 2: SQL Script

**Pros:** No compilation needed, database-native

**Cons:** Less control over distribution, slower

### Approach 3: Hybrid (Go + SQL)

**Pros:** Leverages both strengths

**Cons:** More complex setup

---

## Go Implementation

### Complete Data Generation Script

```go
package main

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "os"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

var (
    deviceIDs     []string
    readingTypes  []string
    units         map[string]string
)

func init() {
    // Initialize device IDs (sensor-0000 to sensor-0999)
    deviceIDs = make([]string, 1000)
    for i := 0; i < 1000; i++ {
        deviceIDs[i] = fmt.Sprintf("sensor-%04d", i)
    }

    // Initialize reading types
    readingTypes = []string{"temperature", "humidity", "pressure", "voltage"}

    // Initialize units by reading type
    units = map[string]string{
        "temperature": "celsius",
        "humidity":    "percent",
        "pressure":     "pascal",
        "voltage":      "volt",
    }
}

// zipfDistribution returns a device index using Zipf distribution
// skew parameter controls how skewed the distribution is
func zipfDistribution(randSource *rand.Rand, n, skew int) int {
    // Simplified Zipf-like distribution
    // Higher probability of selecting lower indices
    r := randSource.Float64()
    return int(float64(n) * pow(1.01, -float64(randSource.Intn(skew*10))) * r)
}

func pow(base, exp float64) float64 {
    result := 1.0
    for i := 0; i < int(exp); i++ {
        result *= base
    }
    return result
}

func generateMetadata(deviceIndex int) string {
    firmware := fmt.Sprintf("2.%d.0", rand.Intn(10)+1)
    battery := rand.Intn(100) + 1
    building := string('A' + rand.Intn(6))

    return fmt.Sprintf(`{
        "firmware_version": "%s",
        "battery_level": %d,
        "location": {"building": "Building %c"}
    }`, firmware, battery, building)
}

func main() {
    // Configuration
    const totalRows = 50_000_000
    const batchSize = 1000
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        dbURL = "postgres://sensor_user:password@localhost:5432/sensor_db"
    }

    // Connect to database
    ctx := context.Background()
    pool, err := pgxpool.Connect(ctx, dbURL)
    if err != nil {
        log.Fatalf("Unable to connect to database: %v", err)
    }
    defer pool.Close()

    log.Println("Starting data generation...")
    log.Printf("Target: %d rows", totalRows)
    log.Printf("Batch size: %d", batchSize)

    // Create seeded random source for reproducibility
    source := rand.NewSource(time.Now().UnixNano())
    rng := rand.New(source)

    startTime := time.Now()
    var inserted int64 = 0

    // Disable autovacuum during load for performance
    _, err = pool.Exec(ctx, "SET autovacuum = off")
    if err != nil {
        log.Printf("Warning: Could not disable autovacuum: %v", err)
    }

    // Generation loop
    for i := 0; i < totalRows; i += batchSize {
        batchSize := min(batchSize, totalRows-i)
        batch := &pgx.Batch{}

        for j := 0; j < batchSize; j++ {
            // Select device with Zipf distribution
            deviceIdx := zipfDistribution(rng, 1000, 3)
            deviceID := deviceIDs[deviceIdx]

            // Random timestamp within last 90 days
            timestamp := time.Now().Add(-time.Duration(rng.Int63n(90*24*3600)))

            // Random reading type
            typeIdx := rng.Intn(len(readingTypes))
            readingType := readingTypes[typeIdx]
            unit := units[readingType]

            // Random value based on reading type
            var value float64
            switch readingType {
            case "temperature":
                value = rng.Float64() * 50  // 0-50°C
            case "humidity":
                value = rng.Float64() * 100 // 0-100%
            case "pressure":
                value = rng.Float64() * 1000  // 0-1000 Pa
            case "voltage":
                value = rng.Float64() * 5  // 0-5V
            }

            // Generate metadata
            metadata := generateMetadata(deviceIdx)

            // Queue insert
            query := `
                INSERT INTO sensor_readings
                    (device_id, timestamp, reading_type, value, unit, metadata)
                VALUES ($1, $2, $3, $4, $5, $6::jsonb)`

            batch.Queue(query, deviceID, timestamp, readingType, value, unit, metadata)
        }

        // Execute batch
        results := pool.SendBatch(ctx, batch)
        results.Close()

        inserted += int64(batchSize)

        // Progress reporting
        if inserted%100000 == 0 {
            elapsed := time.Since(startTime)
            rate := float64(inserted) / elapsed.Seconds()
            log.Printf("Inserted %d rows (%.1f rows/sec, %s elapsed)",
                inserted, rate, elapsed.Round(time.Second))
        }
    }

    // Final statistics
    elapsed := time.Since(startTime)
    finalRate := float64(totalRows) / elapsed.Seconds()

    log.Printf("\n=== Generation Complete ===")
    log.Printf("Total rows: %d", totalRows)
    log.Printf("Total time: %v", elapsed.Round(time.Millisecond))
    log.Printf("Final rate: %.0f rows/sec", finalRate)

    // Re-enable autovacuum
    _, err = pool.Exec(ctx, "SET autovacuum = on")
    if err != nil {
        log.Printf("Warning: Could not re-enable autovacuum: %v", err)
    }

    // Analyze table for query planner
    log.Println("Running ANALYZE...")
    _, err = pool.Exec(ctx, "ANALYZE sensor_readings")
    if err != nil {
        log.Printf("Warning: ANALYZE failed: %v", err)
    }

    // Verification
    var count int64
    err = pool.QueryRow(ctx, "SELECT count(*) FROM sensor_readings").Scan(&count)
    if err != nil {
        log.Printf("Warning: Could not verify row count: %v", err)
    } else {
        log.Printf("Verified row count: %d", count)
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

### How to Run

```bash
# Set database URL
export DATABASE_URL="postgres://sensor_user:password@localhost:5432/sensor_db"

# Run the script
go run data_generator.go

# Or build and run
go build -o data_generator data_generator.go
./data_generator
```

### Expected Output

```
2024/01/15 10:30:00 Starting data generation...
2024/01/15 10:30:00 Target: 50000000 rows
2024/01/15 10:30:00 Batch size: 1000
2024/01/15 10:30:30 Inserted 100000 rows (3421.3 rows/sec, 30s elapsed)
2024/01/15 10:31:00 Inserted 200000 rows (4567.8 rows/sec, 1m0s elapsed)
...
2024/01/15 10:45:00 Inserted 50000000 rows (27845.2 rows/sec, 15m0s elapsed)

=== Generation Complete ===
Total rows: 50000000
Total time: 15m0s
Final rate: 27845 rows/sec
Verified row count: 50000000
```

---

## SQL Implementation

### Alternative: Pure SQL (PostgreSQL)

```sql
-- Generate data using generate_series and random()
-- Slower but doesn't require Go compilation

DO $$
DECLARE
    v_device_id TEXT;
    v_timestamp TIMESTAMPTZ;
    v_reading_type TEXT;
    v_value NUMERIC;
    v_unit TEXT;
    v_metadata JSONB;
    v_type_array TEXT[] := ARRAY['temperature', 'humidity', 'pressure', 'voltage'];
    v_unit_array TEXT[] := ARRAY['celsius', 'percent', 'pascal', 'volt'];
    v_device_start INT := 0;
    v_device_end INT := 1000;
BEGIN
    -- Generate 50M rows with skewed distribution
    INSERT INTO sensor_readings (device_id, timestamp, reading_type, value, unit, metadata)
    SELECT
        'sensor-' || LPAD((v_device_start + (random() * (v_device_end - v_device_start))::int)::text, 4, '0') AS device_id,
        NOW() - (random() * interval '90 days') AS timestamp,
        v_type_array[1 + (random() * 3)::int] AS reading_type,
        CASE
            WHEN v_type_array[1 + (random() * 3)::int] = 'temperature' THEN (random() * 50)::numeric(15,6)
            WHEN v_type_array[1 + (random() * 3)::int] = 'humidity' THEN (random() * 100)::numeric(15,6)
            WHEN v_type_array[1 + (random() * 3)::int] = 'pressure' THEN (random() * 1000)::numeric(15,6)
            ELSE (random() * 5)::numeric(15,6)
        END AS value,
        v_unit_array[1 + (random() * 3)::int] AS unit,
        jsonb_build_object(
            'firmware_version', '2.' || (1 + (random() * 9)::int)::text || '.0',
            'battery_level', (random() * 100 + 1)::int,
            'location', jsonb_build_object('building', 'Building ' || chr(65 + (random() * 6)::int))
        ) AS metadata
    FROM generate_series(1, 50000000);
END;
$$;
```

### Note

The SQL approach is simpler but:
- Requires superuser privileges for `EXECUTE`
- Less control over distribution
- Single transaction (may exceed memory limits)
- Slower than Go batch inserts

---

## Batch Strategy

### Why Batch Inserts?

Single-row inserts are too slow at scale:

```go
// SLOW: One INSERT per row (50M round trips)
for i := 0; i < 50000000; i++ {
    db.Exec("INSERT INTO sensor_readings ...")
}
// Time: Several hours to days

// FAST: Batch inserts (50K round trips)
for i := 0; i < 50000000; i += 1000 {
    batch.Queue("INSERT INTO sensor_readings ...", ...)
    db.SendBatch(batch)
}
// Time: 20-60 minutes
```

### Optimal Batch Size

| Batch Size | Rows/sec | Memory Usage | Recommendation |
|------------|----------|--------------|----------------|
| 100 | ~15,000 | Low | Too small |
| 500 | ~25,000 | Low | Acceptable |
| **1,000** | **~30,000** | **Low** | **Recommended** |
| 2,000 | ~25,000 | Medium | No improvement |
| 5,000 | ~20,000 | High | Too large |

### Disable Autovacuum During Load

```sql
-- Before starting
SET autovacuum = off;

-- After data generation
SET autovovacuum = on;
VACUUM ANALYZE sensor_readings;
```

---

## Verification

### Row Count Check

```sql
SELECT count(*) FROM sensor_readings;
-- Expected: 50000000
```

### Distribution Verification

```sql
-- Check distribution across devices
SELECT
    percentile_cont(0.50) WITHIN GROUP (ORDER BY reading_count) AS p50_reads_per_device,
    percentile_cont(0.95) WITHIN GROUP (ORDER BY reading_count) AS p95_reads_per_device,
    percentile_cont(0.99) WITHIN GROUP (ORDER BY reading_count) AS p99_reads_per_device,
    max(reading_count) AS max_reads_per_device,
    min(reading_count) AS min_reads_per_device,
    avg(reading_count) AS avg_reads_per_device
FROM (
    SELECT device_id, count(*) AS reading_count
    FROM sensor_readings
    GROUP BY device_id
) counts;

-- Expected approximate values:
-- p50_reads_per_device: ~40,000
-- p95_reads_per_device: ~150,000
-- p99_reads_per_device: ~200,000
-- max_reads_per_device: ~200,000+
-- min_reads_per_device: ~5,000
-- avg_reads_per_device: ~50,000
```

### Time Range Verification

```sql
-- Check time range of data
SELECT
    min(timestamp) as earliest_reading,
    max(timestamp) as latest_reading,
    max(timestamp) - min(timestamp) as time_span
FROM sensor_readings;

-- Expected:
-- earliest_reading: ~90 days ago
-- latest_reading: now
-- time_span: ~90 days
```

### Index Size Verification

```sql
-- Check index sizes
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size,
    indexdef
FROM pg_indexes
JOIN pg_class ON pg_class.oid = indexrelid
WHERE indrelid = 'sensor_readings'::regclass
ORDER BY pg_relation_size(indexrelid) DESC;
```

---

## Troubleshooting

### Out of Memory

**Problem:** Process runs out of memory during generation

**Solutions:**
```bash
# Reduce batch size
# Change batchSize from 1000 to 500

# Go: Set GOMEMLIMIT
export GOMEMLIMIT=512MiB

# PostgreSQL: Reduce work_mem
SET work_mem = '8MB';
```

### Disk Space Issues

**Problem:** Not enough disk space

**Solutions:**
```bash
# Check available space
df -h

# Monitor space during generation
watch -n 5 'df -h | grep /dev/sda'

# Stop generation if needed (Ctrl+C)
# Consider generating smaller dataset (10M instead of 50M)
```

### Slow Generation

**Problem:** Generation taking too long

**Solutions:**
```bash
# Check if autovacuum is running (slows down inserts)
SELECT * FROM pg_stat_activity WHERE query LIKE '%autovacuum%';

# Use SSD instead of HDD
# Increase connection pool size
# Reduce batch size slightly
```

### Wrong Distribution

**Problem:** Distribution doesn't look Zipf-like

**Solutions:**
```sql
-- Check distribution
SELECT count(*) FROM sensor_readings GROUP BY device_id ORDER BY count(*) DESC LIMIT 10;

-- If distribution is too uniform, adjust the zipfDistribution function
-- to increase the skew parameter
```

---

## Performance Estimates

### Hardware-Specific Estimates

| CPU | RAM | Storage | Estimated Time |
|-----|-----|---------|----------------|
| 4 cores | 8GB | HDD | Not recommended |
| 4 cores | 16GB | SSD | 1-2 hours |
| 8 cores | 16GB | SSD | 30-60 minutes |
| 8 cores | 32GB | NVMe | 20-40 minutes |

### Factors Affecting Speed

1. **Storage type** — SSD is required; HDD will be 10x slower
2. **CPU cores** — More cores help with parallel processing
3. **RAM** - Sufficient RAM prevents swapping
4. **Database tuning** — Proper configuration helps
5. **Network** - Local database is fastest; remote adds latency

---

## Next Steps

After data generation is complete:

1. Verify row count with `SELECT count(*) FROM sensor_readings;`
2. Run `ANALYZE sensor_readings;` for query planner
3. Verify index sizes with the query in [Verification](#verification)
4. **[api-development.md](api-development.md)** — Build the Go API

---

## Related Documentation

- **[../architecture.md](../architecture.md)** — Schema design rationale
- **[database-setup.md](database-setup.md)** — Database provisioning
- **[../testing.md](../testing.md)** — Test methodology
