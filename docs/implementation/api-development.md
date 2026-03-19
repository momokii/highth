# API Development Guide

This guide covers building the complete Go API layer with chi router, pgx connection pooling, and integration points for caching.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Project Structure](#project-structure)
- [Dependencies](#dependencies)
- [Configuration Management](#configuration-management)
- [Connection Pool Initialization](#connection-pool-initialization)
- [Core Query Implementation](#core-query-implementation)
- [Response Serialization](#response-serialization)
- [Error Handling](#error-handling)
- [Health Check Endpoint](#health-check-endpoint)
- [Local Smoke Testing](#local-smoke-testing)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting API development, ensure:

- [ ] Phase 1 (Database Setup) complete
- [ ] Database `sensor_db` exists and is accessible
- [ ] Table `sensor_readings` created with indexes
- [ ] Go 1.21+ installed and verified
- [ ] Phase 0 (Environment Setup) complete
- [ ] Project directory structure created

---

## Project Structure

### Recommended Go Project Layout

```
highth/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── handler/
│   │   ├── sensor_readings.go      # GET /api/v1/sensor-readings handler
│   │   └── health.go               # GET /health handler
│   ├── service/
│   │   └── sensor_service.go       # Business logic layer
│   ├── repository/
│   │   └── postgres.go             # Database queries (pgx)
│   ├── cache/
│   │   └── redis_cache.go          # Cache operations wrapper
│   ├── model/
│   │   ├── sensor_reading.go       # Data structures
│   │   └── error.go                # Error models
│   └── config/
│       └── config.go               # Configuration loading
├── pkg/
│   └── telemetry/                  # Shared utilities (optional)
├── go.mod
├── go.sum
├── .env
├── Dockerfile
└── docker-compose.yml
```

### Directory Purpose

| Directory | Purpose | Visibility |
|-----------|---------|------------|
| `cmd/api` | Application entry point | Public executable |
| `internal/handler` | HTTP request handlers, chi routes | Private to this app |
| `internal/service` | Business logic, orchestration | Private to this app |
| `internal/repository` | Database access abstraction | Private to this app |
| `internal/cache` | Cache operations wrapper | Private to this app |
| `internal/model` | Shared data structures | Private to this app |
| `internal/config` | Configuration loading | Private to this app |
| `pkg` | Code that could be reused | Public library code |

### Why This Structure?

**Standard Go Project Layout:**
- `cmd/` separates the entry point from the library code
- `internal/` prevents import by external projects (Go compiler enforcement)
- `pkg/` for reusable code (if needed for future projects)

**Layered Architecture:**
- **Handler** → HTTP concerns, request validation, response formatting
- **Service** → Business logic, caching orchestration
- **Repository** → Data access, SQL generation
- **Model** → Shared data structures across layers

This separation enables:
- Independent testing of each layer
- Easy swapping of implementations (e.g., cache provider)
- Clear ownership of concerns

---

## Dependencies

### Core Dependencies (go.mod)

```go
module github.com/yourusername/highth

go 1.21

require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/jackc/pgx/v5 v5.5.1
    github.com/redis/go-redis/v9 v9.4.0
    github.com/google/uuid v1.5.0
)
```

### Dependency Roles

| Package | Version | Purpose | Why This Version? |
|---------|---------|---------|-------------------|
| `github.com/go-chi/chi/v5` | v5.0.12 | HTTP router | Lightweight, composable, idiomatic Go |
| `github.com/jackc/pgx/v5` | v5.5.1 | PostgreSQL driver | Best Go PG driver; connection pooling; binary protocol |
| `github.com/redis/go-redis/v9` | v9.4.0 | Redis client | Official Redis client for Go; supports Redis 7+ |
| `github.com/google/uuid` | v1.5.0 | UUID generation | Request ID generation for tracing |

### Installing Dependencies

```bash
# Initialize module (if not already done)
go mod init github.com/yourusername/highth

# Download dependencies
go mod tidy

# Verify dependencies
go mod verify

# Build to ensure everything compiles
go build ./cmd/api
```

---

## Configuration Management

### Environment Variables (.env)

```bash
# .env file (DO NOT commit to git)
# Server Configuration
PORT=8080
HOST=0.0.0.0

# Database Configuration
DATABASE_URL=postgres://sensor_user:your_password_here@localhost:5432/sensor_db
DB_MAX_CONNECTIONS=25
DB_MIN_CONNECTIONS=5

# Redis Configuration
REDIS_URL=redis://localhost:6379
REDIS_ENABLED=true
REDIS_TTL=30s

# Cache Configuration
CACHE_ENABLED=true

# Application Configuration
LOG_LEVEL=info
REQUEST_TIMEOUT=30s
```

### Configuration Loading Pattern

```go
// internal/config/config.go

package config

import (
    "os"
    "strconv"
    "time"
)

type Config struct {
    // Server
    ServerPort    int
    ServerHost    string

    // Database
    DatabaseURL   string
    DBMaxConn     int
    DBMinConn     int

    // Redis/Cache
    RedisURL      string
    RedisEnabled  bool
    CacheEnabled  bool
    CacheTTL      time.Duration

    // Application
    LogLevel      string
    RequestTimeout time.Duration
}

// Load reads configuration from environment variables
// Uses sensible defaults if variables not set
func Load() *Config {
    return &Config{
        ServerPort:     getEnvInt("PORT", 8080),
        ServerHost:     getEnv("HOST", "0.0.0.0"),
        DatabaseURL:    getEnv("DATABASE_URL", "postgres://sensor_user:password@localhost:5432/sensor_db"),
        DBMaxConn:      getEnvInt("DB_MAX_CONNECTIONS", 25),
        DBMinConn:      getEnvInt("DB_MIN_CONNECTIONS", 5),
        RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
        RedisEnabled:   getEnvBool("REDIS_ENABLED", true),
        CacheEnabled:   getEnvBool("CACHE_ENABLED", true),
        CacheTTL:       getEnvDuration("CACHE_TTL", 30*time.Second),
        LogLevel:       getEnv("LOG_LEVEL", "info"),
        RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
    }
}

// Helper functions for reading environment variables with defaults

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        if boolVal, err := strconv.ParseBool(value); err == nil {
            return boolVal
        }
    }
    return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if duration, err := time.ParseDuration(value); err == nil {
            return duration
        }
    }
    return defaultValue
}
```

### Usage in main()

```go
// cmd/api/main.go

import "github.com/yourusername/highth/internal/config"

func main() {
    // Load configuration
    cfg := config.Load()

    // Use configuration
    log.Printf("Starting server on %s:%d", cfg.ServerHost, cfg.ServerPort)

    // Initialize database connection pool with config
    pool := setupDB(cfg)

    // Initialize Redis client with config
    redisClient := setupRedis(cfg)

    // Start HTTP server
    server := setupServer(cfg, pool, redisClient)
}
```

---

## Connection Pool Initialization

### pgxpool Pattern

```go
// internal/repository/postgres.go

package repository

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
    pool *pgxpool.Pool
}

// NewPostgresRepository creates a new connection pool
// The pool is configured with min/max connections from config
func NewPostgresRepository(ctx context.Context, databaseURL string, maxConn, minConn int) (*PostgresRepository, error) {
    // Parse connection string
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("unable to parse database URL: %w", err)
    }

    // Configure pool settings
    config.MaxConns = int32(maxConn)      // Maximum connections (default: 4)
    config.MinConns = int32(minConn)      // Minimum connections to maintain (default: 0)
    config.MaxConnLifetime = 1 * time.Hour  // Recreate connections after 1 hour
    config.MaxConnIdleTime = 30 * time.Minute  // Close idle connections after 30 minutes
    config.HealthCheckPeriod = 1 * time.Minute  // Check connection health every minute

    // Create connection pool
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("unable to create connection pool: %w", err)
    }

    // Verify connection with ping
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("unable to ping database: %w", err)
    }

    log.Printf("Database connection pool created: min=%d, max=%d",
        minConn, maxConn)

    return &PostgresRepository{
        pool: pool,
    }, nil
}

// Close closes the connection pool
// Should be called with defer in main()
func (r *PostgresRepository) Close() {
    r.pool.Close()
    log.Println("Database connection pool closed")
}
```

### Connection Pool Configuration Rationale

| Parameter | Value | Reason |
|-----------|-------|--------|
| `MaxConns` | 25 | Limits database connections; prevents exhaustion |
| `MinConns` | 5 | Keeps some connections warm; reduces cold start latency |
| `MaxConnLifetime` | 1 hour | Recreates connections periodically (prevents stale connections) |
| `MaxConnIdleTime` | 30 minutes | Closes idle connections (frees resources) |
| `HealthCheckPeriod` | 1 minute | Detects broken connections quickly |

---

## Core Query Implementation

### Request Flow Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           HTTP Request Received                              │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
                         ┌─────────────────────────────┐
                         │  Handler Layer               │
                         │  - Parse query parameters    │
                         │  - Validate input            │
                         │  - Call service layer        │
                         └─────────────┬───────────────┘
                                       │
                                       ▼
                         ┌─────────────────────────────┐
                         │  Service Layer               │
                         │  - Check cache (if enabled)  │
                         │  - Call repository if miss   │
                         │  - Populate cache            │
                         │  - Return result             │
                         └─────────────┬───────────────┘
                                       │
                                       ▼
                         ┌─────────────────────────────┐
                         │  Repository Layer            │
                         │  - Execute SQL query         │
                         │  - Map rows to structs       │
                         │  - Return to service         │
                         └─────────────┬───────────────┘
                                       │
                                       ▼
                         ┌─────────────────────────────┐
                         │  Response Serialization      │
                         │  - Format JSON response      │
                         │  - Set HTTP status code     │
                         │  - Write to client          │
                         └─────────────────────────────┘
```

### Handler Layer (Pseudocode)

```go
// internal/handler/sensor_readings.go

package handler

import (
    "net/http"
    "strconv"
)

type SensorReadingsHandler struct {
    service *service.SensorService
}

func NewSensorReadingsHandler(service *service.SensorService) *SensorReadingsHandler {
    return &SensorReadingsHandler{
        service: service,
    }
}

// GetSensorReadings handles GET /api/v1/sensor-readings
func (h *SensorReadingsHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
    // 1. Extract query parameters
    deviceID := r.URL.Query().Get("device_id")
    limitStr := r.URL.Query().Get("limit")
    readingType := r.URL.Query().Get("reading_type")

    // 2. Validate required parameter
    if deviceID == "" {
        // Return 400 Bad Request
        respondError(w, http.StatusBadRequest, "INVALID_PARAMETER", "device_id is required")
        return
    }

    // 3. Parse and validate optional limit parameter
    limit := 10  // default
    if limitStr != "" {
        parsedLimit, err := strconv.Atoi(limitStr)
        if err != nil || parsedLimit < 1 || parsedLimit > 500 {
            // Return 400 Bad Request
            respondError(w, http.StatusBadRequest, "INVALID_PARAMETER", "limit must be between 1 and 500")
            return
        }
        limit = parsedLimit
    }

    // 4. Call service layer
    readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)
    if err != nil {
        // Check error type and respond appropriately
        if errors.Is(err, service.ErrDeviceNotFound) {
            respondError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "No readings found for device_id: "+deviceID)
        } else {
            respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An error occurred while fetching readings")
        }
        return
    }

    // 5. Format and send response
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data": readings,
        "meta": map[string]interface{}{
            "count":     len(readings),
            "limit":     limit,
            "device_id": deviceID,
        },
    })
}
```

### Service Layer (Pseudocode)

```go
// internal/service/sensor_service.go

package service

import (
    "context"
    "errors"
    "fmt"
)

var ErrDeviceNotFound = errors.New("device not found")

type SensorService struct {
    repo   *repository.PostgresRepository
    cache  *cache.RedisCache
    enabled bool
}

func NewSensorService(repo *repository.PostgresRepository, cache *cache.RedisCache, cacheEnabled bool) *SensorService {
    return &SensorService{
        repo:   repo,
        cache:  cache,
        enabled: cacheEnabled,
    }
}

func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // 1. Check cache if enabled
    if s.enabled && s.cache != nil {
        cacheKey := s.buildCacheKey(deviceID, limit, readingType)

        if cached, found := s.cache.Get(ctx, cacheKey); found {
            // Return cached data
            return cached, nil
        }
    }

    // 2. Query database (cache miss or cache disabled)
    readings, err := s.repo.GetSensorReadings(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, err
    }

    // 3. Check if any results returned
    if len(readings) == 0 {
        return nil, ErrDeviceNotFound
    }

    // 4. Populate cache asynchronously (non-blocking)
    if s.enabled && s.cache != nil {
        cacheKey := s.buildCacheKey(deviceID, limit, readingType)
        // Fire and forget - don't wait for cache write
        go func() {
            cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer cancel()
            s.cache.Set(cacheCtx, cacheKey, readings, 30*time.Second)
        }()
    }

    return readings, nil
}

func (s *SensorService) buildCacheKey(deviceID string, limit int, readingType string) string {
    if readingType != "" {
        return fmt.Sprintf("sensor:%s:readings:%d:%s", deviceID, limit, readingType)
    }
    return fmt.Sprintf("sensor:%s:readings:%d", deviceID, limit)
}
```

### Repository Layer (Pseudocode)

```go
// internal/repository/postgres.go (continued)

func (r *PostgresRepository) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // Build dynamic query based on reading_type filter
    query := `
        SELECT
            id,
            device_id,
            timestamp,
            reading_type,
            value,
            unit,
            metadata
        FROM sensor_readings
        WHERE device_id = $1
    `
    args := []interface{}{deviceID}
    paramIndex := 2

    // Add reading_type filter if provided
    if readingType != "" {
        query += fmt.Sprintf(" AND reading_type = $%d", paramIndex)
        args = append(args, readingType)
        paramIndex++
    }

    // Add ORDER BY and LIMIT
    query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", paramIndex)
    args = append(args, limit)

    // Execute query
    rows, err := r.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()  // Important: always close rows to return connection to pool

    // Map rows to structs
    var readings []model.SensorReading
    for rows.Next() {
        var reading model.SensorReading
        err := rows.Scan(
            &reading.ID,
            &reading.DeviceID,
            &reading.Timestamp,
            &reading.ReadingType,
            &reading.Value,
            &reading.Unit,
            &reading.Metadata,
        )
        if err != nil {
            return nil, fmt.Errorf("scan failed: %w", err)
        }
        readings = append(readings, reading)
    }

    // Check for iteration errors
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %w", err)
    }

    return readings, nil
}
```

### Query Optimization Notes

**Why This Query Pattern?**

1. **Parameterized queries** → Prevents SQL injection, enables prepared statement caching
2. **ORDER BY timestamp DESC** → Matches composite index `(device_id, timestamp DESC)`
3. **LIMIT clause** → Reduces result set size, bounded by 500
4. **defer rows.Close()** → Ensures connection returns to pool even on error
5. **Dynamic query building** → Handles optional reading_type filter efficiently

**Index-Only Scan Expected:**

With the covering index `idx_sensor_readings_device_covering`, PostgreSQL should:
- Scan only the index (no heap access)
- Return `reading_type`, `value`, `unit` from index
- Eliminate random I/O to table

Verify with `EXPLAIN ANALYZE`:
```sql
EXPLAIN ANALYZE
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;
```

Look for: `Index Only Scan using idx_sensor_readings_device_covering`

---

## Response Serialization

### Data Models

```go
// internal/model/sensor_reading.go

package model

import "time"

type SensorReading struct {
    ID          string                 `json:"id"`
    DeviceID    string                 `json:"device_id"`
    Timestamp   time.Time              `json:"timestamp"`
    ReadingType string                 `json:"reading_type"`
    Value       float64                `json:"value"`
    Unit        string                 `json:"unit"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ErrorResponse struct {
    Error struct {
        Code      string    `json:"code"`
        Message   string    `json:"message"`
        Timestamp time.Time `json:"timestamp"`
    } `json:"error"`
}

type SuccessResponse struct {
    Data []SensorReading `json:"data"`
    Meta struct {
        Count    int    `json:"count"`
        Limit    int    `json:"limit"`
        DeviceID string `json:"device_id"`
    } `json:"meta"`
}
```

### JSON Response Helpers

```go
// internal/handler/response.go

package handler

import (
    "encoding/json"
    "net/http"
    "time"
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)

    if err := json.NewEncoder(w).Encode(payload); err != nil {
        // If encoding fails, we can't do much more than log
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}

func respondError(w http.ResponseWriter, status int, code string, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)

    errorResponse := struct {
        Error struct {
            Code      string `json:"code"`
            Message   string `json:"message"`
            Timestamp string `json:"timestamp"`
        } `json:"error"`
    }{}

    errorResponse.Error.Code = code
    errorResponse.Error.Message = message
    errorResponse.Error.Timestamp = time.Now().Format(time.RFC3339)

    json.NewEncoder(w).Encode(errorResponse)
}
```

### Response Examples

**Success Response (200 OK):**
```json
{
  "data": [
    {
      "id": "12345678",
      "device_id": "sensor-001",
      "timestamp": "2024-01-15T10:30:00Z",
      "reading_type": "temperature",
      "value": 23.45,
      "unit": "celsius",
      "metadata": {
        "firmware_version": "2.1.0",
        "battery_level": 87
      }
    }
  ],
  "meta": {
    "count": 10,
    "limit": 10,
    "device_id": "sensor-001"
  }
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "device_id is required",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

**Error Response (404 Not Found):**
```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "No readings found for device_id: sensor-999",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

---

## Error Handling

### Error Types and HTTP Status Codes

| Error Type | HTTP Status | Error Code | Handler Response |
|------------|-------------|------------|------------------|
| Missing required parameter | 400 | INVALID_PARAMETER | "device_id is required" |
| Invalid parameter format | 400 | INVALID_PARAMETER | "limit must be between 1 and 500" |
| Device not found | 404 | DEVICE_NOT_FOUND | "No readings found for device_id: {id}" |
| Database connection error | 500 | INTERNAL_ERROR | "An error occurred while fetching readings" |
| Query timeout | 504 | TIMEOUT | "Request timed out" |
| Cache error | Log only | N/A | Silently fall back to DB |

### Error Handling Strategy

```go
// internal/handler/sensor_readings.go (error handling pattern)

func (h *SensorReadingsHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
    // Input validation errors → 400
    if deviceID == "" {
        respondError(w, http.StatusBadRequest, "INVALID_PARAMETER", "device_id is required")
        return
    }

    // Call service
    readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)

    // Business logic errors → 404
    if errors.Is(err, service.ErrDeviceNotFound) {
        respondError(w, http.StatusNotFound, "DEVICE_NOT_FOUND",
            fmt.Sprintf("No readings found for device_id: %s", deviceID))
        return
    }

    // Database/system errors → 500
    if err != nil {
        // Log the actual error for debugging
        log.Printf("ERROR: Failed to fetch readings: %v", err)
        // Return generic error to client
        respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
            "An error occurred while fetching readings")
        return
    }

    // Success → 200
    respondJSON(w, http.StatusOK, formatResponse(readings, limit, deviceID))
}
```

### Context Timeout Pattern

```go
// Service layer with timeout

func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // Create context with timeout
    queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Pass context with timeout to repository
    readings, err := s.repo.GetSensorReadings(queryCtx, deviceID, limit, readingType)
    if err != nil {
        // Check if error is timeout
        if errors.Is(err, context.DeadlineExceeded) {
            return nil, fmt.Errorf("database query timed out")
        }
        return nil, err
    }

    return readings, nil
}
```

### Graceful Degradation for Cache Errors

```go
// Cache errors should not break the API

func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // Check cache
    if s.enabled && s.cache != nil {
        cacheKey := s.buildCacheKey(deviceID, limit, readingType)

        // Cache errors are logged but don't fail the request
        if cached, found := s.cache.Get(ctx, cacheKey); found {
            return cached, nil
        }
    }

    // Always fall back to database on cache miss or error
    readings, err := s.repo.GetSensorReadings(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, err
    }

    // Try to populate cache, but ignore errors
    if s.enabled && s.cache != nil {
        go func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Cache populate panic: %v", r)
                }
            }()
            // Cache write failure is not critical
            _ = s.cache.Set(context.Background(), cacheKey, readings, 30*time.Second)
        }()
    }

    return readings, nil
}
```

---

## Health Check Endpoint

### Health Check Implementation

```go
// internal/handler/health.go

package handler

import (
    "context"
    "database/sql"
    "encoding/json"
    "net/http"
    "time"
)

type HealthHandler struct {
    db       *pgxpool.Pool
    redis    *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
    return &HealthHandler{
        db:    db,
        redis: redis,
    }
}

type HealthStatus struct {
    Status   string                 `json:"status"`
    Timestamp string                `json:"timestamp"`
    Checks   map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
    Latency string `json:"latency,omitempty"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    checks := make(map[string]CheckResult)
    overallStatus := "healthy"

    // 1. Check database
    dbStart := time.Now()
    err := h.db.Ping(ctx)
    if err != nil {
        checks["database"] = CheckResult{
            Status:  "unhealthy",
            Message: err.Error(),
        }
        overallStatus = "degraded"
    } else {
        // Verify we can query the table
        var count int64
        err = h.db.QueryRow(ctx, "SELECT count(*) FROM sensor_readings LIMIT 1").Scan(&count)
        if err != nil {
            checks["database"] = CheckResult{
                Status:  "degraded",
                Message: "Ping OK but query failed: " + err.Error(),
            }
            overallStatus = "degraded"
        } else {
            checks["database"] = CheckResult{
                Status:  "healthy",
                Latency: time.Since(dbStart).String(),
            }
        }
    }

    // 2. Check Redis (if configured)
    if h.redis != nil {
        redisStart := time.Now()
        err = h.redis.Ping(ctx).Err()
        if err != nil {
            checks["redis"] = CheckResult{
                Status:  "unhealthy",
                Message: err.Error(),
            }
            overallStatus = "degraded"
        } else {
            checks["redis"] = CheckResult{
                Status:  "healthy",
                Latency: time.Since(redisStart).String(),
            }
        }
    }

    // 3. Check connection pool
    stats := h.db.Stat()
    checks["connection_pool"] = CheckResult{
        Status: "healthy",
    }

    // Set HTTP status based on overall health
    statusCode := http.StatusOK
    if overallStatus == "degraded" {
        statusCode = http.StatusServiceUnavailable  // 503
    }

    // Build response
    response := HealthStatus{
        Status:    overallStatus,
        Timestamp: time.Now().Format(time.RFC3339),
        Checks:    checks,
    }

    // Add pool stats as extra info
    response.Checks["connection_pool"].Message = fmt.Sprintf(
        "Acquire: %d/%d (idle: %d, max: %d)",
        stats.TotalConns(),
        stats.MaxConns(),
        stats.IdleConns(),
    )

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(response)
}
```

### Health Check Responses

**Healthy Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": {
      "status": "healthy",
      "latency": "5ms"
    },
    "redis": {
      "status": "healthy",
      "latency": "2ms"
    },
    "connection_pool": {
      "status": "healthy",
      "message": "Acquire: 5/25 (idle: 3, max: 25)"
    }
  }
}
```

**Degraded Response (503 Service Unavailable):**
```json
{
  "status": "degraded",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": {
      "status": "healthy",
      "latency": "8ms"
    },
    "redis": {
      "status": "unhealthy",
      "message": "connection refused"
    },
    "connection_pool": {
      "status": "healthy"
    }
  }
}
```

### Routing the Health Check

```go
// cmd/api/main.go

import "github.com/go-chi/chi/v5"

func setupServer(cfg *config.Config, db *pgxpool.Pool, redis *redis.Client) *http.Server {
    r := chi.NewRouter()

    // Health check (no authentication needed)
    r.Get("/health", NewHealthHandler(db, redis))

    // API routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/sensor-readings", NewSensorReadingsHandler(service))
    })

    server := &http.Server{
        Addr:    fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort),
        Handler: r,
    }

    return server
}
```

---

## Local Smoke Testing

### Basic Smoke Test Commands

```bash
# 1. Start the API server
go run cmd/api/main.go

# Expected output:
# 2024/01/15 10:30:00 Database connection pool created: min=5, max=25
# 2024/01/15 10:30:00 Starting server on 0.0.0.0:8080

# 2. Test health check
curl http://localhost:8080/health

# Expected output (formatted):
# {
#   "status": "healthy",
#   "timestamp": "2024-01-15T10:30:00Z",
#   "checks": {
#     "database": { "status": "healthy", "latency": "5ms" },
#     "redis": { "status": "healthy", "latency": "2ms" }
#   }
# }

# 3. Test valid sensor reading request
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Expected output:
# {
#   "data": [
#     { "id": "...", "device_id": "sensor-001", ... }
#   ],
#   "meta": { "count": 10, "limit": 10, "device_id": "sensor-001" }
# }

# 4. Test missing device_id (should return 400)
curl "http://localhost:8080/api/v1/sensor-readings?limit=10"

# Expected output:
# {
#   "error": {
#     "code": "INVALID_PARAMETER",
#     "message": "device_id is required",
#     "timestamp": "..."
#   }
# }

# 5. Test unknown device (should return 404)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-999"

# Expected output:
# {
#   "error": {
#     "code": "DEVICE_NOT_FOUND",
#     "message": "No readings found for device_id: sensor-999",
#     "timestamp": "..."
#   }
# }

# 6. Test with reading_type filter
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=5&reading_type=temperature"

# Expected output:
# Only temperature readings returned
```

### Smoke Test Script

```bash
#!/bin/bash
# scripts/smoke-test.sh

API_BASE="${API_BASE:-http://localhost:8080}"
FAIL=0

echo "=== API Smoke Test ==="
echo "Testing API at: $API_BASE"
echo ""

# Test 1: Health check
echo -n "1. Health check... "
HEALTH=$(curl -s "$API_BASE/health")
STATUS=$(echo "$HEALTH" | jq -r '.status')
if [ "$STATUS" = "healthy" ]; then
    echo "✓ PASS"
else
    echo "✗ FAIL (status: $STATUS)"
    FAIL=1
fi

# Test 2: Valid request
echo -n "2. Valid request... "
RESPONSE=$(curl -s "$API_BASE/api/v1/sensor-readings?device_id=sensor-001&limit=10")
COUNT=$(echo "$RESPONSE" | jq -r '.meta.count')
if [ "$COUNT" -gt 0 ]; then
    echo "✓ PASS ($COUNT records)"
else
    echo "✗ FAIL"
    FAIL=1
fi

# Test 3: Missing device_id (should return 400)
echo -n "3. Missing device_id (400 expected)... "
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_BASE/api/v1/sensor-readings?limit=10")
if [ "$CODE" = "400" ]; then
    echo "✓ PASS"
else
    echo "✗ FAIL (got $CODE)"
    FAIL=1
fi

# Test 4: Unknown device (should return 404)
echo -n "4. Unknown device (404 expected)... "
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_BASE/api/v1/sensor-readings?device_id=sensor-999")
if [ "$CODE" = "404" ]; then
    echo "✓ PASS"
else
    echo "✗ FAIL (got $CODE)"
    FAIL=1
fi

echo ""
if [ $FAIL -eq 0 ]; then
    echo "✓ All smoke tests passed!"
    exit 0
else
    echo "✗ Some tests failed"
    exit 1
fi
```

### Verify Index Usage

```bash
# Check if the query is using the covering index
psql "postgres://sensor_user:password@localhost:5432/sensor_db" << 'EOF'
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;
EOF

# Look for:
# - Index Only Scan using idx_sensor_readings_device_covering
# - Execution time < 1ms
```

---

## Troubleshooting

### Server Won't Start

**Problem:** `bind: address already in use`

**Solution:**
```bash
# Check what's using port 8080
sudo lsof -i :8080

# Kill the process
kill -9 <PID>

# Or change PORT in .env
PORT=8081 go run cmd/api/main.go
```

### Database Connection Errors

**Problem:** `connection refused` or `authentication failed`

**Solutions:**
```bash
# 1. Verify PostgreSQL is running
docker ps | grep postgres
sudo systemctl status postgresql  # if native

# 2. Verify connection string
echo $DATABASE_URL

# 3. Test connection manually
psql "$DATABASE_URL" -c "SELECT 1"

# 4. Check database and user exist
psql "$DATABASE_URL" -c "\l"
psql "$DATABASE_URL" -c "\du"

# 5. Verify .env is being loaded
# Use godotenv package to load .env file
```

### Slow Query Performance

**Problem:** Queries taking >500ms

**Investigation:**
```sql
-- 1. Check if index is being used
EXPLAIN ANALYZE
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;

-- 2. Check table statistics
SELECT
    relname,
    n_live_tup,
    n_dead_tup,
    last_autovacuum
FROM pg_stat_user_tables
WHERE relname = 'sensor_readings';

-- 3. Check index sizes
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_indexes
WHERE tablename = 'sensor_readings';

-- 4. Run manual ANALYZE
ANALYZE sensor_readings;
```

**Solutions:**
- Ensure covering index exists: `idx_sensor_readings_device_covering`
- Run `ANALYZE sensor_readings` to update statistics
- Check if cache is working
- Verify connection pool isn't exhausted

### Connection Pool Exhaustion

**Problem:** All connections in use, new requests time out

**Investigation:**
```bash
# Check active connections
psql "$DATABASE_URL" -c "
SELECT
    count(*) as connections,
    state
FROM pg_stat_activity
WHERE datname = 'sensor_db'
GROUP BY state;
"
```

**Solutions:**
- Increase `DB_MAX_CONNECTIONS` in `.env`
- Check for connection leaks (ensure `defer rows.Close()` is called)
- Reduce request timeout
- Add connection pool monitoring to health check

### Cache Not Working

**Problem:** Cache misses on every request

**Investigation:**
```bash
# 1. Check if Redis is running
redis-cli ping

# 2. Check cache keys
redis-cli KEYS "sensor:*"

# 3. Check if cache is enabled in .env
grep CACHE_ENABLED .env

# 4. Add logging to cache operations
# Log cache hits/misses in service layer
```

**Solutions:**
- Verify `CACHE_ENABLED=true` in `.env`
- Check Redis connection string
- Verify cache key pattern matches
- Check if TTL is too short

### Import Errors

**Problem:** `undefined: xxx` or `cannot find package`

**Solutions:**
```bash
# 1. Download dependencies
go mod tidy

# 2. Verify module path matches directory structure
grep "module" go.mod

# 3. Clean build cache
go clean -cache
go build -a ./cmd/api

# 4. Check for case sensitivity (Linux is case-sensitive)
# internal/config vs internal/Config
```

---

## Done Criteria

The API development phase is complete when:

- [ ] API server starts successfully on port 8080
- [ ] GET `/health` returns 200 with database and Redis status
- [ ] GET `/api/v1/sensor-readings?device_id=sensor-001&limit=10` returns 200 with data
- [ ] GET `/api/v1/sensor-readings` without device_id returns 400
- [ ] GET `/api/v1/sensor-readings?device_id=sensor-999` returns 404
- [ ] Connection pool is configured (25 max, 5 min)
- [ ] All code compiles without errors: `go build ./cmd/api`
- [ ] EXPLAIN ANALYZE shows index-only scan
- [ ] Smoke tests pass

---

## Next Steps

After API development is complete:

1. **[cache-setup.md](cache-setup.md)** — Integrate Redis caching layer
2. **[load-testing-setup.md](load-testing-setup.md)** — Execute performance tests
3. **[validation-checklist.md](validation-checklist.md)** — End-to-end verification

---

## Related Documentation

- **[../api-spec.md](../api-spec.md)** — Complete API contract
- **[../architecture.md](../architecture.md)** — System architecture and design decisions
- **[database-setup.md](database-setup.md)** — Database schema and indexes
- **[dev-environment.md](dev-environment.md)** — Environment setup
