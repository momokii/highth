# Validation Checklist

This comprehensive checklist verifies the entire system is ready for production use. Complete all checks before considering the project complete.

## Table of Contents

- [How to Use This Checklist](#how-to-use-this-checklist)
- [Phase 0: Environment & Tooling](#phase-0-environment--tooling)
- [Phase 1: Database Setup](#phase-1-database-setup)
- [Phase 2: Data Generation](#phase-2-data-generation)
- [Phase 3: API Development](#phase-3-api-development)
- [Phase 4: Cache Setup](#phase-4-cache-setup)
- [Phase 5: Load Testing](#phase-5-load-testing)
- [Phase 6: Results & Documentation](#phase-6-results--documentation)
- [System-Wide Verification](#system-wide-verification)
- [Final Sign-Off](#final-sign-off)

---

## How to Use This Checklist

### Usage Instructions

1. **Complete phases in order** — Each phase depends on previous phases
2. **Check every item** — Don't skip items; ensure all pass
3. **Document failures** — If an item fails, note the reason and fix
4. **Re-check after fixes** — Verify the fix worked by re-running the check
5. **Timestamp completion** — Note when each phase was completed

### Checking Conventions

| Symbol | Meaning |
|--------|---------|
| ☐ | Not yet checked |
| ☑ | Checked and passed |
| ☒ | Checked and failed (needs fix) |

### Example Usage

```bash
# Create a validation log
echo "# Validation Log - $(date)" > validation-log.md
echo "" >> validation-log.md

# Run checks from this checklist
# Mark each item as ☑ or ☒ as you go
```

---

## Phase 0: Environment & Tooling

### Go Installation

- [ ] **Go 1.21+ installed**
  ```bash
  go version | grep -o 'go[0-9.]*'
  # Expected: go1.21.0 or higher
  ```

- [ ] **Go bin directory in PATH**
  ```bash
  which go
  # Expected: /usr/local/go/bin/go or similar
  echo $PATH | grep -q "$(go env GOPATH)/bin"
  # Expected: exit code 0 (success)
  ```

### Docker Installation

- [ ] **Docker installed and running**
  ```bash
  docker --version
  # Expected: Docker version 20.10.0 or higher
  docker ps
  # Expected: List of containers (may be empty)
  ```

- [ ] **Docker Compose installed**
  ```bash
  docker compose version
  # Expected: Docker Compose version v2.0.0 or higher
  ```

### Client Tools

- [ ] **PostgreSQL client installed**
  ```bash
  psql --version
  # Expected: psql (PostgreSQL) 14.0 or higher
  ```

- [ ] **Redis client installed**
  ```bash
  redis-cli --version
  # Expected: redis-cli 7.0.0 or higher
  ```

- [ ] **Vegeta installed**
  ```bash
  vegeta --version
  # Expected: vegeta version 12.8.0 or higher
  ```

### Project Structure

- [ ] **Go module initialized**
  ```bash
  cat go.mod | head -1
  # Expected: module github.com/yourusername/highth
  ```

- [ ] **Directory structure created**
  ```bash
  ls -la cmd/api internal/{handler,service,repository,cache,model,config}
  # Expected: All directories exist
  ```

- [ ] **.env file exists**
  ```bash
  ls .env
  # Expected: .env file exists
  ```

### Phase 0 Exit Criteria

**Phase 0 is complete when:**
- All tools installed and accessible
- Project structure created
- .env file configured

---

## Phase 1: Database Setup

### PostgreSQL Running

- [ ] **PostgreSQL is running**
  ```bash
  docker ps | grep postgres
  # OR for native:
  sudo systemctl status postgresql | grep "active (running)"
  ```

- [ ] **Database exists**
  ```bash
  psql "$DATABASE_URL" -c "\l" | grep sensor_db
  # Expected: sensor_db database listed
  ```

- [ ] **Table created**
  ```bash
  psql "$DATABASE_URL" -c "\dt sensor_readings"
  # Expected: Table sensor_readings listed
  ```

### Schema Verification

- [ ] **Table structure correct**
  ```bash
  psql "$DATABASE_URL" -c "\d sensor_readings"
  # Expected columns: id, device_id, timestamp, reading_type, value, unit, metadata
  ```

- [ ] **BRIN index exists**
  ```bash
  psql "$DATABASE_URL" -c "\di idx_sensor_readings_ts_brin"
  # Expected: Index listed
  ```

- [ ] **Composite B-tree index exists**
  ```bash
  psql "$DATABASE_URL" -c "\di idx_sensor_readings_device_ts"
  # Expected: Index listed
  ```

- [ ] **Covering index exists**
  ```bash
  psql "$DATABASE_URL" -c "\di idx_sensor_readings_device_covering"
  # Expected: Index listed
  ```

### Connection Test

- [ ] **Can connect to database**
  ```bash
  psql "$DATABASE_URL" -c "SELECT 1"
  # Expected: Single row with value 1
  ```

- [ ] **Connection string in .env is correct**
  ```bash
  grep DATABASE_URL .env
  # Expected: DATABASE_URL=postgres://...@localhost:5432/sensor_db
  ```

### Phase 1 Exit Criteria

**Phase 1 is complete when:**
- PostgreSQL 16+ running
- Database `sensor_db` exists
- Table `sensor_readings` created with 3 indexes
- Connection verified

---

## Phase 2: Data Generation

### Data Generated

- [ ] **50M rows inserted**
  ```bash
  psql "$DATABASE_URL" -c "SELECT count(*) FROM sensor_readings"
  # Expected: 50000000
  ```

- [ ] **Distribution is non-uniform**
  ```bash
  psql "$DATABASE_URL" -c "
  SELECT
      percentile_cont(0.50) WITHIN GROUP (ORDER BY reading_count) as p50,
      percentile_cont(0.95) WITHIN GROUP (ORDER BY reading_count) as p95,
      max(reading_count) as max,
      min(reading_count) as min
  FROM (
      SELECT device_id, count(*) as reading_count
      FROM sensor_readings
      GROUP BY device_id
  ) counts;"
  # Expected: p50 ~40K, p95 ~150K, max >150K, min <20K
  ```

### Data Integrity

- [ ] **All devices have data**
  ```bash
  psql "$DATABASE_URL" -c "SELECT count(DISTINCT device_id) FROM sensor_readings"
  # Expected: 1000 (or close to it)
  ```

- [ ] **Time range is reasonable**
  ```bash
  psql "$DATABASE_URL" -c "
  SELECT
      min(timestamp) as earliest,
      max(timestamp) as latest,
      max(timestamp) - min(timestamp) as span
  FROM sensor_readings;"
  # Expected: span ~90 days
  ```

### Index Verification

- [ ] **Indexes are functional**
  ```bash
  psql "$DATABASE_URL" -c "EXPLAIN SELECT * FROM sensor_readings WHERE device_id = 'sensor-001' ORDER BY timestamp DESC LIMIT 10"
  # Expected: Index Scan or Index Only Scan
  ```

- [ ] **ANALYZE has been run**
  ```bash
  psql "$DATABASE_URL" -c "SELECT last_analyze FROM pg_stat_user_tables WHERE relname = 'sensor_readings'"
  # Expected: Recent timestamp
  ```

### Phase 2 Exit Criteria

**Phase 2 is complete when:**
- 50M rows inserted
- Distribution verified (non-uniform)
- Indexes functional
- ANALYZE run

---

## Phase 3: API Development

### API Running

- [ ] **API server starts**
  ```bash
  go run cmd/api/main.go &
  sleep 2
  curl -s http://localhost:8080/health
  # Expected: Valid JSON response
  ```

- [ ] **Server listening on port 8080**
  ```bash
  sudo lsof -i :8080 | grep LISTEN
  # Expected: Process listening on port 8080
  ```

### Health Check

- [ ] **Health endpoint returns 200**
  ```bash
  curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health
  # Expected: 200
  ```

- [ ] **Health shows database healthy**
  ```bash
  curl -s http://localhost:8080/health | jq '.checks.database.status'
  # Expected: "healthy"
  ```

### Sensor Readings Endpoint

- [ ] **Valid request returns 200**
  ```bash
  curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
  # Expected: 200
  ```

- [ ] **Valid request returns data**
  ```bash
  curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | jq '.data | length'
  # Expected: 10 (or close to it)
  ```

- [ ] **Missing device_id returns 400**
  ```bash
  curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/sensor-readings?limit=10"
  # Expected: 400
  ```

- [ ] **Unknown device returns 404**
  ```bash
  curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-999&limit=10"
  # Expected: 404
  ```

### Connection Pool

- [ ] **Connection pool configured**
  ```bash
  curl -s http://localhost:8080/health | jq '.checks.connection_pool'
  # Expected: Contains pool stats
  ```

- [ ] **Max connections matches config**
  ```bash
  grep DB_MAX_CONNECTIONS .env
  # Expected: DB_MAX_CONNECTIONS=25
  ```

### Phase 3 Exit Criteria

**Phase 3 is complete when:**
- API server runs on port 8080
- Health check returns 200
- `/api/v1/sensor-readings` endpoint functional
- Error handling working (400, 404, 500)
- Connection pool configured

---

## Phase 4: Cache Setup

### Redis Running

- [ ] **Redis is running**
  ```bash
  docker ps | grep redis
  # OR for native:
  sudo systemctl status redis | grep "active (running)"
  ```

- [ ] **Redis responds to ping**
  ```bash
  redis-cli ping
  # Expected: PONG
  ```

### Cache Integration

- [ ] **Cache is enabled**
  ```bash
  grep CACHE_ENABLED .env
  # Expected: CACHE_ENABLED=true
  ```

- [ ] **Cache populates on request**
  ```bash
  redis-cli FLUSHALL
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
  redis-cli KEYS "sensor:*"
  # Expected: At least one key
  ```

- [ ] **Cache hits return data**
  ```bash
  redis-cli GET "sensor:sensor-001:readings:10"
  # Expected: JSON array of sensor readings
  ```

### Cache Behavior

- [ ] **TTL is 30 seconds**
  ```bash
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
  redis-cli TTL "sensor:sensor-001:readings:10"
  # Expected: ~30 (seconds)
  ```

- [ ] **Different limits create different keys**
  ```bash
  redis-cli FLUSHALL
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=50" > /dev/null
  redis-cli KEYS "sensor:sensor-001:*"
  # Expected: 2 keys (limit=10 and limit=50)
  ```

### Health Check

- [ ] **Health shows cache status**
  ```bash
  curl -s http://localhost:8080/health | jq '.checks.cache'
  # Expected: Status and hit rate info
  ```

### Graceful Degradation

- [ ] **API works when Redis is down**
  ```bash
  docker stop redis-cache
  sleep 2
  curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
  # Expected: 200 (should still work)
  docker start redis-cache
  ```

### Phase 4 Exit Criteria

**Phase 4 is complete when:**
- Redis running and accessible
- Cache integration working
- 30s TTL functioning
- Cache hits returning <10ms
- Graceful degradation working

---

## Phase 5: Load Testing

### Test Execution

- [ ] **All 6 test scenarios executed**
  ```bash
  ls test-results/
  # Expected: At least one timestamped directory
  ```

- [ ] **Results files exist**
  ```bash
  ls test-results/*/health.json
  ls test-results/*/cold_start.txt
  ls test-results/*/baseline.txt
  ls test-results/*/concurrent.txt
  ls test-results/*/hot_device.txt
  ls test-results/*/large_n.txt
  ```

### Performance Targets

- [ ] **Concurrent test passes**
  ```bash
  grep "Latencies" test-results/*/concurrent.txt
  # Expected: p50 ≤ 500ms, p95 ≤ 800ms
  ```

- [ ] **Cold start test passes**
  ```bash
  grep "Latencies" test-results/*/cold_start.txt
  # Expected: p50 ≤ 600ms, p95 ≤ 1000ms
  ```

- [ ] **Baseline test passes**
  ```bash
  grep "Latencies" test-results/*/baseline.txt
  # Expected: p50 ≤ 50ms, p95 ≤ 100ms
  ```

- [ ] **Hot device test passes**
  ```bash
  grep "Latencies" test-results/*/hot_device.txt
  # Expected: p50 ≤ 500ms, p95 ≤ 600ms
  ```

- [ ] **Large N test passes**
  ```bash
  grep "Latencies" test-results/*/large_n.txt
  # Expected: p50 ≤ 500ms, p95 ≤ 800ms
  ```

### Error Rates

- [ ] **Success rate ≥ 99%**
  ```bash
  grep "Success" test-results/*/concurrent.txt
  # Expected: Success ratio ≥ 99.00%
  ```

- [ ] **No HTTP 500 errors**
  ```bash
  grep "Status Codes" test-results/*/concurrent.txt
  # Expected: Only 200 status codes (or 99%+ 200)
  ```

### Phase 5 Exit Criteria

**Phase 5 is complete when:**
- All 6 scenarios executed
- Results saved and documented
- Pass/fail determined
- Concurrent test passes (or documented why not)

---

## Phase 6: Results & Documentation

### Documentation

- [ ] **Performance report created**
  ```bash
  ls docs/results/performance-report.md
  # Expected: File exists
  ```

- [ ] **Report contains all sections**
  ```bash
  grep -q "Performance Summary" docs/results/performance-report.md
  grep -q "Test Results" docs/results/performance-report.md
  grep -q "Conclusions" docs/results/performance-report.md
  # Expected: All sections present
  ```

- [ ] **Results documented honestly**
  ```bash
  # If targets missed:
  grep -q "Root Cause" docs/results/performance-report.md
  grep -q "Why Targets Were Missed" docs/results/performance-report.md
  # Expected: Honest assessment
  ```

### Analysis

- [ ] **p50 latency documented**
  ```bash
  grep "p50" docs/results/performance-report.md
  # Expected: Actual p50 value listed
  ```

- [ ] **p95 latency documented**
  ```bash
  grep "p95" docs/results/performance-report.md
  # Expected: Actual p95 value listed
  ```

- [ ] **Cache hit rate documented**
  ```bash
  grep "hit rate" docs/results/performance-report.md
  # Expected: Hit rate percentage listed
  ```

### Phase 6 Exit Criteria

**Phase 6 is complete when:**
- Performance report created
- All metrics documented
- Conclusions and recommendations included
- Lessons learned captured

---

## System-Wide Verification

### End-to-End Test

- [ ] **Complete request flow works**
  ```bash
  # 1. Make request
  response=$(curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10")

  # 2. Verify response
  echo "$response" | jq '.data' > /dev/null
  # Expected: Valid JSON

  # 3. Verify cache
  redis-cli EXISTS "sensor:sensor-001:readings:10"
  # Expected: 1 (key exists)
  ```

- [ ] **System recovers from errors**
  ```bash
  # 1. Stop Redis
  docker stop redis-cache

  # 2. Make request (should still work)
  curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | jq '.data'
  # Expected: Valid JSON (from database)

  # 3. Restart Redis
  docker start redis-cache

  # 4. Make request (should use cache again)
  curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | jq '.data'
  # Expected: Valid JSON
  ```

### Resource Usage

- [ ] **Memory usage is reasonable**
  ```bash
  # Check API process memory
  ps aux | grep "cmd/api/main" | awk '{print $6}'
  # Expected: < 500MB (typical for Go API)

  # Check Redis memory
  redis-cli INFO memory | grep used_memory_human
  # Expected: < 1GB (depends on cache size)
  ```

- [ ] **Database connections not exhausted**
  ```bash
  psql "$DATABASE_URL" -c "
  SELECT count(*) FROM pg_stat_activity
  WHERE datname = 'sensor_db';"
  # Expected: < DB_MAX_CONNECTIONS (default 25)
  ```

### Documentation Completeness

- [ ] **All implementation docs exist**
  ```bash
  ls docs/implementation/
  # Expected: README.md, plan.md, dev-environment.md,
  #                   database-setup.md, data-generation.md,
  #                   api-development.md, cache-setup.md,
  #                   load-testing-setup.md, validation-checklist.md
  ```

- [ ] **All Phase 1 docs exist**
  ```bash
  ls docs/
  # Expected: README.md, architecture.md, stack.md,
  #                   api-spec.md, testing.md, ui-consideration.md
  ```

---

## Final Sign-Off

### Primary Success Criteria

The system is **complete** when ALL of the following are true:

- [ ] **Concurrent load test passes** (p50 ≤ 500ms, p95 ≤ 800ms)
- [ ] **API returns HTTP 200** for known device_id queries
- [ ] **API returns HTTP 404** for unknown device_id
- [ ] **Health check returns 200** with database/cache status
- [ ] **Cache hit rate ≥ 80%** (or documented why not)
- [ ] **Error rate ≤ 1%** across all load tests
- [ ] **50M rows** in sensor_readings table
- [ ] **All 3 indexes** created and functional
- [ ] **Graceful degradation** working (API works when cache is down)

### What If Targets Are Missed?

**Project passes with notes if:**
- Targets missed but documented honestly
- Root cause identified
- Architecture principles remain sound
- Mitigation attempted

**Project requires rework if:**
- Error rate > 5% (system instability)
- p50 > 1000ms (unacceptable performance)
- System crashes under load
- Basic functionality broken

### Quick Verification Commands

```bash
# Run all verification commands at once
echo "=== Quick System Verification ==="

# Database
echo -n "1. Database row count: "
psql "$DATABASE_URL" -t -c "SELECT count(*) FROM sensor_readings"

# API
echo -n "2. API health: "
curl -s http://localhost:8080/health | jq -r '.status'

# Cache
echo -n "3. Redis status: "
redis-cli ping

# Load test
echo "4. Quick load test (10s @ 10 RPS):"
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=10s -rate=10 | vegeta report -type=text

echo ""
echo "=== Verification Complete ==="
```

### Declaration

**I declare that:**

- [ ] All verification checks have been completed
- [ ] Failed checks (if any) are documented
- [ ] System is ready for production use (or notes indicate otherwise)
- [ ] Documentation is complete and accurate
- [ ] Results are honestly reported

**Signature:** __________________________

**Date:** __________________________

---

## Next Steps

After validation is complete:

1. **Document results** — Create performance report in `docs/results/`
2. **Capture lessons learned** — What went well? What could be improved?
3. **Plan next iteration** — How would you improve this architecture?
4. **Share results** — Present findings to team/stakeholders

---

## Related Documentation

- **[plan.md](plan.md)** — Master implementation plan
- **[load-testing-setup.md](load-testing-setup.md)** — Test execution guide
- **[../testing.md](../testing.md)** — Testing methodology
