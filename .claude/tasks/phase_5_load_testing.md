# Phase 5: Load Testing Tasks

**Goal:** Execute all 6 test scenarios and collect performance metrics to validate ≤500ms target.

**Estimated Time:** 2-4 hours
**Total Tasks:** 9
**Entry Criteria:** Phase 4 complete

---

## TASK-039: Create test-runner.sh Script

**Status:** pending
**Dependencies:** TASK-038
**Estimated Time:** 30 minutes

**Description:**
Create shell script to run all Vegeta test scenarios.

**Steps:**
1. Create `scripts/test-runner.sh`
2. Implement all 6 test scenarios
3. Add result collection
4. Add pass/fail determination

**Output Definition:**
- test-runner.sh script created
- All 6 scenarios implemented
- Results saved to timestamped directory

**File:** `scripts/test-runner.sh`

**Scenarios:**
1. Health Check
2. Cold Start
3. Baseline (single-thread)
4. Concurrent (50 users, 60s) - PRIMARY TEST
5. Hot Device (90% same device)
6. Large N (500 records)

**See:** `.claude/templates/test_runner_template.sh` for reference

**Verification Commands:**
```bash
cat scripts/test-runner.sh
chmod +x scripts/test-runner.sh
```

**Next Task:** TASK-040

---

## TASK-040: Create Test Scenarios Config

**Status:** pending
**Dependencies:** TASK-039
**Estimated Time:** 30 minutes

**Description:**
Create Vegeta test configuration files for each scenario.

**Output Definition:**
- Test scenarios defined
- Targets configured correctly
- Duration and rate set

**Test Configuration:**

| Scenario | Rate | Duration | Target | Pass Criteria |
|----------|------|----------|--------|---------------|
| Health Check | 1/sec | 10s | /health | p50 ≤ 10ms |
| Cold Start | 1/sec | 10s | /api/v1/sensor-readings | Flush Redis first |
| Baseline | 1/sec | 30s | /api/v1/sensor-readings | p50 ≤ 50ms |
| Concurrent | 50/sec | 60s | /api/v1/sensor-readings | p50 ≤ 500ms, p95 ≤ 800ms |
| Hot Device | 50/sec | 30s | 90% same device | p50 ≤ 500ms |
| Large N | 10/sec | 30s | limit=500 | p50 ≤ 500ms |

**Verification:**
Review test configuration files.

**Next Task:** TASK-041

---

## TASK-041: Run Health Check Test

**Status:** pending
**Dependencies:** TASK-040
**Estimated Time:** 5 minutes

**Description:**
Run health check test to verify system sanity.

**Commands:**
```bash
mkdir -p test-results/$(date +%Y%m%d_%H%M%S)
RESULTS_DIR=test-results/$(date +%Y%m%d_%H%M%S)

echo "GET http://localhost:8080/health" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text > $RESULTS_DIR/health.txt

cat $RESULTS_DIR/health.txt
```

**Output Definition:**
- Health check test complete
- Results saved
- p50 ≤ 10ms

**Expected Output:**
```
Requests      [total, rate]            10, 1.00
Duration      [total, attack, wait]    10.0005s, 9.999s, 1.5ms
Latencies     [mean, 50, 95, 99, max]  5ms, 4ms, 8ms, 10ms, 12ms
Success       [ratio]                  100.0%
Status Codes  [code:count]             200:10
```

**Pass Criteria:** p50 ≤ 10ms

**Next Task:** TASK-042

---

## TASK-042: Run Cold Start Test

**Status:** pending
**Dependencies:** TASK-041
**Estimated Time:** 10 minutes

**Description:**
Run cold start test with empty cache.

**Commands:**
```bash
# Flush Redis first
redis-cli FLUSHALL

# Run test
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text > $RESULTS_DIR/cold_start.txt

cat $RESULTS_DIR/cold_start.txt
```

**Output Definition:**
- Cold start test complete
- Results saved
- p50 ≤ 600ms

**Pass Criteria:** p50 ≤ 600ms (higher tolerance for cold)

**Next Task:** TASK-043

---

## TASK-043: Run Baseline Test

**Status:** pending
**Dependencies:** TASK-042
**Estimated Time:** 10 minutes

**Description:**
Run baseline single-thread test with warm cache.

**Commands:**
```bash
# Warm up cache first
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null

# Run test
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=1 | \
  vegeta report -type=text > $RESULTS_DIR/baseline.txt

cat $RESULTS_DIR/baseline.txt
```

**Output Definition:**
- Baseline test complete
- Results saved
- p50 ≤ 50ms (cache warm)

**Pass Criteria:** p50 ≤ 50ms

**Next Task:** TASK-044

---

## TASK-044: Run Concurrent Test (PRIMARY)

**Status:** pending
**Dependencies:** TASK-043
**Estimated Time:** 15 minutes

**Description:**
Run concurrent load test with 50 users for 60 seconds. **This is the primary validation test.**

**Commands:**
```bash
# Use multiple devices for realistic load
DEVICES=(sensor-001 sensor-002 sensor-003 sensor-004 sensor-005)

# Create targets file
for device in "${DEVICES[@]}"; do
  echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=$device&limit=10"
done > targets.txt

# Run test (shuffle targets for variety)
shuf targets.txt | \
  vegeta attack -duration=60s -rate=50 | \
  vegeta report -type=text > $RESULTS_DIR/concurrent.txt

cat $RESULTS_DIR/concurrent.txt
```

**Output Definition:**
- Concurrent test complete
- Results saved
- **PRIMARY VALIDATION: p50 ≤ 500ms, p95 ≤ 800ms**

**Pass Criteria:**
- p50 ≤ 500ms ✓ **PRIMARY TARGET**
- p95 ≤ 800ms ✓ **PRIMARY TARGET**
- Error rate ≤ 1%

**Expected Output:**
```
Requests      [total, rate]            3000, 50.00
Duration      [total, attack, wait]    60.5s, 60s, 0.5s
Latencies     [mean, 50, 95, 99, max]  350ms, 320ms, 650ms, 800ms, 1200ms
Success       [ratio]                  99.8%
```

**Note:** This is the most important test. It validates the project's primary performance target.

**Next Task:** TASK-045

---

## TASK-045: Run Hot Device Test

**Status:** pending
**Dependencies:** TASK-044
**Estimated Time:** 10 minutes

**Description:**
Run hot device test with 90% of requests to the same device.

**Commands:**
```bash
# Create targets with 90% to sensor-001 (hot device)
for i in {1..90}; do
  echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
done
for i in {1..10}; do
  echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-002&limit=10"
done > hot_targets.txt

# Run test
shuf hot_targets.txt | \
  vegeta attack -duration=30s -rate=50 | \
  vegeta report -type=text > $RESULTS_DIR/hot_device.txt

cat $RESULTS_DIR/hot_device.txt
```

**Output Definition:**
- Hot device test complete
- Results saved
- p50 ≤ 500ms, no outliers >2x p95

**Pass Criteria:**
- p50 ≤ 500ms
- p99 ≤ 2x p95 (no extreme outliers)

**Next Task:** TASK-046

---

## TASK-046: Run Large N Test

**Status:** pending
**Dependencies:** TASK-045
**Estimated Time:** 10 minutes

**Description:**
Run test requesting 500 records (maximum limit).

**Commands:**
```bash
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=500" | \
  vegeta attack -duration=30s -rate=10 | \
  vegeta report -type=text > $RESULTS_DIR/large_n.txt

cat $RESULTS_DIR/large_n.txt
```

**Output Definition:**
- Large N test complete
- Results saved
- p50 ≤ 500ms

**Pass Criteria:** p50 ≤ 500ms with 500 records

**Next Task:** TASK-047

---

## TASK-047: Analyze Test Results

**Status:** pending
**Dependencies:** TASK-046
**Estimated Time:** 30 minutes

**Description:**
Analyze all test results and determine pass/fail status.

**Steps:**
1. Review all test result files
2. Compare against pass criteria
3. Determine overall pass/fail
4. Identify any performance issues

**Output Definition:**
- All test results analyzed
- Pass/fail determined for each scenario
- Overall project success determined

**Analysis Template:**

| Scenario | p50 | p95 | p99 | Error Rate | Pass/Fail | Notes |
|----------|-----|-----|-----|------------|-----------|-------|
| Health Check | ≤10ms | - | - | 0% | | |
| Cold Start | ≤600ms | ≤1000ms | - | 0% | | |
| Baseline | ≤50ms | ≤100ms | - | 0% | | |
| Concurrent | ≤500ms | ≤800ms | - | ≤1% | | **PRIMARY** |
| Hot Device | ≤500ms | ≤600ms | ≤1200ms | ≤1% | | |
| Large N | ≤500ms | ≤800ms | - | ≤1% | | |

**Commands:**
```bash
# Display all results
echo "=== Test Results Summary ==="
echo ""
echo "Health Check:"
cat $RESULTS_DIR/health.txt | grep "Latencies"
echo ""
echo "Cold Start:"
cat $RESULTS_DIR/cold_start.txt | grep "Latencies"
echo ""
echo "Baseline:"
cat $RESULTS_DIR/baseline.txt | grep "Latencies"
echo ""
echo "Concurrent (PRIMARY):"
cat $RESULTS_DIR/concurrent.txt | grep "Latencies"
echo ""
echo "Hot Device:"
cat $RESULTS_DIR/hot_device.txt | grep "Latencies"
echo ""
echo "Large N:"
cat $RESULTS_DIR/large_n.txt | grep "Latencies"
```

**Success Criteria:**
- **PRIMARY:** Concurrent test passes (p50 ≤ 500ms, p95 ≤ 800ms)
- All other tests pass their respective criteria
- Error rate ≤ 1% across all tests

**Next Task:** TASK-048 (Phase 6)

---

## Phase 5 Completion Checklist

- [ ] TASK-039: test-runner.sh script created
- [ ] TASK-040: Test scenarios configured
- [ ] TASK-041: Health check test run
- [ ] TASK-042: Cold start test run
- [ ] TASK-043: Baseline test run
- [ ] TASK-044: Concurrent test run (PRIMARY)
- [ ] TASK-045: Hot device test run
- [ ] TASK-046: Large N test run
- [ ] TASK-047: Test results analyzed

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 6.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
