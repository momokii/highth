# Coding Standards and Architecture

**Project:** High-Performance IoT Sensor Query System
**Purpose:** Define all architecture patterns, naming conventions, and quality requirements
**Version:** 1.0

---

## Table of Contents

1. [Architecture Patterns](#architecture-patterns)
2. [Package Organization Rules](#package-organization-rules)
3. [Naming Conventions (Go Idioms)](#naming-conventions-go-idioms)
4. [Error Handling Patterns](#error-handling-patterns)
5. [Code Quality Requirements](#code-quality-requirements)
6. [Database Query Patterns](#database-query-patterns)
7. [Caching Patterns](#caching-patterns)
8. [Testing Expectations](#testing-expectations)
9. [Performance Requirements](#performance-requirements)
10. [Security Considerations](#security-considerations)
11. [Dependencies](#dependencies)
12. [Complete Code Examples](#complete-code-examples)

---

## Architecture Patterns

### Layered Architecture (Handler → Service → Repository)

```
Request → Handler → Service → Repository → Database
                    ↓           ↓
                   Cache      Model
```

**Strict Separation of Concerns:**

#### 1. Handler Layer (`internal/handler/`)

**Responsibilities:**
- HTTP concerns ONLY
- Request validation
- Response formatting
- HTTP status codes

**Prohibited:**
- NO business logic
- NO database queries
- NO cache operations (delegate to service)

#### 2. Service Layer (`internal/service/`)

**Responsibilities:**
- Business logic
- Cache orchestration
- Transaction coordination

**Prohibited:**
- NO HTTP concerns
- NO direct database access (uses repository)

#### 3. Repository Layer (`internal/repository/`)

**Responsibilities:**
- Database queries ONLY
- Connection management

**Prohibited:**
- NO business logic
- NO HTTP concerns
- NO cache operations

#### 4. Cache Layer (`internal/cache/`)

**Responsibilities:**
- Cache operations ONLY

**Prohibited:**
- NO business logic
- NO database queries

#### 5. Model Layer (`internal/model/`)

**Responsibilities:**
- Data structures

**Prohibited:**
- NO logic

#### 6. Config Layer (`internal/config/`)

**Responsibilities:**
- Configuration loading
- Environment variable parsing

**Prohibited:**
- NO business logic

---

## Package Organization Rules

```
highth/
├── cmd/
│   └── api/
│       └── main.go              # Application entry point ONLY
├── internal/
│   ├── handler/
│   │   ├── sensor_handler.go    # HTTP handlers for sensor endpoints
│   │   └── health_handler.go    # HTTP handlers for health endpoint
│   ├── service/
│   │   ├── sensor_service.go    # Business logic for sensors
│   │   └── health_service.go    # Business logic for health
│   ├── repository/
│   │   ├── sensor_repo.go       # Database queries for sensors
│   │   └── health_repo.go       # Database queries for health
│   ├── cache/
│   │   └── redis_cache.go       # Redis operations
│   ├── model/
│   │   ├── sensor.go            # Sensor data structures
│   │   └── health.go            # Health data structures
│   └── config/
│       └── config.go            # Configuration loading
├── pkg/                         # External-facing packages (if needed)
├── docs/                        # Documentation (existing)
├── scripts/                     # Utility scripts
├── test-results/                # Vegeta test results
├── .env                         # Environment variables (not in git)
├── .env.example                 # Environment template
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml
```

**Rules:**
- `internal/` packages CANNOT be imported by external projects
- `pkg/` packages CAN be imported by external projects
- Each package has a SINGLE responsibility
- NO circular dependencies between packages

---

## Naming Conventions (Go Idioms)

### Files

- **Lowercase snake_case:** `sensor_handler.go`, `redis_cache.go`
- **Single responsibility:** One file per major type/function group
- **Test files:** `sensor_handler_test.go` (same package, `_test.go` suffix)

### Packages

- **Lowercase single word:** `handler`, `service`, `repository`
- **NO underscores:** Use `handler` not `sensor_handler`
- **Descriptive but concise:** `cache` not `redis_cache_layer`

### Types/Structs

- **PascalCase:** `SensorReading`, `HealthCheck`, `CacheConfig`
- **Full words:** `SensorReading` not `SensorReading` (no abbreviations)
- **Interfaces:** Add `er` suffix if single method: `Reader`, `Writer`

### Functions/Methods

- **PascalCase** if exported: `GetSensorReadings`, `ValidateRequest`
- **camelCase** if private: `parseDeviceID`, `validateLimit`
- **Verbs for actions:** `Get`, `Create`, `Update`, `Delete`, `Validate`
- **Nouns for getters:** `DeviceID()`, `Timestamp()` (not `GetDeviceID()`)

### Constants

- **PascalCase** if exported: `MaxLimit`, `DefaultTTL`
- **camelCase** if private: `defaultCacheKey`

### Variables

- **camelCase:** `deviceID`, `readingCount`, `cacheKey`
- **Short for local scope:** `i`, `db`, `ctx`
- **Descriptive for package scope:** `databaseURL`, `redisClient`

### Interfaces

- **PascalCase:** `CacheProvider`, `SensorRepository`
- **-er suffix** for single-method interfaces: `Reader`, `Writer`
- **Keep interfaces small:** 1-3 methods preferred

### Configuration Keys

- **UPPER_SNAKE_CASE:** `DATABASE_URL`, `REDIS_ENABLED`, `CACHE_TTL`
- **Match .env file format**

---

## Error Handling Patterns

### Error Wrapping

```go
// ALWAYS wrap errors with context
if err != nil {
    return fmt.Errorf("failed to query sensor readings: %w", err)
}

// Use errors.Is() for error checking
if errors.Is(err, sql.ErrNoRows) {
    return ErrDeviceNotFound
}
```

### Custom Error Types

```go
// Define package-level errors
var (
    ErrDeviceNotFound = errors.New("device not found")
    ErrInvalidParameter = errors.New("invalid parameter")
    ErrDatabaseUnavailable = errors.New("database unavailable")
)

// Use in returns
if len(readings) == 0 {
    return nil, ErrDeviceNotFound
}
```

### HTTP Error Mapping

```go
// Map domain errors to HTTP status codes
switch {
case errors.Is(err, ErrInvalidParameter):
    writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
case errors.Is(err, ErrDeviceNotFound):
    writeError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", err.Error())
default:
    writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
}
```

---

## Code Quality Requirements

### Mandatory Code Review Checklist

Before marking any code complete, verify:

- [ ] NO hardcoded values (use constants or config)
- [ ] NO god functions (>50 lines = split)
- [ ] NO commented-out code (delete it)
- [ ] ALL errors are handled or wrapped
- [ ] ALL public functions have godoc comments
- [ ] NO package-level variables (use config or dependency injection)
- [ ] ALL database queries use prepared statements
- [ ] ALL external calls have timeouts
- [ ] NO magic numbers (use named constants)

### Godoc Comments

```go
// GetSensorReadings retrieves the most recent N sensor readings for a device.
//
// It returns up to limit readings for the specified device_id, ordered by
// timestamp DESC (newest first). Results are cached for 30 seconds.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - deviceID: Device identifier to fetch readings for
//   - limit: Maximum number of readings to return (1-500)
//
// Returns:
//   - []SensorReading: Array of sensor readings (may be empty)
//   - error: ErrDeviceNotFound if no readings exist, error on failure
func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int) ([]SensorReading, error) {
    // Implementation...
}
```

### Context Usage

```go
// ALWAYS accept context as first parameter
func (r *SensorRepository) Query(ctx context.Context, deviceID string, limit int) ([]SensorReading, error) {
    // ALWAYS use context in database operations
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    rows, err := r.db.Query(ctx, query, deviceID, limit)
    // ...
}
```

---

## Database Query Patterns

### Prepared Statements

```go
// Use pgx's query API with prepared statements
const query = `
    SELECT id, device_id, timestamp, reading_type, value, unit, metadata
    FROM sensor_readings
    WHERE device_id = $1
    ORDER BY timestamp DESC
    LIMIT $2
`

rows, _ := r.db.Query(ctx, query, deviceID, limit)
```

### Connection Pooling

```go
// Configure pool in config
poolConfig.MaxConns = 25
poolConfig.MinConns = 5
poolConfig.MaxConnLifetime = 1 * time.Hour
poolConfig.MaxConnIdleTime = 30 * time.Minute
poolConfig.HealthCheckPeriod = 1 * time.Minute
```

### Query Performance

```go
// Use covering indexes for index-only scans
const query = `
    SELECT device_id, timestamp, reading_type, value, unit
    FROM sensor_readings
    WHERE device_id = $1
    ORDER BY timestamp DESC
    LIMIT $2
`

// EXCLUDE columns not in covering index (like id, metadata) unless needed
```

---

## Caching Patterns

### Cache Key Format

```go
// Use consistent cache key format
func cacheKey(deviceID string, limit int, readingType string) string {
    if readingType != "" {
        return fmt.Sprintf("sensor:%s:readings:%d:%s", deviceID, limit, readingType)
    }
    return fmt.Sprintf("sensor:%s:readings:%d", deviceID, limit)
}
```

**Examples:**
- `sensor:sensor-001:readings:10`
- `sensor:sensor-002:readings:50:temperature`

### Cache-Aside Pattern

```go
// Check cache first
data, err := c.cache.Get(ctx, key)
if err == nil {
    return data, nil // Cache hit
}

// Cache miss - query database
data, err = r.repository.Query(ctx, deviceID, limit)
if err != nil {
    return nil, err
}

// Populate cache
_ = c.cache.Set(ctx, key, data, 30*time.Second)
return data, nil
```

### Cache Configuration

- **TTL:** 30 seconds
- **Pattern:** Cache-aside (lazy population)
- **Eviction:** LRU (allkeys-lru)
- **Fallback:** Serve from database if cache unavailable

---

## Testing Expectations

### What to Test

- **Handler:** HTTP status codes, request parsing, response formatting
- **Service:** Business logic, cache interaction
- **Repository:** Query results, error handling
- **Integration:** End-to-end request flow

### What NOT to Test

- External libraries (assume they work)
- Database internals (test your queries, not PostgreSQL)
- HTTP framework details (test handlers, not chi router)

### Test Organization

```go
func TestSensorHandler_GetSensorReadings(t *testing.T) {
    tests := []struct {
        name           string
        deviceID       string
        limit          int
        expectedStatus int
        expectedError  string
    }{
        {
            name:           "valid request returns 200",
            deviceID:       "sensor-001",
            limit:          10,
            expectedStatus: http.StatusOK,
        },
        {
            name:           "missing device_id returns 400",
            deviceID:       "",
            limit:          10,
            expectedStatus: http.StatusBadRequest,
            expectedError:  "INVALID_PARAMETER",
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation...
        })
    }
}
```

---

## Performance Requirements

### Target Metrics

- **p50 latency:** ≤ 500ms (primary validation)
- **p95 latency:** ≤ 800ms
- **Cache hit rate:** ≥ 80%
- **Error rate:** ≤ 1%

### Optimization Priorities

1. Use covering indexes (index-only scans)
2. Maximize cache hits
3. Minimize database round-trips
4. Use connection pooling
5. Set appropriate timeouts

---

## Security Considerations

### Input Validation

```go
// ALWAYS validate user input
if limit < 1 || limit > 500 {
    return nil, ErrInvalidParameter
}

if !isValidDeviceID(deviceID) {
    return nil, ErrInvalidParameter
}

// Validate device ID format
func isValidDeviceID(deviceID string) bool {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, deviceID)
    return matched && len(deviceID) > 0 && len(deviceID) <= 50
}
```

### SQL Injection Prevention

```go
// ALWAYS use parameterized queries
// NEVER concatenate strings into SQL
const query = `WHERE device_id = $1` // ✓ Correct
// query := `WHERE device_id = '` + deviceID + `'` // ✗ WRONG - SQL INJECTION
```

### Environment Variables

```go
// NEVER log sensitive data
log.Printf("Connecting to database: %s", os.Getenv("DATABASE_URL")) // ✗ WRONG
log.Printf("Connecting to database") // ✓ Correct
```

---

## Dependencies

### Allowed Dependencies

```go
require (
    github.com/go-chi/chi/v5 v5.0.12          // HTTP router
    github.com/jackc/pgx/v5 v5.5.1            // PostgreSQL driver
    github.com/redis/go-redis/v9 v9.4.0        // Redis client
    github.com/google/uuid v1.5.0              // UUID generation
)
```

### Dependency Management

- Pin exact versions in go.mod
- Run `go mod tidy` after adding dependencies
- Review dependencies for security vulnerabilities
- Prefer standard library over external packages

---

## Complete Code Examples

### Complete Handler

```go
// internal/handler/sensor_handler.go
package handler

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/yourusername/highth/internal/model"
    "github.com/yourusername/highth/internal/service"
)

// SensorHandler handles HTTP requests for sensor readings
type SensorHandler struct {
    service *service.SensorService
}

// NewSensorHandler creates a new sensor handler
func NewSensorHandler(service *service.SensorService) *SensorHandler {
    return &SensorHandler{service: service}
}

// GetSensorReadings handles GET /api/v1/sensor-readings
func (h *SensorHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // Parse and validate parameters
    deviceID := r.URL.Query().Get("device_id")
    if deviceID == "" {
        h.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "device_id is required")
        return
    }

    limit := h.parseIntOrDefault(r.URL.Query().Get("limit"), 10)
    if limit < 1 || limit > 500 {
        h.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "limit must be between 1 and 500")
        return
    }

    readingType := r.URL.Query().Get("reading_type")

    // Call service layer
    readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)
    if err != nil {
        h.handleServiceError(w, err)
        return
    }

    // Return response
    h.writeResponse(w, http.StatusOK, map[string]interface{}{
        "data": readings,
        "meta": map[string]interface{}{
            "count":        len(readings),
            "limit":        limit,
            "device_id":    deviceID,
            "reading_type": readingType,
        },
    }, start)
}

// Helper methods (private)
func (h *SensorHandler) parseIntOrDefault(s string, defaultVal int) int {
    if s == "" {
        return defaultVal
    }
    var val int
    if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
        return defaultVal
    }
    return val
}

func (h *SensorHandler) writeResponse(w http.ResponseWriter, status int, data interface{}, start time.Time) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func (h *SensorHandler) writeError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": map[string]interface{}{
            "code":    code,
            "message": message,
            "timestamp": time.Now().Format(time.RFC3339),
        },
    })
}

func (h *SensorHandler) handleServiceError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, service.ErrInvalidParameter):
        h.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
    case errors.Is(err, service.ErrDeviceNotFound):
        h.writeError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", err.Error())
    default:
        h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
    }
}
```

### Complete Service

```go
// internal/service/sensor_service.go
package service

import (
    "context"
    "errors"
    "fmt"

    "github.com/yourusername/highth/internal/cache"
    "github.com/yourusername/highth/internal/model"
    "github.com/yourusername/highth/internal/repository"
)

var (
    ErrInvalidParameter = errors.New("invalid parameter")
    ErrDeviceNotFound   = errors.New("device not found")
)

// SensorService handles business logic for sensor readings
type SensorService struct {
    repo   *repository.SensorRepository
    cache  *cache.RedisCache
}

// NewSensorService creates a new sensor service
func NewSensorService(repo *repository.SensorRepository, cache *cache.RedisCache) *SensorService {
    return &SensorService{
        repo:  repo,
        cache: cache,
    }
}

// GetSensorReadings retrieves the most recent N sensor readings for a device.
//
// Results are cached for 30 seconds. Cache-aside pattern is used.
func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // Validate input
    if !s.isValidDeviceID(deviceID) {
        return nil, fmt.Errorf("%w: invalid device_id", ErrInvalidParameter)
    }

    // Check cache first
    key := cacheKey(deviceID, limit, readingType)
    if readings, err := s.cache.Get(ctx, key); err == nil {
        return readings, nil
    }

    // Cache miss - query database
    readings, err := s.repo.Query(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, fmt.Errorf("failed to query sensor readings: %w", err)
    }

    if len(readings) == 0 {
        return nil, fmt.Errorf("%w: device %s", ErrDeviceNotFound, deviceID)
    }

    // Populate cache (fire and forget)
    _ = s.cache.Set(ctx, key, readings, 30*time.Second)

    return readings, nil
}

func (s *SensorService) isValidDeviceID(deviceID string) bool {
    if len(deviceID) == 0 || len(deviceID) > 50 {
        return false
    }
    // Check for valid characters (alphanumeric, hyphen, underscore)
    for _, c := range deviceID {
        if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
             (c >= '0' && c <= '9') || c == '-' || c == '_') {
            return false
        }
    }
    return true
}

func cacheKey(deviceID string, limit int, readingType string) string {
    if readingType != "" {
        return fmt.Sprintf("sensor:%s:readings:%d:%s", deviceID, limit, readingType)
    }
    return fmt.Sprintf("sensor:%s:readings:%d", deviceID, limit)
}
```

### Complete Repository

```go
// internal/repository/sensor_repo.go
package repository

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/yourusername/highth/internal/model"
)

// SensorRepository handles database queries for sensor readings
type SensorRepository struct {
    db *pgxpool.Pool
}

// NewSensorRepository creates a new sensor repository
func NewSensorRepository(db *pgxpool.Pool) *SensorRepository {
    return &SensorRepository{db: db}
}

// Query retrieves sensor readings from the database
func (r *SensorRepository) Query(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    const baseQuery = `
        SELECT id, device_id, timestamp, reading_type, value, unit, metadata
        FROM sensor_readings
        WHERE device_id = $1
    `

    var query string
    var args []interface{}
    args = append(args, deviceID)

    if readingType != "" {
        query = baseQuery + ` AND reading_type = $2 ORDER BY timestamp DESC LIMIT $3`
        args = append(args, readingType, limit)
    } else {
        query = baseQuery + ` ORDER BY timestamp DESC LIMIT $2`
        args = append(args, limit)
    }

    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()

    var readings []model.SensorReading
    for rows.Next() {
        var r model.SensorReading
        if err := rows.Scan(&r.ID, &r.DeviceID, &r.Timestamp, &r.ReadingType, &r.Value, &r.Unit, &r.Metadata); err != nil {
            return nil, fmt.Errorf("scan failed: %w", err)
        }
        readings = append(readings, r)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows error: %w", err)
    }

    return readings, nil
}
```

### Complete Model

```go
// internal/model/sensor.go
package model

import (
    "time"
)

// SensorReading represents a single sensor reading from the database
type SensorReading struct {
    ID          string          `json:"id"`
    DeviceID    string          `json:"device_id"`
    Timestamp   time.Time       `json:"timestamp"`
    ReadingType string          `json:"reading_type"`
    Value       float64         `json:"value"`
    Unit        string          `json:"unit"`
    Metadata    map[string]any  `json:"metadata,omitempty"`
}

// HealthStatus represents the health check response
type HealthStatus struct {
    Status    string             `json:"status"`
    Timestamp string             `json:"timestamp"`
    Checks    map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents a single health check result
type HealthCheck struct {
    Status     string `json:"status"`
    LatencyMs  int64  `json:"latency_ms,omitempty"`
    Error      string `json:"error,omitempty"`
}
```

---

**Document Version:** 1.0
**Last Updated:** 2026-03-11
**Required Reading:** ALL agents must read this before writing any code
