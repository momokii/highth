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

## Load Testing Tool: k6

### Why k6?

k6 was chosen for load testing because:
- Modern JavaScript-based testing (easy to write and maintain)
- Built-in percentile metrics (p50, p95, p99)
- Realistic scenario-based testing model
- Multiple output formats (text, JSON, HTML)
- Docker integration for consistent testing
- Active development and community support

> **Note:** The original design specified Vegeta, but the implementation uses k6 for better scenario management and Docker integration.

### Installation

```bash
# Using Docker (recommended)
docker pull grafana/k6:latest

# Or install directly
sudo gpg -k \
  https://dl.k6.io/rpm/repo.rpm.gpg \
  | sudo tee /etc/yum.repos.d/k6.repo
sudo yum install k6

# Or on macOS
brew install k6

# Or using Go
go install go.k6.io/k6@latest
```

### Basic Usage

```bash
# Run the benchmark test suite
./tests/run-benchmarks.sh

# Run specific scenario
./tests/run-benchmarks.sh --scenario hot

# Custom RPS and duration
./tests/run-benchmarks.sh --rps 100 --duration 5m

# Test remote API
./tests/run-benchmarks.sh --target-url https://api.example.com
```

### k6 Test Scenarios

The test suite includes six scenarios:

1. **Hot Device Pattern** (`01-hot-device-pattern.js`)
   - Simulates Zipf distribution (20% devices get 80% queries)
   - Tests cache effectiveness

2. **Time Range Queries** (`02-time-range-queries.js`)
   - Tests queries across different time ranges
   - Validates BRIN index performance

3. **Mixed Workload** (`03-mixed-workload.js`)
   - Realistic mix of query patterns
   - Tests system under normal load

4. **Cache Performance** (`04-cache-performance.js`)
   - Cold start vs warm cache
   - Cache hit rate measurement

5. **Stats and Aggregation** (`05-stats-and-aggregation.js`)
   - Tests materialized view queries
   - Validates MV performance under load

6. **Primary-Key Hot Lookup** (`06-pk-lookup.js`)
   - Single-row primary key B-tree index scan
   - Benchmarks raw PostgreSQL hot-path performance
   - Dynamic ID range detection via `/api/v1/stats` (total_readings == MAX(id) for BIGSERIAL)
   - Tightest latency thresholds in the suite (p95 < 100ms)

### Creating Custom Tests

```javascript
// custom-test.js
import http from 'k6/http';
import { check } from 'k6';

export let options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 50 },     // Ramp up to 50 users
    { duration: '30s', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% of requests under 500ms
    http_req_failed: ['rate<0.01'],     // Error rate < 1%
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

export default function () {
  const deviceId = `sensor-${Math.floor(Math.random() * 1000)}`;
  const response = http.get(
    `${BASE_URL}/api/v1/sensor-readings?device_id=${deviceId}&limit=10`
  );

  check(response, {
    'status is 200': (r) => r.status === 200,
    'has data': (r) => JSON.parse(r.body).data.length > 0,
  });
}
```

### Running Custom Tests

```bash
# Run with Docker
docker run --rm --network host \
  -v $(pwd):/tests \
  grafana/k6:latest run \
  --env TARGET_URL=http://localhost:8080 \
  /tests/custom-test.js

# Run locally
k6 run --env TARGET_URL=http://localhost:8080 custom-test.js

# With custom options
k6 run --env RPS=100 --env DURATION=5m custom-test.js
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
- [ ] k6 is available (Docker or installed locally)

### Test Script

**Use the provided benchmark runner:**

```bash
# Run all scenarios
./tests/run-benchmarks.sh

# Run specific scenario
./tests/run-benchmarks.sh --scenario hot

# Custom load and duration
./tests/run-benchmarks.sh --rps 100 --duration 5m

# Test remote API
./tests/run-benchmarks.sh --target-url https://api.example.com
```

**Or run k6 directly for custom tests:**

```bash
# Run custom test
k6 run custom-test.js

# With environment variables
k6 run --env TARGET_URL=http://localhost:8080 custom-test.js

# With Docker
docker run --rm --network host \
  -v $(pwd):/tests \
  grafana/k6:latest run /tests/custom-test.js
```

---

## Interpreting Results

### Understanding k6 Output

```
✓ status is 200
✓ has data

checks:
..................--------.--.

✓ status is 200 [ 95% ]
✗ has data     [ 80% ]

/data......................... .......... .......... .......... ......
/data......................... .......... .......... .......... ......
/data......................... .......... .......... .......... ......
data................... .......... .......... .......... .......... ..
data................... .......... .......... .......... .......... ..
data................... .......... .......... .......... .......... ..

✓ status is 200 [ 99% ]
✓ has data     [ 95% ]
```

**Key metrics:**
- **checks**: Pass/fail rates for assertions
- **http_req_duration**: Request latency distribution
- **http_req_failed**: Error rate
- **vus**: Virtual users (concurrent connections)

**Percentiles:**
- **p(50)**: Median latency (50% of requests)
- **p(95)**: 95th percentile (95% of requests faster than this)
- **p(99)**: 99th percentile (99% of requests faster than this)

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
