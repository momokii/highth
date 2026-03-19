# Testing & Benchmarking Strategy

This document covers the complete testing methodology, including test scenarios, pass/fail criteria, and tooling recommendations.

## Table of Contents

- [Testing Philosophy](#testing-philosophy)
- [Test Scenarios](#test-scenarios)
- [Pass/Fail Criteria](#passfail-criteria)
- [Load Testing Tool: Vegeta](#load-testing-tool-vegeta)
- [Test Data Generation](#test-data-generation)
- [Executing Tests](#executing-tests)
- [Interpreting Results](#interpreting-results)
- [Performance Degradation Analysis](#performance-degradation-analysis)

---

## Testing Philosophy

This project takes a **realistic production testing** approach, not a raw benchmark mindset.

### Key Principles

1. **Simulate real-world conditions** — Tests must reflect actual production usage patterns
2. **Account for cache state** — Test both cold (cache miss) and warm (cache hit) scenarios
3. **Include non-uniform distribution** — Some devices queried more frequently than others
4. **Test under concurrent load** — Single-threaded performance doesn't tell the whole story
5. **Document honestly** — Report both successes and failures transparently

### What We're NOT Testing

| Aspect | Why Not Tested |
|--------|----------------|
| Raw database performance | We care about the full stack, not just PostgreSQL |
| Maximum theoretical throughput | Unrealistic; production has limits |
| Ideal conditions | Production is never ideal |
| Single-threaded only | Real users make concurrent requests |

---

## Test Scenarios

### Scenario 1: Baseline Single-Thread Query

**Purpose:** Establish the minimum latency floor for a single query.

**Description:**
- Single client making sequential requests
- Same device_id for all requests (cache warm)
- Limit = 10 records

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=1 | \
  vegeta report -type=text
```

**Expected Results:**
- p50: 5-20ms (cache hit)
- p95: 10-30ms

**Pass Criteria:**
- p50 ≤ 50ms
- 100% success rate

---

### Scenario 2: Concurrent Load Test

**Purpose:** Validate performance under realistic concurrent load.

**Description:**
- 50 concurrent clients
- Random device_id selection (from pool of 1000 devices)
- Limit = 10 records
- Duration = 60 seconds
- Cache warm-up period of 10 seconds

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-{0..999}&limit=10" | \
  vegeta attack -duration=60s -rate=50 -workers=50 | \
  vegeta report -type=text
```

**Expected Results:**
- p50: 50-200ms (mix of cache hits and misses)
- p95: 200-500ms

**Pass Criteria:**
- p50 ≤ 500ms
- p95 ≤ 800ms
- Error rate ≤ 1%

**Failure Analysis:**
- If p50 > 500ms: Check database indexing, connection pool size
- If p95 > 800ms: Check for hot keys, cache hit rate
- If error rate > 1%: Check database connections, timeout settings

---

### Scenario 3: Hot Device (Hot Key) Test

**Purpose:** Ensure that frequently-accessed devices don't cause outlier latency.

**Description:**
- 10 concurrent clients
- 90% of requests to same device_id (hot device with 1M+ rows)
- 10% of requests to random devices
- Duration = 60 seconds

**Command:**
```bash
# Create targets file with 90% hot device, 10% random
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-hot&limit=10" > targets.txt
for i in {1..100}; do
  echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-$((RANDOM % 1000))&limit=10" >> targets.txt
done

# Run attack with weighted targets
vegeta attack -targets=targets.txt -duration=60s -rate=10 -workers=10 | \
  vegeta report -type=text
```

**Expected Results:**
- p50: 50-150ms (hot device should be cached)
- p95: 100-300ms

**Pass Criteria:**
- No outlier > 2x p95
- p95 ≤ 600ms

**Failure Analysis:**
- Hot device outliers: Check cache configuration, consider longer TTL
- Overall high latency: Check database query plan for hot device

---

### Scenario 4: Cold Start Test

**Purpose:** Measure performance when cache is empty (e.g., after restart).

**Description:**
- Flush Redis cache before test
- 20 concurrent clients
- Random device_id selection
- No warm-up period
- Duration = 60 seconds

**Command:**
```bash
# Flush cache
redis-cli FLUSHALL

# Run attack immediately
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-{0..999}&limit=10" | \
  vegeta attack -duration=60s -rate=20 -workers=20 | \
  vegeta report -type=text
```

**Expected Results:**
- First requests: 200-600ms (cache misses)
- Later requests: 50-200ms (cache warming up)

**Pass Criteria:**
- p50 ≤ 600ms (higher than warm cache)
- No individual request > 2 seconds

**Failure Analysis:**
- Cold queries > 600ms: Check database indexing, query execution plan
- Very slow warm-up: Consider cache warm-up strategy

---

### Scenario 5: Large N Query Test

**Purpose:** Validate performance when requesting more records.

**Description:**
- 10 concurrent clients
- Same device_id (cache warm)
- Limit = 500 records (maximum allowed)
- Duration = 30 seconds

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=500" | \
  vegeta attack -duration=30s -rate=10 -workers=10 | \
  vegeta report -type=text
```

**Expected Results:**
- p50: 50-150ms (still cached)
- p95: 100-250ms

**Pass Criteria:**
- p50 ≤ 500ms
- Response size < 1MB

**Failure Analysis:**
- Large N causing high latency: Consider pagination for >100 records
- Response size issues: Implement field filtering or compression

---

### Scenario 6: Dataset Scale Test

**Purpose:** Document performance degradation as dataset grows.

**Description:**
Run concurrent load test at different dataset sizes:
- 10M rows
- 50M rows (primary target)
- 100M rows (stretch goal)

**Command (for 50M rows):**
```bash
# Ensure dataset has 50M rows
psql -c "SELECT count(*) FROM sensor_readings;"  # Should show 50000000

# Run standard concurrent test
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-{0..999}&limit=10" | \
  vegeta attack -duration=60s -rate=50 -workers=50 | \
  vegeta report -type=text > results_50m.txt
```

**Expected Degradation Pattern:**

| Dataset Size | Expected p50 | Expected p95 |
|--------------|--------------|--------------|
| 10M rows | 100-200ms | 300-500ms |
| 50M rows | 150-300ms | 400-700ms |
| 100M rows | 200-400ms | 500-900ms |

**Pass Criteria:**
- Linear or sub-linear degradation
- p50 ≤ 500ms at 50M rows
- p95 ≤ 1000ms at 100M rows

**Failure Analysis:**
- Super-linear degradation: Check index bloat, consider partitioning
- Very slow at 100M: Partitioning likely needed

---

## Pass/Fail Criteria

### Summary Table

| Metric | Pass | Warn | Fail |
|--------|------|------|------|
| **p50 latency** | ≤ 500ms | 500-700ms | > 700ms |
| **p95 latency** | ≤ 800ms | 800-1200ms | > 1200ms |
| **Error rate** | ≤ 1% | 1-5% | > 5% |
| **Cache hit rate** | ≥ 80% | 60-80% | < 60% |
| **Concurrent users** | 50+ | 30-50 | < 30 |

### Success Definition

The project **succeeds** if:
1. Concurrent load test passes (p50 ≤ 500ms, p95 ≤ 800ms)
2. Hot device test passes (no outliers > 2x p95)
3. Scale test shows acceptable degradation (p50 ≤ 500ms at 50M rows)
4. Error rate remains ≤ 1% across all tests

### Graceful Failure Definition

The project **fails gracefully** if:
1. Performance targets are missed but documented honestly
2. Root cause is identified (e.g., hardware limitation)
3. Mitigation strategies are proposed
4. Architecture principles remain sound

---

## Load Testing Tool: Vegeta

### Why Vegeta?

Vegeta was chosen for load testing because:
- Go-native (fits our ecosystem)
- Simple CLI interface
- Built-in percentile metrics (p50, p95, p99)
- Attack-based testing model (realistic)
- Multiple output formats (text, JSON, histogram)

### Installation

```bash
# Go install
go install github.com/tsenart/vegeta@latest

# Or download binary
wget https://github.com/tsenart/vegeta/releases/download/v12.11.0/vegeta_12.11.0_linux_amd64.tar.gz
tar -xvf vegeta_12.11.0_linux_amd64.tar.gz
sudo mv vegeta /usr/local/bin/
```

### Basic Usage

```bash
# Simple attack
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=10 | \
  vegeta report -type=text

# Save results for later analysis
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=10 | \
  tee results.bin | \
  vegeta report -type=text

# Generate histogram
vegeta report -inputs=results.bin -type=hist[0,50ms,100ms,500ms,1s]

# Generate JSON for programmatic analysis
vegeta report -inputs=results.bin -type=json > metrics.json
```

### Advanced Usage

```bash
# Custom headers
echo "GET http://localhost:8080/api/v1/sensor-readings" | \
  vegeta attack -duration=30s -rate=10 \
    -header="Authorization: Bearer token123" \
    -header="X-API-Key: key456"

# Max workers (concurrent connections)
vegeta attack -duration=30s -rate=100 -max-workers=200

# Attack from targets file
cat targets.txt | vegeta attack -duration=60s -rate=50

# Distributed testing (multiple instances)
# On instance 1:
echo "GET http://localhost:8080/api/v1/sensor-readings" | \
  vegeta attack -duration=60s -rate=50 | \
  tee results1.bin

# On instance 2:
echo "GET http://localhost:8080/api/v1/sensor-readings" | \
  vegeta attack -duration=60s -rate=50 | \
  tee results2.bin

# Combine results:
cat results1.bin results2.bin | vegeta report -type=text
```

---

## Test Data Generation

### Generating Realistic Test Data

For meaningful performance testing, we need realistic test data:

#### SQL Data Generation Script

```sql
-- Create 1000 devices
INSERT INTO devices (device_id)
SELECT 'sensor-' || LPAD(i::text, 4, '0') FROM generate_series(1, 1000) AS s(i);

-- Generate 50M readings with realistic distribution
-- Some devices have more readings than others (zipf distribution)
INSERT INTO sensor_readings (device_id, timestamp, reading_type, value, unit, metadata)
SELECT
    'sensor-' || LPAD((random() * 1000)::int::text, 4, '0') AS device_id,
    NOW() - (random() * interval '90 days') AS timestamp,
    (ARRAY['temperature', 'humidity', 'pressure', 'voltage'])[floor(random() * 4 + 1)] AS reading_type,
    round((random() * 100)::numeric, 2) AS value,
    CASE
        WHEN (ARRAY['temperature', 'humidity', 'pressure', 'voltage'])[floor(random() * 4 + 1)] = 'temperature' THEN 'celsius'
        WHEN (ARRAY['temperature', 'humidity', 'pressure', 'voltage'])[floor(random() * 4 + 1)] = 'humidity' THEN 'percent'
        WHEN (ARRAY['temperature', 'humidity', 'pressure', 'voltage'])[floor(random() * 4 + 1)] = 'pressure' THEN 'pascal'
        ELSE 'volt'
    END AS unit,
    jsonb_build_object(
        'firmware_version', '2.' || (floor(random() * 10) + 1)::text || '.0',
        'battery_level', (floor(random() * 100) + 1)::int,
        'location', jsonb_build_object('building', 'Building ' || char(65 + (random() * 6)::int))
    ) AS metadata
FROM generate_series(1, 50000000);
```

#### Go Data Generation Script

```go
package main

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    ctx := context.Background()
    pool, err := pgxpool.Connect(ctx, "postgres://localhost/sensor_db")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Device IDs
    deviceIDs := make([]string, 1000)
    for i := 0; i < 1000; i++ {
        deviceIDs[i] = fmt.Sprintf("sensor-%04d", i)
    }

    // Reading types
    readingTypes := []string{"temperature", "humidity", "pressure", "voltage"}

    // Batch insert
    batch := &pgx.Batch{}
    batchSize := 1000

    startTime := time.Now()
    for i := 0; i < 50000000; i++ {
        deviceID := deviceIDs[rand.Intn(1000)]
        timestamp := time.Now().Add(-time.Duration(rand.Intn(90*24*3600)) * time.Second)
        readingType := readingTypes[rand.Intn(4)]

        var value float64
        var unit string
        switch readingType {
        case "temperature":
            value = rand.Float64() * 50
            unit = "celsius"
        case "humidity":
            value = rand.Float64() * 100
            unit = "percent"
        case "pressure":
            value = rand.Float64() * 1000
            unit = "pascal"
        case "voltage":
            value = rand.Float64() * 5
            unit = "volt"
        }

        metadata := fmt.Sprintf(`{
            "firmware_version": "2.%d.0",
            "battery_level": %d,
            "location": {"building": "Building %c"}
        }`, rand.Intn(10)+1, rand.Intn(100)+1, 'A'+rand.Intn(6))

        query := `INSERT INTO sensor_readings
            (device_id, timestamp, reading_type, value, unit, metadata)
            VALUES ($1, $2, $3, $4, $5, $6::jsonb)`

        batch.Queue(query, deviceID, timestamp, readingType, value, unit, metadata)

        if i%batchSize == 0 {
            results := pool.SendBatch(ctx, batch)
            results.Close()
            batch = &pgx.Batch{}

            if i%100000 == 0 {
                elapsed := time.Since(startTime)
                rate := float64(i) / elapsed.Seconds()
                log.Printf("Inserted %d rows (%.0f rows/sec)", i, rate)
            }
        }
    }

    elapsed := time.Since(startTime)
    log.Printf("Completed: %d rows in %v (%.0f rows/sec)", 50000000, elapsed, float64(50000000)/elapsed.Seconds())
}
```

---

## Executing Tests

### Test Execution Order

Run tests in this order for consistent results:

1. **Health check** — Verify system is running
2. **Cold start test** — Measure baseline without cache
3. **Hot device test** — Warm cache with hot device
4. **Baseline single-thread** — Measure minimum latency
5. **Concurrent load test** — Primary performance validation
6. **Large N test** — Validate larger result sets
7. **Scale tests** — If dataset can be varied

### Pre-Test Checklist

- [ ] PostgreSQL is running and healthy
- [ ] Redis is running and healthy
- [ ] API is running and accessible
- [ ] Dataset is populated to target size
- [ ] Indexes are created and analyzed
- [ ] Connection pool is configured
- [ ] Vegeta is installed

### Test Script

```bash
#!/bin/bash

# Test execution script

API_BASE="http://localhost:8080"
RESULTS_DIR="./test-results/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo "Starting test run at $(date)" | tee "$RESULTS_DIR/test-run.log"

# Health check
echo "=== Health Check ===" | tee -a "$RESULTS_DIR/test-run.log"
curl -s "$API_BASE/health" | tee "$RESULTS_DIR/health.json" | tee -a "$RESULTS_DIR/test-run.log"
echo "" | tee -a "$RESULTS_DIR/test-run.log"

# Cold start test
echo "=== Cold Start Test ===" | tee -a "$RESULTS_DIR/test-run.log"
redis-cli FLUSHALL
echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-{0..999}&limit=10" | \
  vegeta attack -duration=60s -rate=20 -workers=20 | \
  tee "$RESULTS_DIR/cold-start.bin" | \
  vegeta report -type=text | tee "$RESULTS_DIR/cold-start.txt" | tee -a "$RESULTS_DIR/test-run.log"

# Baseline test
echo "=== Baseline Test ===" | tee -a "$RESULTS_DIR/test-run.log"
echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=1 | \
  tee "$RESULTS_DIR/baseline.bin" | \
  vegeta report -type=text | tee "$RESULTS_DIR/baseline.txt" | tee -a "$RESULTS_DIR/test-run.log"

# Concurrent load test
echo "=== Concurrent Load Test ===" | tee -a "$RESULTS_DIR/test-run.log"
echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-{0..999}&limit=10" | \
  vegeta attack -duration=60s -rate=50 -workers=50 | \
  tee "$RESULTS_DIR/concurrent.bin" | \
  vegeta report -type=text | tee "$RESULTS_DIR/concurrent.txt" | tee -a "$RESULTS_DIR/test-run.log"

# Hot device test
echo "=== Hot Device Test ===" | tee -a "$RESULTS_DIR/test-run.log"
for i in {1..90}; do
  echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-hot&limit=10"
done > "$RESULTS_DIR/hot-targets.txt"
for i in {1..10}; do
  echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-$((RANDOM % 1000))&limit=10"
done >> "$RESULTS_DIR/hot-targets.txt"
cat "$RESULTS_DIR/hot-targets.txt" | \
  vegeta attack -duration=60s -rate=10 -workers=10 | \
  tee "$RESULTS_DIR/hot-device.bin" | \
  vegeta report -type=text | tee "$RESULTS_DIR/hot-device.txt" | tee -a "$RESULTS_DIR/test-run.log"

# Large N test
echo "=== Large N Test ===" | tee -a "$RESULTS_DIR/test-run.log"
echo "GET $API_BASE/api/v1/sensor-readings?device_id=sensor-001&limit=500" | \
  vegeta attack -duration=30s -rate=10 -workers=10 | \
  tee "$RESULTS_DIR/large-n.bin" | \
  vegeta report -type=text | tee "$RESULTS_DIR/large-n.txt" | tee -a "$RESULTS_DIR/test-run.log"

echo "Test run completed at $(date)" | tee -a "$RESULTS_DIR/test-run.log"
echo "Results saved to: $RESULTS_DIR"
```

---

## Interpreting Results

### Understanding Vegeta Output

```
Requests      [total, rate]            3000, 100.10
Duration      [total, attack, wait]    30s, 29.97s, 29.18ms
Latencies     [mean, 50, 95, 99, max]  185ms, 167ms, 289ms, 401ms, 1.2s
Bytes In      [total, mean]            4500000, 1500.00
Bytes Out     [total, mean]            0, 0.00
Success       [ratio]                  100.00%
Status Codes  [code:count]             200:3000
Error Set:
```

**Key metrics:**
- **Requests [total]**: Total requests made
- **Requests [rate]**: Actual requests per second achieved
- **Duration [attack]**: Time spent sending requests
- **Duration [wait]**: Time waiting for responses
- **Latencies [50]**: p50 (median) latency
- **Latencies [95]**: p95 latency (95% of requests faster than this)
- **Latencies [99]**: p99 latency (99% of requests faster than this)
- **Success [ratio]**: Percentage of successful requests

### Histogram Analysis

```bash
vegeta report -inputs=results.bin -type=hist[0,10ms,50ms,100ms,200ms,500ms,1s,2s]
```

Output example:
```
Bucket           #       %       Histogram
[0ms, 10ms]      234     7.8%    ▃
[10ms, 50ms]     1456    48.5%   ████████████▊
[50ms, 100ms]    892     29.7%   ███████▏
[100ms, 200ms]   312     10.4%   ██▏
[200ms, 500ms]   96      3.2%    ▎
[500ms, 1s]      8       0.3%
[1s, 2s]         2       0.1%
```

**Interpretation:**
- Most requests (48.5%) complete in 10-50ms (cache hits)
- 86% of requests complete in <100ms
- Only 3.6% take >200ms (likely cache misses)

---

## Performance Degradation Analysis

### Expected Degradation Pattern

As dataset grows from 10M → 50M → 100M rows:

| Dataset | BRIN Size | B-tree Size | Expected p50 | Expected p95 |
|---------|-----------|-------------|--------------|--------------|
| 10M | ~5 MB | ~500 MB | 100-150ms | 300-500ms |
| 50M | ~25 MB | ~2.5 GB | 150-300ms | 400-700ms |
| 100M | ~50 MB | ~5 GB | 200-400ms | 500-900ms |

### When Degradation is Problematic

**Red flags:**
- Super-linear degradation (e.g., 5x slower for 2x data)
- p95 > 1200ms at target scale
- Increasing error rate with dataset size

**Typical causes:**
1. **Index bloat** — Run `REINDEX CONCURRENTLY`
2. **Insufficient work_mem** — Increase PostgreSQL work_mem
3. **Table bloat** — Run `VACUUM ANALYZE`
4. **Cache not effective** — Check cache hit rate

### Mitigation Strategies

| Problem | Mitigation |
|---------|------------|
| Index too large | Enable partitioning |
| Slow cold queries | Add more RAM to PostgreSQL |
| Cache misses | Increase TTL |
| Hot key contention | Shard hot device across multiple IDs |

---

## Related Documentation

- [architecture.md](architecture.md) — Database design and indexing strategy
- [stack.md](stack.md) — Technology stack details
- [api-spec.md](api-spec.md) — API contract for testing
