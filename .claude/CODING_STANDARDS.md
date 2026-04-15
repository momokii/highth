# Coding Standards — Higth Project

Conventions derived from the actual implemented codebase. Follow these patterns exactly.

---

## Architecture

```
cmd/api/main.go          → Entry point, dependency wiring, graceful shutdown
internal/config/         → Environment variable loading (godotenv)
internal/handler/        → HTTP handlers (chi router) — HTTP concerns ONLY
internal/service/        → Business logic + cache orchestration
internal/repository/     → PostgreSQL queries (pgx/v5 pool)
internal/cache/          → Redis cache (go-redis/v9, JSON serialization)
internal/middleware/     → Gzip compression, Prometheus metrics, /metrics endpoint
internal/model/          → Data structures (no logic)
```

**Layer rule**: Handler never touches DB. Service never returns HTTP codes. Repository never touches cache. No circular imports.

---

## Package Map (Real Files)

| Package | Files | Purpose |
|---------|-------|---------|
| `cmd/api` | `main.go` | Entrypoint, router setup, graceful shutdown |
| `config` | `config.go` | `Load()` → `*Config` from env vars |
| `handler` | `sensor_handler.go`, `health_handler.go` | HTTP parsing, response formatting |
| `service` | `sensor_service.go` | Business logic, cache-aside, validation |
| `repository` | `sensor_repo.go` | Parameterized SQL queries, pool management |
| `cache` | `redis_cache.go` | Get/Set/Delete/FlushAll with JSON serialization |
| `model` | `sensor.go`, `health.go` | Structs with JSON tags |
| `middleware` | `compression.go`, `metrics.go`, `prometheus.go` | Chi middleware |

---

## Naming Conventions

| Element | Convention | Examples |
|---------|-----------|----------|
| Files | snake_case | `sensor_handler.go`, `redis_cache.go` |
| Packages | lowercase single word | `handler`, `service`, `cache` |
| Exported types | PascalCase | `SensorHandler`, `RedisCache`, `SensorReading` |
| Exported functions | PascalCase verbs | `GetSensorReadings`, `NewSensorHandler` |
| Private functions | camelCase | `isValidDeviceID`, `parseIntOrDefault` |
| Local variables | camelCase | `deviceID`, `cacheKey`, `readings` |
| Constants | PascalCase | `ReadingTypeTemperature`, `HealthStatusHealthy` |
| Env vars | UPPER_SNAKE_CASE | `DATABASE_URL`, `REDIS_TTL` |
| Config struct fields | PascalCase | `DatabaseURL`, `RedisEnabled` |

---

## Error Handling

### Sentinel Errors (defined in `internal/service/sensor_service.go`)

```go
var (
    ErrInvalidParameter = errors.New("invalid parameter")
    ErrDeviceNotFound   = errors.New("device not found")
    ErrReadingNotFound  = errors.New("reading not found")
)
```

### Wrapping Pattern

```go
// Service wraps sentinel errors with context
return "", nil, fmt.Errorf("%w: invalid device_id", ErrInvalidParameter)
return "", nil, fmt.Errorf("%w: no readings found for device_id: %s", ErrDeviceNotFound, deviceID)
```

### Handler Error Mapping (from `sensor_handler.go`)

```go
func (h *SensorHandler) handleServiceError(w, r, err, start) {
    switch {
    case errors.Is(err, service.ErrInvalidParameter):
        h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", ...)
    case errors.Is(err, service.ErrDeviceNotFound):
        h.writeError(w, r, http.StatusNotFound, "DEVICE_NOT_FOUND", ...)
    case errors.Is(err, service.ErrReadingNotFound):
        h.writeError(w, r, http.StatusNotFound, "READING_NOT_FOUND", ...)
    default:
        h.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", ...)
    }
}
```

### Repository Not-Found Pattern

```go
// Repository returns nil, nil for not-found (service interprets)
if err == pgx.ErrNoRows {
    return nil, nil
}
```

---

## HTTP Response Format

### Success Response

```json
{
  "data": [ ... ],
  "meta": {
    "count": 10,
    "limit": 10,
    "device_id": "sensor-001",
    "reading_type": "temperature"
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "no readings found for device_id: sensor-999",
    "timestamp": "2026-04-15T10:30:00Z",
    "details": {
      "parameter": "device_id",
      "provided": "sensor 001",
      "constraints": {"rule": "alphanumeric, hyphens, underscores only"}
    }
  }
}
```

### Response Headers (always set)

```
Content-Type: application/json
X-Response-Time: 45
X-Cache-Status: HIT|MISS|BYPASS
X-Request-ID: abc123
Cache-Control: public, max-age=30
```

---

## Database Patterns

### Parameterized Queries

```go
// Dynamic WHERE clause with argIdx counter
baseQuery += fmt.Sprintf(" WHERE device_id = $%d", argIdx)
args = append(args, deviceID)
argIdx++
```

### Context Timeout

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

### ID Conversion

```go
// DB stores int64, JSON returns string
var id int64
rows.Scan(&id, ...)
r.ID = fmt.Sprintf("%d", id)
```

### Stats from Materialized Views

```go
// Uses TABLESAMPLE for device count estimation
SELECT COUNT(DISTINCT device_id) FROM (
    SELECT device_id FROM sensor_readings TABLESAMPLE SYSTEM (0.5)
) sample
```

---

## Cache Patterns

### Cache-Aside in Service

```go
// 1. Check cache
if cache.Get(ctx, key, &cached) == nil {
    return "HIT", cached, nil
}
// 2. Query DB on miss
readings, err := s.repo.Query(ctx, ...)
// 3. Populate cache (fire-and-forget)
_ = s.cache.Set(ctx, key, readings)
return "MISS", readings, nil
```

### Cache Key Format

```
sensor:{device_id}:readings:{limit}[:{reading_type}][:{from_unix}][:{to_unix}]
sensor:id:{id}
```

### Stats Bypass

```go
// Stats endpoint always bypasses cache
h.writeResponse(w, r, 200, data, start, "BYPASS")
```

---

## Config Pattern

```go
// Load from env via godotenv with defaults
cfg := &Config{
    DatabaseURL: getEnv("DATABASE_URL", ""),
    RedisTTL:    getEnvAsDuration("REDIS_TTL", 30*time.Second),
    CacheEnabled: getEnvAsBool("CACHE_ENABLED", true),
}
// Only DATABASE_URL is required
if cfg.DatabaseURL == "" {
    return nil, fmt.Errorf("DATABASE_URL is required")
}
```

---

## Middleware Chain (order matters)

```go
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
r.Use(middleware.Timeout(cfg.RequestTimeout))
r.Use(higthmiddleware.GzipMiddleware)
r.Use(middleware.SetHeader("Content-Type", "application/json"))
r.Use(higthmiddleware.MetricsMiddleware)
```

---

## Dependencies (pinned, from go.mod)

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `go-chi/chi/v5` | v5.2.5 | HTTP router |
| `jackc/pgx/v5` | v5.8.0 | PostgreSQL driver (pgxpool) |
| `joho/godotenv` | v1.5.1 | .env file loading |
| `prometheus/client_golang` | v1.23.2 | Prometheus metrics |
| `redis/go-redis/v9` | v9.18.0 | Redis client |

Go version: 1.25.7

---

## Testing

- **Benchmark testing**: k6 via Docker. Scenarios in `tests/scenarios/`. Run via `tests/run-benchmarks.sh`.
- **Go unit tests**: None exist yet. When adding: table-driven with `t.Run()`, mock interfaces for repository/cache.
