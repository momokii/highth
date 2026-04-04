# Load Testing Setup Guide

This guide covers executing all 6 test scenarios using Vegeta, collecting performance metrics, and analyzing results against the ≤500ms target.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Vegeta Overview](#vegeta-overview)
- [Test Execution Order](#test-execution-order)
- [Test Scenarios](#test-scenarios)
- [Results Collection](#results-collection)
- [Pass/Fail Evaluation](#passthrough-fail-evaluation)
- [Performance Analysis](#performance-analysis)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting load testing, ensure:

- [ ] Phase 4 (Caching Integration) complete
- [ ] Full system running (API + Database + Redis)
- [ ] Dataset at target scale (50M rows in `sensor_readings`)
- [ ] Vegeta installed (`vegeta --version` works)
- [ ] At least 1GB free RAM for load testing
- [ ] API server running on port 8080
- [ ] Health check returns 200

---

## Vegeta Overview

### What is Vegeta?

Vegeta is a versatile HTTP load testing tool written in Go. It's chosen for this project because:

- **Go-based** — Fits the ecosystem
- **Attack-based testing** — Realistic load simulation
- **Built-in metrics** — p50, p95, p99 latency percentiles
- **Simple CLI** — Easy to integrate into CI/CD
- **Multiple output formats** — Text, JSON, HTML, Prometheus

### Installation

Vegeta installation is covered in **[dev-environment.md](dev-environment.md)**. Briefly:

```bash
# Via Go install
go install github.com/tsenart/vegeta@latest
export PATH=$PATH:$(go env GOPATH)/bin

# Verify
vegeta --version
```

### Basic Usage Pattern

```bash
# Create attack file (list of HTTP requests)
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=10 | \
  vegeta report -type=text

# Components:
# 1. Attack file: List of HTTP requests (one per line)
# 2. vegeta attack: Executes load test
#    -duration=30s: Run for 30 seconds
#    -rate=10: 10 requests per second
# 3. vegeta report: Generates results
#    -type=text: Human-readable text output
```

### Output Format

```
Requests      [total, rate, throughput]
  500000.00   50000.00  50000.00  # Total requests, rate, actual throughput
Success       [ratio]                    # Success rate
  100.00%
Latencies     [mean, 50, 95, 99, max]
    150ms    120ms   300ms   500ms    2s  # Mean, p50, p95, p99, max
Bytes In      [total, mean]
    125000000     250.00               # Total bytes received, mean per request
Bytes Out     [total, mean]
     15000000       30.00               # Total bytes sent, mean per request
Status Codes  [code:count]
  200:500000                               # HTTP 200: 500,000 responses
```

---

## Test Execution Order

### Execution Sequence

Tests should be executed in this order:

```
1. Health Check Test         → System sanity verification
2. Cold Start Test           → Baseline without cache
3. Baseline Test             → Single-thread minimum latency
4. Concurrent Load Test      → PRIMARY VALIDATION (50 users)
5. Hot Device Test           → Hot key scenario
6. Large N Test              → Request 500 records
7. PK Lookup Test            → Single reading by ID (B-tree index scan
```

### Why This Order?

| Order | Reason |
|-------|--------|
| Health first | Verify system is running before generating load |
| Cold start next | Measure baseline before cache warms up |
| Baseline third | Measure single-thread latency floor |
| Concurrent fourth | PRIMARY TEST after cache is warm |
| Hot device fifth | Test hot key scenario with warm cache |
| Large N last | Test worst-case result set size |

### Between Tests

```bash
# Optional: Flush Redis between tests to reset cache state
redis-cli FLUSHALL

# Optional: Wait 5 seconds for system to stabilize
sleep 5
```

---

## Test Scenarios

### Test 1: Health Check Test

**Purpose:** Verify system is running and healthy

**Command:**
```bash
curl http://localhost:8080/health
```

**Expected Output:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": { "status": "healthy", "latency": "5ms" },
    "redis": { "status": "healthy", "latency": "2ms" },
    "connection_pool": { "status": "healthy" }
  }
}
```

**Pass Criteria:**
- HTTP 200 status
- `status: "healthy"`
- Database and Redis both healthy

---

### Test 2: Cold Start Test

**Purpose:** Measure latency with no cache (baseline cold performance)

**Setup:**
```bash
# Flush Redis to ensure cold start
redis-cli FLUSHALL

# Verify cache is empty
redis-cli DBSIZE
# Expected: 0
```

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text
```

**Parameters:**
- Duration: 10 seconds
- Rate: 1 request/second (single-thread)
- Device: `sensor-001` (should exist in database)

**Expected Results:**
```
Requests      [total, rate, throughput]
         10         1.00         1.00
Success       [ratio]
  100.00%
Latencies     [mean, 50, 95, 99, max]
    350ms    300ms    500ms    600ms    800ms
```

**Pass Criteria:**
- p50 ≤ 600ms
- p95 ≤ 1000ms
- Success rate = 100%

**What This Tests:**
- Database query performance (no cache)
- Index effectiveness
- Connection pool warm-up

---

### Test 3: Baseline Test

**Purpose:** Measure minimum latency floor with warm cache

**Setup:**
```bash
# First, warm up the cache by making a few requests
for i in {1..5}; do
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
done

# Verify cache is populated
redis-cli DBSIZE
# Expected: 1 (or more) keys
```

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text
```

**Parameters:**
- Duration: 10 seconds
- Rate: 1 request/second (single-thread)
- Device: `sensor-001` (cache should be warm)

**Expected Results:**
```
Requests      [total, rate, throughput]
         10         1.00         1.00
Success       [ratio]
  100.00%
Latencies     [mean, 50, 95, 99, max]
     15ms     10ms     25ms     50ms    100ms
```

**Pass Criteria:**
- p50 ≤ 50ms
- p95 ≤ 100ms
- Success rate = 100%

**What This Tests:**
- Cache hit performance
- Minimum achievable latency
- Network overhead

---

### Test 4: Concurrent Load Test (PRIMARY VALIDATION)

**Purpose:** Validate performance under target load (50 concurrent users)

**Setup:**
```bash
# Ensure cache is warm
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
```

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=60s -rate=50 | \
  tee results/concurrent.bin | \
  vegeta report -type=text
```

**Parameters:**
- Duration: 60 seconds
- Rate: 50 requests/second (simulating 50 concurrent users)
- Device: `sensor-001` (cache warm)

**Expected Results:**
```
Requests      [total, rate, throughput]
       3000        50.00        50.00
Success       [ratio]
  100.00%
Latencies     [mean, 50, 95, 99, max]
    250ms    200ms    400ms    600ms    1.2s
Bytes In      [total, mean]
     750000     250.00
Bytes Out     [total, mean]
      90000       30.00
Status Codes  [code:count]
  200:3000
```

**Pass Criteria:**
- **p50 ≤ 500ms** (PRIMARY TARGET)
- **p95 ≤ 800ms** (PRIMARY TARGET)
- Success rate ≥ 99%
- Error rate ≤ 1%

**What This Tests:**
- Primary performance validation
- Connection pool under load
- Cache effectiveness at scale
- System stability

**Why 50 Requests/Second?**
- Represents ~50 concurrent users (assuming 1 request per second per user)
- Realistic load for IoT monitoring dashboard
- Stress-tests connection pool (max 25 connections)

---

### Test 5: Hot Device Test

**Purpose:** Test hot key scenario (90% queries to same device)

**Setup:**
```bash
# Create attack file with weighted distribution
cat > targets-hot.txt << 'EOF'
# 90% of requests to sensor-001 (hot device)
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-002&limit=10
EOF
```

**Command:**
```bash
cat targets-hot.txt | \
  vegeta attack -duration=60s -rate=50 | \
  vegeta report -type=text
```

**Parameters:**
- Duration: 60 seconds
- Rate: 50 requests/second
- Distribution: 90% to `sensor-001`, 10% to `sensor-002`

**Expected Results:**
```
Requests      [total, rate, throughput]
       3000        50.00        50.00
Success       [ratio]
  100.00%
Latencies     [mean, 50, 95, 99, max]
    220ms    180ms    350ms    500ms    900ms
```

**Pass Criteria:**
- p50 ≤ 500ms
- p95 ≤ 600ms (stricter than concurrent test)
- No outliers >2x p95
- Success rate = 100%

**What This Tests:**
- Hot key performance
- Cache effectiveness for popular device
- No connection pool exhaustion

**Why This Matters:**
- In production, some devices are monitored more frequently
- Hot keys should NOT cause performance degradation
- Cache should handle hot keys well

---

### Test 6: Large N Test

**Purpose:** Test worst-case result set size (500 records)

**Command:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=500" | \
  vegeta attack -duration=30s -rate=10 | \
  vegeta report -type=text
```

**Parameters:**
- Duration: 30 seconds
- Rate: 10 requests/second
- Limit: 500 records (maximum allowed)

**Expected Results:**
```
Requests      [total, rate, throughput]
        300        10.00        10.00
Success       [ratio]
  100.00%
Latencies     [mean, 50, 95, 99, max]
    300ms    250ms    450ms    600ms    900ms
Bytes In      [total, mean]
    3750000   12500.00
Bytes Out     [total, mean]
       9000       30.00
```

**Pass Criteria:**
- p50 ≤ 500ms
- p95 ≤ 800ms
- Success rate = 100%

**What This Tests:**
- Large result set handling
- JSON serialization performance
- Memory usage for large responses
- Network bandwidth for large payloads

**Why 500 Records?**
- Maximum allowed by API limit parameter
- Worst-case for response size
- Tests upper bound of performance

---

## Results Collection

### Automated Test Runner

Create `scripts/test-runner.sh`:

```bash
#!/bin/bash
# scripts/test-runner.sh
# Automated load testing script

set -e  # Exit on error

API_BASE="${API_BASE:-http://localhost:8080}"
RESULTS_DIR="./test-results/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo "=== Load Testing ==="
echo "API Base: $API_BASE"
echo "Results: $RESULTS_DIR"
echo ""

# Color codes
PASS='\033[0;32m'
FAIL='\033[0;31m'
NC='\033[0m' # No Color

# Test counter
PASS_COUNT=0
FAIL_COUNT=0

# Function to run test and check results
run_test() {
    local test_name=$1
    local command=$2
    local p50_target=$3
    local p95_target=$4

    echo "Running: $test_name"
    echo "Command: $command"

    # Run test and capture output
    eval "$command" > "$RESULTS_DIR/${test_name}.txt" 2>&1

    # Parse results
    p50=$(grep "Latencies" "$RESULTS_DIR/${test_name}.txt" | awk '{print $3}' | sed 's/ms//')
    p95=$(grep "Latencies" "$RESULTS_DIR/${test_name}.txt" | awk '{print $4}' | sed 's/ms//')

    # Extract numeric values (handle cases like "200ms" or "200")
    p50_num=$(echo "$p50" | sed 's/[^0-9.]//g')
    p95_num=$(echo "$p95" | sed 's/[^0-9.]//g')

    echo "  p50: ${p50}ms (target: ≤${p50_target}ms)"
    echo "  p95: ${p95}ms (target: ≤${p95_target}ms)"

    # Check pass/fail
    p50_pass=$(echo "$p50_num <= $p50_target" | bc -l)
    p95_pass=$(echo "$p95_num <= $p95_target" | bc -l)

    if [ "$p50_pass" -eq 1 ] && [ "$p95_pass" -eq 1 ]; then
        echo -e "  ${PASS}PASS${NC}"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo -e "  ${FAIL}FAIL${NC}"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    echo ""
}

# 1. Health check
echo "1. Health Check"
if curl -sf "$API_BASE/health" > "$RESULTS_DIR/health.json"; then
    echo -e "  ${PASS}PASS${NC}"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo -e "  ${FAIL}FAIL${NC}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi
echo ""

# 2. Cold start test
redis-cli FLUSHALL > /dev/null 2>&1
sleep 2
run_test "cold_start" \
    'echo "GET '$API_BASE'/api/v1/sensor-readings?device_id=sensor-001&limit=10" | vegeta attack -duration=10s -rate=1 | vegeta report -type=text' \
    600 1000

# 3. Baseline test (warm cache)
curl -s "$API_BASE/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
sleep 1
run_test "baseline" \
    'echo "GET '$API_BASE'/api/v1/sensor-readings?device_id=sensor-001&limit=10" | vegeta attack -duration=10s -rate=1 | vegeta report -type=text' \
    50 100

# 4. Concurrent load test (PRIMARY)
run_test "concurrent" \
    'echo "GET '$API_BASE'/api/v1/sensor-readings?device_id=sensor-001&limit=10" | vegeta attack -duration=60s -rate=50 | vegeta report -type=text' \
    500 800

# 5. Hot device test
cat > /tmp/targets-hot.txt << 'EOF'
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10
GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-002&limit=10
EOF
run_test "hot_device" \
    'cat /tmp/targets-hot.txt | vegeta attack -duration=60s -rate=50 | vegeta report -type=text' \
    500 600

# 6. Large N test
run_test "large_n" \
    'echo "GET '$API_BASE'/api/v1/sensor-readings?device_id=sensor-001&limit=500" | vegeta attack -duration=30s -rate=10 | vegeta report -type=text' \
    500 800

# Summary
echo "=== Test Summary ==="
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"
echo "Results saved to: $RESULTS_DIR"

if [ $FAIL_COUNT -eq 0 ]; then
    echo -e "${PASS}All tests passed!${NC}"
    exit 0
else
    echo -e "${FAIL}Some tests failed${NC}"
    exit 1
fi
```

### Make Script Executable

```bash
chmod +x scripts/test-runner.sh
```

### Running All Tests

```bash
# Run all tests
./scripts/test-runner.sh

# Expected output:
# === Load Testing ===
# API Base: http://localhost:8080
# Results: ./test-results/20240115-103000
#
# 1. Health Check
#   PASS
#
# Running: cold_start
#   p50: 350ms (target: ≤600ms)
#   p95: 500ms (target: ≤1000ms)
#   PASS
#
# Running: baseline
#   p50: 15ms (target: ≤50ms)
#   p95: 25ms (target: ≤100ms)
#   PASS
#
# Running: concurrent
#   p50: 250ms (target: ≤500ms)
#   p95: 400ms (target: ≤800ms)
#   PASS
#
# Running: hot_device
#   p50: 220ms (target: ≤500ms)
#   p95: 350ms (target: ≤600ms)
#   PASS
#
# Running: large_n
#   p50: 300ms (target: ≤500ms)
#   p95: 450ms (target: ≤800ms)
#   PASS
#
# === Test Summary ===
# Passed: 6
# Failed: 0
# Results saved to: ./test-results/20240115-103000
# All tests passed!
```

---

## Pass/Fail Evaluation

### Pass Criteria Summary

| Test | p50 Target | p95 Target | Notes |
|------|------------|------------|-------|
| Health Check | N/A | N/A | HTTP 200, status=healthy |
| Cold Start | ≤600ms | ≤1000ms | No cache, higher tolerance |
| Baseline | ≤50ms | ≤100ms | Warm cache, minimum latency |
| **Concurrent** | **≤500ms** | **≤800ms** | **PRIMARY VALIDATION** |
| Hot Device | ≤500ms | ≤600ms | No outliers >2x p95 |
| Large N | ≤500ms | ≤800ms | 500 records |

### Primary Success Criteria

**Project succeeds if:**
1. ✅ Concurrent test passes (p50 ≤ 500ms, p95 ≤ 800ms)
2. ✅ Hot device test passes (no significant outliers)
3. ✅ Error rate ≤ 1% across all tests

**Project passes with notes if:**
1. ⚠️ Targets missed but documented honestly
2. ⚠️ Root cause identified
3. ⚠️ Architecture principles remain sound

**Project fails if:**
1. ❌ Error rate > 5% (system instability)
2. ❌ p50 > 1000ms (unacceptable performance)
3. ❌ System crashes under load

### Sample Results Table

| Metric | Expected | Actual | Status |
|--------|----------|--------|--------|
| p50 (concurrent) | ≤500ms | 250ms | ✅ Pass |
| p95 (concurrent) | ≤800ms | 400ms | ✅ Pass |
| p50 (cold) | ≤600ms | 350ms | ✅ Pass |
| p95 (cold) | ≤1000ms | 500ms | ✅ Pass |
| p50 (baseline) | ≤50ms | 15ms | ✅ Pass |
| p95 (baseline) | ≤100ms | 25ms | ✅ Pass |
| Cache hit rate | ≥80% | 85% | ✅ Pass |
| Error rate | ≤1% | 0% | ✅ Pass |

---

## Performance Analysis

### Interpreting Results

#### Latency Distribution

```
Latencies     [mean, 50, 95, 99, max]
    250ms    200ms    400ms    600ms    1.2s
             ↑       ↑        ↑
             p50     p95      p99
```

- **p50 (median)**: 50% of requests complete in this time or less
- **p95**: 95% of requests complete in this time or less
- **p99**: 99% of requests complete in this time or less
- **max**: Worst-case observed latency

#### Success Rate

```
Success       [ratio]
  100.00%
```

- 100% = All requests succeeded
- < 100% = Some requests failed (timeout, error, etc.)

#### Throughput

```
Requests      [total, rate, throughput]
       3000        50.00        50.00
              ↑         ↑
              total     actual RPS
```

- **rate**: Target requests per second
- **throughput**: Actual requests per second (should match rate)
- If throughput < rate, system is saturated

### Common Patterns

| Pattern | Cause | Action |
|---------|-------|--------|
| p50 high, p95 close to p50 | Consistently slow | Check database query, verify index usage |
| p50 low, p95 very high | Occasional outliers | Check cache misses, hot keys, connection pool |
| p99 >> p95 | Tail latency | Investigate garbage collection, network issues |
| Throughput < rate | System saturated | Increase resources, optimize queries |
| Success rate < 100% | Errors | Check logs, verify error handling |

### Performance Degradation

If testing multiple scales (10M, 50M, 100M rows):

| Rows | p50 | p95 | Degradation |
|------|-----|-----|-------------|
| 10M | 150ms | 250ms | Baseline |
| 50M | 250ms | 400ms | +67% p50, +60% p95 |
| 100M | 400ms | 700ms | +167% p50, +180% p95 |

**Expected degradation:**
- 10M → 50M: ~50-100% degradation (acceptable)
- 50M → 100M: ~50-100% degradation (acceptable)
- >2x degradation indicates indexing issue

---

## Troubleshooting

### API Crashes Under Load

**Problem:** API stops responding during test

**Investigation:**
```bash
# Check API logs
journalctl -u your-api-service -f  # systemd
docker logs api-container            # Docker

# Check for panics
grep "panic" /var/log/api.log

# Check connection pool exhaustion
psql "$DATABASE_URL" -c "
SELECT count(*) FROM pg_stat_activity
WHERE datname = 'sensor_db';
"
```

**Solutions:**
- Increase `DB_MAX_CONNECTIONS` in `.env`
- Check for goroutine leaks
- Add connection timeout to requests
- Monitor memory usage

### Slow Performance

**Problem:** Latency higher than expected

**Investigation:**
```bash
# Check if cache is working
redis-cli DBSIZE
redis-cli GET "sensor:sensor-001:readings:10"

# Check database query plan
psql "$DATABASE_URL" << 'EOF'
EXPLAIN ANALYZE
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;
EOF

# Look for: Index Only Scan using idx_sensor_readings_device_covering

# Check system resources
top
htop
iostat -x 1
```

**Solutions:**
- Verify covering index is being used
- Run `ANALYZE sensor_readings`
- Check if cache is being populated
- Verify connection pool isn't exhausted
- Check system resources (CPU, RAM, disk I/O)

### High Error Rate

**Problem:** More than 1% of requests failing

**Investigation:**
```bash
# Check Vegeta output for status codes
grep "Status Codes" results/*.txt

# Expected:
# Status Codes  [code:count]
#   200:3000

# If seeing 500 errors:
# Check API logs for errors

# If seeing timeouts:
# Increase timeout in Vegeta: -timeout=10s
```

**Solutions:**
- Check error handling in API
- Verify database connection is stable
- Increase request timeout
- Check for rate limiting

### Inconsistent Results

**Problem:** Results vary significantly between runs

**Solutions:**
```bash
# Run each test multiple times and use median
for i in {1..3}; do
    ./scripts/test-runner.sh
done

# Flush Redis between tests
redis-cli FLUSHALL

# Restart API between tests
systemctl restart your-api-service

# Wait for system to stabilize
sleep 10
```

### Vegeta Issues

**Problem:** Vegeta command fails

**Solutions:**
```bash
# Verify Vegeta is installed
vegeta --version

# Check syntax
# Incorrect: echo GET... | vegeta attack
# Correct:  echo "GET..." | vegeta attack

# Check API is accessible
curl http://localhost:8080/health

# Increase timeout if requests are slow
vegeta attack ... -timeout=10s
```

---

## Done Criteria

The load testing phase is complete when:

- [ ] All 6 test scenarios executed
- [ ] Results saved in timestamped directory (`./test-results/YYYYMMDD-HHMMSS/`)
- [ ] Pass/fail status determined for each scenario
- [ ] Concurrent test passes (p50 ≤ 500ms, p95 ≤ 800ms) OR documented why not
- [ ] Test summary report created
- [ ] Root cause analysis for any failures
- [ ] Results documented for analysis phase

---

## Next Steps

After load testing is complete:

1. **[validation-checklist.md](validation-checklist.md)** — End-to-end verification
2. **[../testing.md](../testing.md)** — Create performance report
3. Analyze results and document conclusions

---

## Related Documentation

- **[../testing.md](../testing.md)** — Testing methodology and scenarios
- **[api-development.md](api-development.md)** — API implementation
- **[cache-setup.md](cache-setup.md)** — Caching integration
- **[database-setup.md](database-setup.md)** — Database and indexing
