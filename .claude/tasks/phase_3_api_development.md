# Phase 3: API Development Tasks

**Goal:** Build complete Go API with chi router, pgx connection pooling, and caching integration points.

**Estimated Time:** 4-8 hours
**Total Tasks:** 10
**Entry Criteria:** Phase 1 and Phase 2 complete

---

## TASK-024: Create Model Package (Data Structures)

**Status:** pending
**Dependencies:** TASK-018
**Estimated Time:** 30 minutes

**Description:**
Create data structures for sensor readings and health checks.

**Steps:**
1. Create `internal/model/sensor.go`
2. Create `internal/model/health.go`
3. Define SensorReading struct
4. Define HealthStatus structs

**Output Definition:**
- Model package created
- All data structures defined with JSON tags

**Files:**
- `internal/model/sensor.go`
- `internal/model/health.go`

**See:** `.claude/templates/go_model_template.go` for reference

**Verification Commands:**
```bash
cat internal/model/sensor.go
cat internal/model/health.go
```

**Next Task:** TASK-025

---

## TASK-025: Create Config Package (Configuration)

**Status:** pending
**Dependencies:** TASK-024
**Estimated Time:** 30 minutes

**Description:**
Create configuration loading from environment variables.

**Steps:**
1. Create `internal/config/config.go`
2. Define Config struct
3. Implement Load() function
4. Load from environment variables

**Output Definition:**
- Config package created
- Loads all required environment variables
- Provides defaults for optional values

**File:** `internal/config/config.go`

**See:** `.claude/templates/go_config_template.go` for reference

**Verification Commands:**
```bash
cat internal/config/config.go
```

**Next Task:** TASK-026

---

## TASK-026: Create Repository Package (Database)

**Status:** pending
**Dependencies:** TASK-025
**Estimated Time:** 60 minutes

**Description:**
Create database repository with pgx connection pooling.

**Steps:**
1. Create `internal/repository/sensor_repo.go`
2. Create `internal/repository/health_repo.go`
3. Implement connection pooling (25 max, 5 min)
4. Implement Query() method for sensor readings
5. Use prepared statements

**Output Definition:**
- Repository package created
- Connection pooling configured
- Query methods implemented
- Prepared statements used

**Files:**
- `internal/repository/sensor_repo.go`
- `internal/repository/health_repo.go`

**See:** `.claude/templates/go_repository_template.go` for reference

**Key Requirements:**
- Use pgx/v5 for connection pooling
- Max connections: 25, Min: 5
- Query timeout: 5 seconds
- Prepared statements for all queries

**Verification Commands:**
```bash
cat internal/repository/sensor_repo.go
cat internal/repository/health_repo.go
```

**Next Task:** TASK-027

---

## TASK-027: Create Cache Package (Redis Wrapper)

**Status:** pending
**Dependencies:** TASK-025
**Estimated Time:** 45 minutes

**Description:**
Create Redis cache client wrapper.

**Steps:**
1. Create `internal/cache/redis_cache.go`
2. Implement Get() and Set() methods
3. Use go-redis/v9 client
4. Configure cache key format

**Output Definition:**
- Cache package created
- Get/Set methods implemented
- Cache key format consistent

**File:** `internal/cache/redis_cache.go`

**Cache Key Format:**
```
sensor:{device_id}:readings:{limit}[:{reading_type}]
```

**Examples:**
- `sensor:sensor-001:readings:10`
- `sensor:sensor-002:readings:50:temperature`

**Verification Commands:**
```bash
cat internal/cache/redis_cache.go
```

**Next Task:** TASK-028

---

## TASK-028: Create Service Package (Business Logic)

**Status:** pending
**Dependencies:** TASK-026, TASK-027
**Estimated Time:** 60 minutes

**Description:**
Create service layer with business logic and cache orchestration.

**Steps:**
1. Create `internal/service/sensor_service.go`
2. Create `internal/service/health_service.go`
3. Implement GetSensorReadings() with cache-aside pattern
4. Implement validation logic

**Output Definition:**
- Service package created
- Cache-aside pattern implemented
- Input validation implemented
- Error handling implemented

**Files:**
- `internal/service/sensor_service.go`
- `internal/service/health_service.go`

**See:** `.claude/templates/go_service_template.go` for reference

**Key Requirements:**
- Cache-aside: check cache, query DB on miss, populate cache
- 30s TTL for cache entries
- Input validation for device_id and limit
- Custom error types (ErrDeviceNotFound, ErrInvalidParameter)

**Verification Commands:**
```bash
cat internal/service/sensor_service.go
cat internal/service/health_service.go
```

**Next Task:** TASK-029

---

## TASK-029: Create Handler Package (HTTP)

**Status:** pending
**Dependencies:** TASK-028
**Estimated Time:** 60 minutes

**Description:**
Create HTTP handlers with chi router.

**Steps:**
1. Create `internal/handler/sensor_handler.go`
2. Create `internal/handler/health_handler.go`
3. Implement GetSensorReadings handler
4. Implement HealthCheck handler
5. Add request validation
6. Add error responses

**Output Definition:**
- Handler package created
- All endpoints implemented
- Request validation implemented
- Error responses formatted per API spec

**Files:**
- `internal/handler/sensor_handler.go`
- `internal/handler/health_handler.go`

**See:** `.claude/templates/go_handler_template.go` for reference

**Key Requirements:**
- Validate device_id (required)
- Validate limit (1-500)
- Return proper HTTP status codes
- Format response per API spec

**Verification Commands:**
```bash
cat internal/handler/sensor_handler.go
cat internal/handler/health_handler.go
```

**Next Task:** TASK-030

---

## TASK-030: Create cmd/api/main.go (Entry Point)

**Status:** pending
**Dependencies:** TASK-029
**Estimated Time:** 30 minutes

**Description:**
Create application entry point with chi router setup.

**Steps:**
1. Create `cmd/api/main.go`
2. Initialize config
3. Initialize dependencies (repository, cache, service, handler)
4. Setup chi router
5. Register routes
6. Start HTTP server

**Output Definition:**
- main.go created
- Server starts on port 8080
- All routes registered
- Graceful shutdown implemented

**File:** `cmd/api/main.go`

**Routes:**
```
GET /api/v1/sensor-readings
GET /health
```

**Verification Commands:**
```bash
cat cmd/api/main.go
```

**Next Task:** TASK-031

---

## TASK-031: Implement Request Validation

**Status:** pending
**Dependencies:** TASK-029
**Estimated Time:** 30 minutes

**Description:**
Ensure all request validation is implemented in handlers.

**Validation Rules:**
- `device_id`: Required, 1-50 chars, alphanumeric + hyphen/underscore
- `limit`: Optional, 1-500, default 10
- `reading_type`: Optional, 1-30 chars, alphanumeric

**Output Definition:**
- All validation rules implemented
- Returns 400 for invalid input
- Error response includes INVALID_PARAMETER code

**Verification:**
Review handler code to ensure validation is present.

**Next Task:** TASK-032

---

## TASK-032: Implement Error Handling

**Status:** pending
**Dependencies:** TASK-029
**Estimated Time:** 30 minutes

**Description:**
Ensure all error handling is implemented per API spec.

**Error Mapping:**
| Error | HTTP Status | Error Code |
|-------|-------------|------------|
| Invalid parameter | 400 | INVALID_PARAMETER |
| Device not found | 404 | DEVICE_NOT_FOUND |
| Database error | 500 | INTERNAL_ERROR |

**Output Definition:**
- All errors mapped to correct HTTP status
- Error responses formatted per API spec

**Verification:**
Review handler error handling code.

**Next Task:** TASK-033

---

## TASK-033: Test API Endpoints Manually

**Status:** pending
**Dependencies:** TASK-030
**Estimated Time:** 30 minutes

**Description:**
Start the API server and test endpoints manually.

**Steps:**
1. Start API server: `go run cmd/api/main.go`
2. Test health endpoint
3. Test sensor-readings endpoint with valid request
4. Test error cases (missing device_id, invalid limit)
5. Verify responses match API spec

**Output Definition:**
- API server running on port 8080
- All endpoints return correct responses
- Error cases handled correctly

**Verification Commands:**
```bash
# Start server (in separate terminal)
go run cmd/api/main.go

# Test health endpoint
curl http://localhost:8080/health

# Test sensor-readings endpoint
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Test error case (missing device_id)
curl "http://localhost:8080/api/v1/sensor-readings?limit=10" | jq '.error.code'

# Test error case (invalid limit)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=1000" | jq '.error.code'
```

**Expected Output:**
```json
# Health endpoint
{
  "status": "healthy",
  "timestamp": "2026-03-11T22:15:00Z",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5
    }
  }
}

# Sensor readings endpoint
{
  "data": [
    {
      "id": "12345678",
      "device_id": "sensor-001",
      "timestamp": "2026-03-11T10:30:00Z",
      "reading_type": "temperature",
      "value": 23.45,
      "unit": "celsius"
    }
  ],
  "meta": {
    "count": 1,
    "limit": 10,
    "device_id": "sensor-001"
  }
}
```

**Next Task:** TASK-034 (Phase 4)

---

## Phase 3 Completion Checklist

- [ ] TASK-024: Model package created
- [ ] TASK-025: Config package created
- [ ] TASK-026: Repository package created
- [ ] TASK-027: Cache package created
- [ ] TASK-028: Service package created
- [ ] TASK-029: Handler package created
- [ ] TASK-030: cmd/api/main.go created
- [ ] TASK-031: Request validation implemented
- [ ] TASK-032: Error handling implemented
- [ ] TASK-033: API endpoints tested manually

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 4.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
