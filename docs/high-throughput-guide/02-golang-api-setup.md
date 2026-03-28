# Golang API Setup for High-Throughput Systems

This guide covers building a high-performance Golang API layer that achieves ≤500ms median latency when integrated with PostgreSQL.

## Overview

The Golang API layer is critical for overall system performance. This document covers:

1. **Why Golang** for high-throughput systems
2. **Database connection pool configuration** with pgx
3. **Request handling patterns** (Handler → Service → Repository)
4. **Caching implementation** using Redis
5. **Latency optimization techniques**

## Why Golang for High-Throughput

### Performance Advantages

| Feature | Benefit | Impact on Latency |
|---------|---------|-------------------|
| **Goroutines** | 10K+ concurrent connections per instance | Handle load without thread overhead |
| **Fast compilation** | Quick deployment and iteration | Faster development cycle |
| **pgx driver** | Optimized PostgreSQL driver | 2-3x faster than database/sql |
| **Static typing** | Compile-time error checking | Fewer runtime errors |
| **Efficient GC** | Low garbage collection overhead | Consistent latency |

### Concurrency Model

```go
// Goroutines are lightweight (~2KB each)
go handleRequest(request)

// vs Threads (~2MB each)
// Can handle 5000x more goroutines than threads
```

**Result**: Single API instance can handle 10K+ concurrent connections.

## Database Connection Pool Configuration

### Use pgx/v5 (Not database/sql)

```go
// Recommended: pgx with connection pool
import "github.com/jackc/pgx/v5/pgxpool"

// Avoid: database/sql (slower, less feature-rich)
import "database/sql"
```

**Why pgx**:
- 2-3x faster row scanning
- Built-in connection pooling
- Better PostgreSQL feature support
- Lower memory allocation

### Connection Pool Configuration

```go
package repository

import (
    "context"
    "time"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
    DatabaseURL          string
    MaxOpenConns         int32
    MinOpenConns         int32
    MaxConnLifetime      time.Duration
    MaxConnIdleTime      time.Duration
    HealthCheckPeriod    time.Duration
}

func New(cfg Config) (*pgxpool.Pool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
    if err != nil {
        return nil, err
    }

    // Connection pool settings
    config.MaxConns = cfg.MaxOpenConns           // 50
    config.MinConns = cfg.MinOpenConns           // 10
    config.MaxConnLifetime = cfg.MaxConnLifetime  // 1h
    config.MaxConnIdleTime = cfg.MaxConnIdleTime  // 10m
    config.HealthCheckPeriod = cfg.HealthCheckPeriod  // 30s

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }

    // Verify connection
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, err
    }

    return pool, nil
}
```

### Configuration Parameters

| Parameter | Value | Explanation |
|-----------|-------|-------------|
| **MaxConns** | 50 | Maximum open connections |
| **MinConns** | 10 | Minimum idle connections maintained |
| **MaxConnLifetime** | 1h | Maximum time a connection can be reused |
| **MaxConnIdleTime** | 10m | Close idle connections after this duration |
| **HealthCheckPeriod** | 30s | Frequency of connection health checks |

### Pool Size Calculation

```
MaxConns = (CPU cores × 2) + effective_spindle_count

Example (4 cores, SSD):
MaxConns = (4 × 2) + 1 = 9 → round up to 10

For high-throughput (production):
MaxConns = 50 (handles 1000+ RPS)
```

**Why not more connections?**
- Each connection consumes memory (~10MB for PostgreSQL backend)
- Too many connections cause contention
- Connection pool overhead increases with size

### Environment Configuration

```go
// From docker-compose.yml or .env file
DATABASE_URL=postgres://user:pass@localhost:5432/dbname
DB_MAX_CONNECTIONS=50
DB_MIN_CONNECTIONS=10
DB_MAX_CONN_LIFETIME=1h
DB_MAX_CONN_IDLE_TIME=10m
DB_HEALTH_CHECK_PERIOD=30s
```

## Request Handling Architecture

### Layer Separation

```
┌─────────────────────────────────────────────────────┐
│                   HTTP Handler                       │
│  (Request parsing, response writing, HTTP status)   │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                   Service Layer                     │
│  (Business logic, validation, cache coordination)   │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                 Repository Layer                    │
│  (Database queries, connection management)          │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                 Database (PostgreSQL)                │
└─────────────────────────────────────────────────────┘
```

### Handler Implementation

```go
package handler

import (
    "encoding/json"
    "net/http"
    "github.com/kelanach/higth/internal/service"
)

type SensorHandler struct {
    service *service.SensorService
}

func NewSensorHandler(service *service.SensorService) *SensorHandler {
    return &SensorHandler{service: service}
}

func (h *SensorHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    deviceID := r.URL.Query().Get("device_id")
    limit := parseIntOrDefault(r.URL.Query().Get("limit"), 100)
    readingType := r.URL.Query().Get("type")

    // Call service layer
    readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Write response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data": readings,
        "meta": map[string]interface{}{
            "count":    len(readings),
            "device_id": deviceID,
            "limit":    limit,
        },
    })
}
```

### Service Layer Implementation

```go
package service

import (
    "context"
    "fmt"
    "regexp"
    "github.com/kelanach/higth/internal/cache"
    "github.com/kelanach/higth/internal/repository"
)

type SensorService struct {
    repo  *repository.SensorRepository
    cache *cache.RedisCache
}

func New(repo *repository.SensorRepository, cache *cache.RedisCache) *SensorService {
    return &SensorService{
        repo:  repo,
        cache: cache,
    }
}

func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]SensorReading, error) {
    // Validate input
    if !isValidDeviceID(deviceID) {
        return nil, fmt.Errorf("invalid device_id")
    }

    if limit < 1 || limit > 500 {
        return nil, fmt.Errorf("limit must be between 1 and 500")
    }

    // Check cache first
    if s.cache != nil && s.cache.IsEnabled() {
        key := cacheKey(deviceID, limit, readingType)
        var cached []SensorReading
        if err := s.cache.Get(ctx, key, &cached); err == nil {
            return cached, nil
        }
    }

    // Cache miss - query database
    readings, err := s.repo.Query(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, err
    }

    // Populate cache (fire and forget)
    if s.cache != nil && s.cache.IsEnabled() {
        key := cacheKey(deviceID, limit, readingType)
        _ = s.cache.Set(ctx, key, readings)
    }

    return readings, nil
}

func isValidDeviceID(deviceID string) bool {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, deviceID)
    return len(deviceID) > 0 && len(deviceID) <= 50 && matched
}

func cacheKey(deviceID string, limit int, readingType string) string {
    if readingType != "" {
        return fmt.Sprintf("entity:%s:readings:%d:%s", deviceID, limit, readingType)
    }
    return fmt.Sprintf("entity:%s:readings:%d", deviceID, limit)
}
```

### Repository Implementation

```go
package repository

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
)

type SensorRepository struct {
    db *pgxpool.Pool
}

func (r *SensorRepository) Query(ctx context.Context, deviceID string, limit int, readingType string) ([]SensorReading, error) {
    query := `
        SELECT id, entity_id, timestamp, reading_type, value, unit
        FROM entity_readings
        WHERE entity_id = $1
    `
    args := []interface{}{deviceID}
    argIdx := 2

    if readingType != "" {
        query += fmt.Sprintf(" AND reading_type = $%d", argIdx)
        args = append(args, readingType)
        argIdx++
    }

    query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", argIdx)
    args = append(args, limit)

    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()

    var readings []SensorReading
    for rows.Next() {
        var r SensorReading
        var id int64
        if err := rows.Scan(&id, &r.EntityID, &r.Timestamp, &r.ReadingType, &r.Value, &r.Unit); err != nil {
            return nil, fmt.Errorf("scan failed: %w", err)
        }
        r.ID = fmt.Sprintf("%d", id)
        readzings = append(readings, r)
    }

    return readings, nil
}
```

## Caching Implementation

### Cache-Aside Pattern

```go
// 1. Check cache first
if cached := cache.Get(key); cached != nil {
    return cached
}

// 2. Query database
data := database.Query(key)

// 3. Populate cache
cache.Set(key, data)
```

### Redis Cache Configuration

```go
package cache

import (
    "context"
    "encoding/json"
    "time"
    "github.com/redis/go-redis/v9"
)

type RedisCache struct {
    client    *redis.Client
    enabled   bool
    defaultTTL time.Duration
}

func New(redisURL string, ttl time.Duration) *RedisCache {
    client := redis.NewClient(&redis.Options{
        Addr:     extractAddr(redisURL),
        Password: extractPassword(redisURL),
        DB:       0,
    })

    return &RedisCache{
        client:    client,
        enabled:   true,
        defaultTTL: ttl,
    }
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
    val, err := c.client.Get(ctx, key).Result()
    if err != nil {
        return err
    }
    return json.Unmarshal([]byte(val), dest)
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return c.client.Set(ctx, key, data, c.defaultTTL).Err()
}

func (c *RedisCache) IsEnabled() bool {
    return c.enabled
}

func (c *RedisCache) Ping(ctx context.Context) error {
    return c.client.Ping(ctx).Err()
}
```

### Cache Key Strategy

```go
// Format: entity:{entity_id}:readings:{limit}[:{type}]
// Example: entity:user-123:readings:100:temperature

func cacheKey(entityID string, limit int, readingType string) string {
    if readingType != "" {
        return fmt.Sprintf("entity:%s:readings:%d:%s", entityID, limit, readingType)
    }
    return fmt.Sprintf("entity:%s:readings:%d", entityID, limit)
}
```

### Cache Configuration

| Parameter | Value | Explanation |
|-----------|-------|-------------|
| **TTL** | 30 seconds | Balance freshness vs performance |
| **Eviction** | allkeys-lru | Remove least recently used keys |
| **Max Memory** | 512MB | Prevent memory exhaustion |
| **Compression** | Yes (gzip) | Reduce memory usage |

### Docker Compose Configuration

```yaml
redis:
  image: redis:7-alpine
  command: redis-server --appendonly yes --maxmemory 512mb --maxmemory-policy allkeys-lru
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
```

## Latency Optimization Techniques

### 1. Use pgx's Fast Row Scanning

```go
// Fast: pgx rows.Scan
for rows.Next() {
    rows.Scan(&id, &entityID, &timestamp, &value)
}

// Slower: sql.Rows with reflection
rows.StructScan(&entity)
```

### 2. Avoid N+1 Queries

```go
// BAD: N+1 query pattern
for _, entityID := range entityIDs {
    data := queryEntity(entityID)  // N queries
}

// GOOD: Single query with IN clause
data := queryEntities(entityIDs)  // 1 query
```

### 3. Use Context Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result := db.QueryContext(ctx, query)
```

### 4. Connection Pool Monitoring

```go
// Add metrics endpoint
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
    stats := h.repo.Stats()
    json.NewEncoder(w)._encode(map[string]interface{}{
        "open_connections": stats.OpenConnections,
        "idle_connections": stats.IdleConnections,
        "max_connections":  stats.MaxConnections,
    })
}
```

### 5. Response Compression

```go
import "github.com/gorilla/handlers"

// Wrap handler with gzip compression
compressedHandler := handlers.CompressHandler(http.HandlerFunc(yourHandler))
```

## Graceful Shutdown

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    // Start server in goroutine
    go func() {
        log.Println("Server starting on :8080")
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down server...")

    // Graceful shutdown with 30s timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server shutdown complete")
}
```

## Health Check Implementation

```go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    results := h.service.PingWithLatency(r.Context())

    allHealthy := true
    components := make(map[string]interface{})

    for name, result := range results {
        status := "healthy"
        if result.Error != nil {
            status = "unhealthy"
            allHealthy = false
        }
        components[name] = map[string]interface{}{
            "status": status,
            "latency_ms": result.LatencyMs,
        }
    }

    statusCode := http.StatusOK
    if !allHealthy {
        statusCode = http.StatusServiceUnavailable
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": map[bool]string{true: "healthy", false: "unhealthy"}[allHealthy],
        "components": components,
    })
}
```

## Performance Considerations

### Latency Breakdown

| Component | Typical Latency | Optimization |
|-----------|-----------------|--------------|
| **HTTP Handler** | 1-5ms | Minimal overhead |
| **Service Layer** | 0-1ms | Validation logic |
| **Cache Hit (Redis)** | 1-5ms | Network round-trip |
| **Cache Miss → DB** | 50-200ms | Database query |
| **Total (cache hit)** | 5-20ms | Sub-30ms target |
| **Total (cache miss)** | 50-200ms | Sub-500ms target |

### Throughput Scaling

```
Single instance (50 connections): ~500-1000 RPS
Horizontal scaling (N instances): ~500-1000 × N RPS
```

## Best Practices Summary

1. **Use pgx/v5** instead of database/sql for PostgreSQL
2. **Configure connection pool** properly (MaxConns=50, MinConns=10)
3. **Implement cache-aside pattern** for hot data (30s TTL)
4. **Separate layers** (Handler → Service → Repository)
5. **Add context timeouts** to prevent hanging requests
6. **Monitor connection pool stats** via metrics endpoint
7. **Implement graceful shutdown** with 30s timeout
8. **Use gzip compression** for API responses
9. **Validate input** at service layer before database queries
10. **Handle cache failures** gracefully (degrade to database)

## Next Steps

- [Performance Targets](./03-performance-targets.md) - Define and measure performance goals
- [General Setup Guide](./04-general-setup-guide.md) - Complete implementation guide
- [PostgreSQL Setup](./01-postgresql-setup.md) - Database configuration details
