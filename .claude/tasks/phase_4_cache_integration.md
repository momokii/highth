# Phase 4: Cache Integration Tasks

**Goal:** Integrate Redis caching with write-through pattern and 30s TTL.

**Estimated Time:** 1-2 hours
**Total Tasks:** 5
**Entry Criteria:** Phase 3 complete

---

## TASK-034: Start Redis Container

**Status:** pending
**Dependencies:** TASK-009
**Estimated Time:** 5 minutes

**Description:**
Start Redis container using Docker Compose.

**Steps:**
1. Run `docker compose up -d redis`
2. Wait for container to be healthy
3. Verify Redis is accepting connections

**Output Definition:**
- Redis container running
- Health check passing
- Port 6379 accessible

**Verification Commands:**
```bash
docker ps | grep highth-redis
docker compose ps redis
redis-cli ping
```

**Expected Output:**
```
highth-redis   Up   0.0.0.0:6379->6379/tcp   (healthy)
PONG
```

**Next Task:** TASK-035

---

## TASK-035: Implement Write-Through Cache Logic

**Status:** pending
**Dependencies:** TASK-034, TASK-027
**Estimated Time:** 30 minutes

**Description:**
Implement cache-aside pattern in service layer.

**Pattern:**
```
1. Check cache
2. If hit: return cached data
3. If miss: query database, populate cache, return data
```

**Implementation Location:**
- `internal/service/sensor_service.go`

**Output Definition:**
- Cache-aside pattern implemented
- Cache checked before database query
- Cache populated on miss

**Key Implementation Points:**
```go
func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]SensorReading, error) {
    // 1. Check cache
    key := cacheKey(deviceID, limit, readingType)
    if data, err := s.cache.Get(ctx, key); err == nil {
        return data, nil // Cache hit
    }

    // 2. Query database
    data, err := s.repo.Query(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, err
    }

    // 3. Populate cache
    _ = s.cache.Set(ctx, key, data, 30*time.Second)

    return data, nil
}
```

**Verification:**
Review service code to confirm cache-aside pattern.

**Next Task:** TASK-036

---

## TASK-036: Configure 30s TTL

**Status:** pending
**Dependencies:** TASK-035
**Estimated Time:** 15 minutes

**Description:**
Configure 30 second TTL for all cache entries.

**Steps:**
1. Set TTL to 30 seconds in cache.Set() calls
2. Verify TTL is applied correctly

**Output Definition:**
- All cache entries have 30s TTL
- TTL verified with redis-cli

**Verification Commands:**
```bash
# Make a request to populate cache
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null

# Check TTL
redis-cli TTL "sensor:sensor-001:readings:10"
```

**Expected Output:**
```
30  (or close to 30)
```

**Note:** TTL decrements over time. Value should be approximately 30 immediately after request.

**Next Task:** TASK-037

---

## TASK-037: Implement Graceful Degradation

**Status:** pending
**Dependencies:** TASK-036
**Estimated Time:** 30 minutes

**Description:**
Ensure system continues working if Redis is unavailable.

**Behavior:**
- If Redis is down: Log error, serve from database
- Health check: Show cache as degraded
- No errors returned to client

**Implementation:**
- Cache errors are logged but not returned
- Database queries continue working
- Health endpoint reflects cache status

**Output Definition:**
- System works without Redis
- Cache errors logged only
- Health check shows cache status

**Verification Commands:**
```bash
# Stop Redis
docker compose stop redis

# Make request (should still work)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Check health (should show cache as degraded)
curl http://localhost:8080/health

# Restart Redis
docker compose start redis
```

**Expected Health Output (Redis down):**
```json
{
  "status": "degraded",
  "checks": {
    "database": {"status": "healthy"},
    "cache": {"status": "unhealthy", "error": "connection refused"}
  }
}
```

**Next Task:** TASK-038

---

## TASK-038: Test Cache Hit/Miss Behavior

**Status:** pending
**Dependencies:** TASK-037
**Estimated Time:** 30 minutes

**Description:**
Test cache behavior to confirm it's working correctly.

**Test Scenarios:**

1. **Cold Cache (Miss)**
```bash
redis-cli FLUSHALL
time curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
```
Expected: Slower response (database query)

2. **Warm Cache (Hit)**
```bash
time curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
```
Expected: Faster response (<10ms)

3. **Cache Key Verification**
```bash
redis-cli KEYS "sensor:*"
redis-cli GET "sensor:sensor-001:readings:10"
```

4. **TTL Verification**
```bash
redis-cli TTL "sensor:sensor-001:readings:10"
```
Expected: ~30 seconds

5. **Different Cache Keys**
```bash
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=50" > /dev/null
redis-cli KEYS "sensor:*"
```
Expected: Two different keys

**Output Definition:**
- Cache miss → database query
- Cache hit → fast response
- Cache keys created correctly
- TTL functioning

**Performance Targets:**
| Operation | Target Latency |
|-----------|----------------|
| Cache hit | <10ms |
| Cache miss | 200-600ms |

**Next Task:** TASK-039 (Phase 5)

---

## Phase 4 Completion Checklist

- [ ] TASK-034: Redis container running
- [ ] TASK-035: Write-through cache logic implemented
- [ ] TASK-036: 30s TTL configured
- [ ] TASK-037: Graceful degradation implemented
- [ ] TASK-038: Cache hit/miss behavior tested

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 5.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
