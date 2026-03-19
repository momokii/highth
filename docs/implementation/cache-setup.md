# Cache Setup Guide

This guide covers integrating Redis caching with write-through pattern, 30-second TTL, and graceful degradation.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Redis Installation](#redis-installation)
- [Redis Configuration](#redis-configuration)
- [Cache Key Pattern](#cache-key-pattern)
- [Cache Implementation](#cache-implementation)
- [Cache Integration Flow](#cache-integration-flow)
- [TTL Configuration](#ttl-configuration)
- [Cache Hit Rate Monitoring](#cache-hit-rate-monitoring)
- [Graceful Degradation](#graceful-degradation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting cache setup, ensure:

- [ ] Phase 3 (API Development) complete
- [ ] API server runs on port 8080
- [ ] Database queries working without cache
- [ ] Phase 0 (Environment Setup) complete
- [ ] Redis client tools installed (`redis-cli`)
- [ ] At least 512MB free RAM for Redis

---

## Redis Installation

### Option 1: Docker (Recommended)

**Pros:** Isolated environment, easy reset, consistent configuration

**Cons:** Additional resource overhead

```bash
# Pull Redis 7 image
docker pull redis:7-alpine

# Start Redis container
docker run -d \
  --name redis-cache \
  -p 6379:6379 \
  -v redis_data:/data \
  redis:7-alpine \
  redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru --save ""

# Wait for startup
sleep 2

# Verify connection
docker exec -it redis-cache redis-cli ping
# Expected output: PONG

# Check configuration
docker exec -it redis-cache redis-cli CONFIG GET maxmemory
# Expected output: maxmemory: 536870912 (512MB)
```

### Option 2: Native Installation

**Pros:** No container overhead, direct access to system resources

**Cons:** System-wide installation, harder to reset

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y redis-server

# macOS
brew install redis
brew services start redis

# Start Redis service
sudo systemctl start redis    # Linux
brew services start redis     # macOS

# Verify connection
redis-cli ping
# Expected output: PONG
```

### Docker Compose Integration

Add to `docker-compose.yml`:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: postgres-sensor
    environment:
      POSTGRES_DB: sensor_db
      POSTGRES_USER: sensor_user
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    container_name: redis-cache
    command: >
      redis-server
      --maxmemory 512mb
      --maxmemory-policy allkeys-lru
      --save ""
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
  redis_data:
```

---

## Redis Configuration

### Cache-Optimal Configuration

```bash
# redis.conf or command-line arguments

# Memory limit (adjust based on available RAM)
maxmemory 512mb

# Eviction policy: LRU for all keys
maxmemory-policy allkeys-lru

# Disable persistence (cache-only)
save ""
# Alternatively: save ""  # Empty string disables RDB snapshots

# Disable AOF (optional, for pure cache)
appendonly no

# Max clients (default: 10000)
maxclients 10000

# Timeout for idle connections (default: 300s)
timeout 300
```

### Configuration Parameters Explained

| Parameter | Value | Purpose |
|-----------|-------|---------|
| `maxmemory` | 512MB | Limits Redis memory usage; prevents OOM |
| `maxmemory-policy` | allkeys-lru | Evicts least-recently-used keys when memory full |
| `save` | "" (empty) | Disables RDB persistence; data loss is acceptable for cache |
| `appendonly` | no | Disables AOF; improves performance |
| `maxclients` | 10000 | Maximum concurrent connections |
| `timeout` | 300 | Closes idle client connections after 5 minutes |

### Why These Settings?

**Memory-limited with LRU eviction:**
- Cache is a performance optimization, not data source
- Losing cache keys is acceptable (they can be refetched from DB)
- LRU eviction keeps frequently-used keys in memory

**Persistence disabled:**
- Faster performance (no disk I/O for snapshots)
- Simpler operation (no AOF rewrite overhead)
- Cache data can be recreated from database

**Trade-off:** If Redis restarts, cache is cold until repopulated

---

## Cache Key Pattern

### Key Format

```
Format: sensor:{device_id}:readings:{limit}[:{reading_type}]

Components:
- sensor:        Static prefix (identifies domain)
- {device_id}:   Variable device identifier
- readings:      Static prefix (identifies resource type)
- {limit}:       Query limit parameter (different limits = different keys)
- {reading_type}: Optional reading type filter
```

### Example Keys

| Query | Cache Key |
|-------|-----------|
| `device_id=sensor-001&limit=10` | `sensor:sensor-001:readings:10` |
| `device_id=sensor-001&limit=50` | `sensor:sensor-001:readings:50` |
| `device_id=sensor-002&limit=10` | `sensor:sensor-002:readings:10` |
| `device_id=sensor-001&limit=10&reading_type=temperature` | `sensor:sensor-001:readings:10:temperature` |

### Key Pattern Rationale

**Include `limit` in key:**
- Different limits return different result sets
- Prevents over-fetching (e.g., requesting 10 when cached 100)

**Include `reading_type` in key (if present):**
- Filters change the result set
- Separate cache entry for each reading type

**Colon (`:`) separator:**
- Redis convention for hierarchical keys
- Enables key pattern matching with `KEYS sensor:*`
- Easy to read in `redis-cli`

**Key length considerations:**
- Keys are stored in memory (longer keys = more memory)
- Current pattern averages ~40-50 bytes per key
- Acceptable trade-off for readability

---

## Cache Implementation

### Redis Cache Wrapper (Go)

```go
// internal/cache/redis_cache.go

package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/yourusername/highth/internal/model"
)

type RedisCache struct {
    client    *redis.Client
    enabled   bool
    hitCount  int64
    missCount int64
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(redisURL string, enabled bool) (*RedisCache, error) {
    if !enabled {
        log.Println("Cache disabled by configuration")
        return &RedisCache{
            enabled: false,
        }, nil
    }

    // Parse Redis URL
    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, fmt.Errorf("unable to parse redis URL: %w", err)
    }

    // Configure client
    client := redis.NewClient(opt)

    // Test connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("unable to connect to redis: %w", err)
    }

    log.Printf("Redis cache connected: %s", redisURL)

    return &RedisCache{
        client:  client,
        enabled: true,
    }, nil
}

// Get retrieves sensor readings from cache
// Returns (readings, found, error)
func (c *RedisCache) Get(ctx context.Context, key string) ([]model.SensorReading, bool) {
    if !c.enabled || c.client == nil {
        return nil, false
    }

    // Get from Redis
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        // Key not found (cache miss)
        c.missCount++
        return nil, false
    } else if err != nil {
        // Redis error - log but don't fail
        log.Printf("ERROR: Redis GET failed: %v", err)
        c.missCount++
        return nil, false
    }

    // Unmarshal JSON
    var readings []model.SensorReading
    if err := json.Unmarshal([]byte(val), &readings); err != nil {
        log.Printf("ERROR: Failed to unmarshal cached data: %v", err)
        c.missCount++
        return nil, false
    }

    // Cache hit
    c.hitCount++
    log.Printf("CACHE HIT: %s (%d records)", key, len(readings))
    return readings, true
}

// Set stores sensor readings in cache
func (c *RedisCache) Set(ctx context.Context, key string, readings []model.SensorReading, ttl time.Duration) error {
    if !c.enabled || c.client == nil {
        return nil
    }

    // Marshal to JSON
    data, err := json.Marshal(readings)
    if err != nil {
        return fmt.Errorf("failed to marshal readings: %w", err)
    }

    // Set with TTL
    if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
        log.Printf("ERROR: Redis SET failed: %v", err)
        return err
    }

    log.Printf("CACHE SET: %s (TTL: %v, %d records)", key, ttl, len(readings))
    return nil
}

// Delete removes a key from cache
// Use for cache invalidation if needed
func (c *RedisCache) Delete(ctx context.Context, key string) error {
    if !c.enabled || c.client == nil {
        return nil
    }

    if err := c.client.Del(ctx, key).Err(); err != nil {
        log.Printf("ERROR: Redis DEL failed: %v", err)
        return err
    }

    log.Printf("CACHE DELETE: %s", key)
    return nil
}

// FlushAll clears all cache entries
// Use with caution - wipes entire cache
func (c *RedisCache) FlushAll(ctx context.Context) error {
    if !c.enabled || c.client == nil {
        return nil
    }

    if err := c.client.FlushAll(ctx).Err(); err != nil {
        log.Printf("ERROR: Redis FLUSHALL failed: %v", err)
        return err
    }

    log.Println("CACHE FLUSHALL: All cache entries cleared")
    return nil
}

// Stats returns cache hit rate statistics
func (c *RedisCache) Stats() CacheStats {
    total := c.hitCount + c.missCount
    hitRate := 0.0
    if total > 0 {
        hitRate = float64(c.hitCount) / float64(total) * 100
    }

    return CacheStats{
        HitCount:  c.hitCount,
        MissCount: c.missCount,
        HitRate:   hitRate,
    }
}

type CacheStats struct {
    HitCount  int64   `json:"hit_count"`
    MissCount int64   `json:"miss_count"`
    HitRate   float64 `json:"hit_rate"`
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
    if c.client != nil {
        return c.client.Close()
    }
    return nil
}
```

---

## Cache Integration Flow

### Write-Through Pattern

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API Request Received                              │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
                         ┌─────────────────────────────┐
                         │  Validate Request           │
                         │  - device_id required       │
                         │  - limit between 1-500      │
                         └─────────────┬───────────────┘
                                       │
                                       ▼
                         ┌─────────────────────────────┐
                         │  Check Redis Cache          │
                         │  Key: sensor:{id}:readings  │
                         └─────────────┬───────────────┘
                                       │
                         ┌─────────────┴───────────────┐
                         │ Cache Hit?                  │
                         └─────────────┬───────────────┘
                                       │
                    ┌──────────────────┴──────────────────┐
                    │ YES                                 │ NO
                    ▼                                     ▼
         ┌──────────────────────┐           ┌──────────────────────────────┐
         │ Return Cached Data   │           │ Query PostgreSQL             │
         │ (5-15ms)              │           │ (200-400ms)                  │
         │ CACHE HIT logged      │           │ Using covering index         │
         └──────────────────────┘           └──────────────┬───────────────┘
                                                     │
                                                     ▼
                                        ┌──────────────────────────────┐
                                        │ Populate Redis Cache         │
                                        │ SET key value EX 30          │
                                        │ Async, non-blocking          │
                                        │ (fire and forget)            │
                                        └──────────────┬───────────────┘
                                                       │
                                                       ▼
                                        ┌──────────────────────────────┐
                                        │ Return Data to Client        │
                                        │ CACHE MISS logged            │
                                        └──────────────────────────────┘
```

### Service Layer Integration

```go
// internal/service/sensor_service.go (updated with cache)

func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
    // 1. Build cache key
    cacheKey := s.buildCacheKey(deviceID, limit, readingType)

    // 2. Check cache if enabled
    if s.enabled && s.cache != nil {
        if cached, found := s.cache.Get(ctx, cacheKey); found {
            // Cache hit - return immediately
            return cached, nil
        }
        // Cache miss - continue to database
    }

    // 3. Query database (cache miss or cache disabled)
    readings, err := s.repo.GetSensorReadings(ctx, deviceID, limit, readingType)
    if err != nil {
        return nil, err
    }

    // 4. Check if any results returned
    if len(readings) == 0 {
        return nil, ErrDeviceNotFound
    }

    // 5. Populate cache asynchronously (non-blocking)
    if s.enabled && s.cache != nil {
        go func() {
            // Use background context with timeout
            cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer cancel()

            // Ignore cache write errors (fire and forget)
            _ = s.cache.Set(cacheCtx, cacheKey, readings, 30*time.Second)
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

### Async Cache Write Pattern

**Why write cache asynchronously?**

1. **Faster response time** — Client doesn't wait for cache write
2. **Cache is optimization** — Failed cache write doesn't break functionality
3. **Non-blocking** — Service can handle more concurrent requests

**Trade-offs:**
- Race condition: Multiple concurrent misses might write to same key
- Solution: Redis SET overwrites existing key; last write wins (acceptable)

---

## TTL Configuration

### TTL Strategy

**30-second TTL balances:**
- **Freshness** — Data is at most 30 seconds old
- **Performance** — High cache hit rate for repeated queries
- **Memory** — Keys expire automatically, preventing memory bloat

### Why 30 Seconds?

| TTL | Hit Rate | Freshness | Use Case |
|-----|----------|-----------|----------|
| 5s | Low | Very fresh | Real-time critical systems |
| **30s** | **High** | **Fresh** | **IoT monitoring (default)** |
| 60s | Very high | Acceptable | Dashboards, analytics |
| 300s | Max | Stale | Static/reference data |

**For IoT sensor monitoring:**
- Readings are historical data (append-only)
- 30-second delay is acceptable for monitoring dashboards
- Real-time alerts use separate system (not cached queries)

### TTL Implementation

```go
// Environment variable
CACHE_TTL=30s

// Config loading
func Load() *Config {
    return &Config{
        CacheTTL: getEnvDuration("CACHE_TTL", 30*time.Second),
    }
}

// Service layer uses configured TTL
s.cache.Set(cacheCtx, cacheKey, readings, cfg.CacheTTL)
```

### Verifying TTL

```bash
# Set a key and check its TTL
redis-cli SET sensor:sensor-001:readings:10 "test" EX 30
redis-cli TTL sensor:sensor-001:readings:10

# Expected output:
# (integer) 30  # Seconds remaining

# Wait 5 seconds and check again
sleep 5
redis-cli TTL sensor:sensor-001:readings:10

# Expected output:
# (integer) 25  # Seconds remaining

# After 30 seconds
sleep 30
redis-cli GET sensor:sensor-001:readings:10

# Expected output:
# (nil)  # Key has expired
```

---

## Cache Hit Rate Monitoring

### Stats Tracking

```go
// internal/cache/redis_cache.go (stats already implemented above)

type CacheStats struct {
    HitCount  int64   `json:"hit_count"`
    MissCount int64   `json:"miss_count"`
    HitRate   float64 `json:"hit_rate"`
}

func (c *RedisCache) Stats() CacheStats {
    total := c.hitCount + c.missCount
    hitRate := 0.0
    if total > 0 {
        hitRate = float64(c.hitCount) / float64(total) * 100
    }

    return CacheStats{
        HitCount:  c.hitCount,
        MissCount: c.missCount,
        HitRate:   hitRate,
    }
}
```

### Exposing Stats in Health Check

```go
// internal/handler/health.go (updated)

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ... existing health checks ...

    // Add cache stats
    if h.cache != nil {
        stats := h.cache.Stats()
        checks["cache"] = CheckResult{
            Status: "healthy",
            Message: fmt.Sprintf("Hit rate: %.1f%% (%d hits, %d misses)",
                stats.HitRate, stats.HitCount, stats.MissCount),
        }
    }

    // ... rest of health check ...
}
```

### Expected Hit Rates

| Scenario | Expected Hit Rate | Notes |
|----------|-------------------|-------|
| Cold start | 0% | No data in cache |
| Repeated queries (same device) | 80-95% | Most queries hit cache |
| Random devices (uniform) | 20-40% | Many unique keys |
| Random devices (Zipf) | 60-80% | Hot devices cached well |

**Monitoring thresholds:**
- Hit rate < 50%: Investigate cache configuration
- Hit rate > 80%: Cache working well
- Hit rate 100%: Possible stale data (check TTL)

---

## Graceful Degradation

### Cache Failure Modes

| Failure | Symptom | Response |
|---------|---------|----------|
| Redis unreachable | Connection refused | Log error, serve from DB |
| Redis OOM | `OOM command not allowed` | Log error, serve from DB |
| Redis slow | High latency | Log warning, extend timeout |
| Serialization error | Failed to marshal | Log error, serve from DB |

### Degradation Strategy

```go
// Cache Get: Always return (readings, false) on error
func (c *RedisCache) Get(ctx context.Context, key string) ([]model.SensorReading, bool) {
    if !c.enabled || c.client == nil {
        return nil, false  // Cache disabled or not initialized
    }

    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return nil, false  // Normal cache miss
    } else if err != nil {
        // Redis error - log but don't fail the request
        log.Printf("ERROR: Redis GET failed: %v", err)
        return nil, false  // Treat as cache miss
    }

    // ... unmarshal and return
}

// Cache Set: Always return nil on error (fire and forget)
func (c *RedisCache) Set(ctx context.Context, key string, readings []model.SensorReading, ttl time.Duration) error {
    if !c.enabled || c.client == nil {
        return nil  // Silent failure if cache disabled
    }

    data, err := json.Marshal(readings)
    if err != nil {
        log.Printf("ERROR: Failed to marshal: %v", err)
        return nil  // Don't fail the request
    }

    if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
        log.Printf("ERROR: Redis SET failed: %v", err)
        return nil  // Log but don't return error
    }

    return nil
}
```

### Health Check Integration

```go
// Health check should reflect cache status
checks["cache"] = CheckResult{
    Status:  "healthy",
    Message: "Active",
}

// If Redis is down, mark as degraded but don't fail health check
if redisErr != nil {
    checks["cache"] = CheckResult{
        Status:  "unhealthy",
        Message: redisErr.Error(),
    }
    overallStatus = "degraded"  // Not "unhealthy" - API still works
}
```

### Degradation Levels

| Level | Cache Status | API Status | Action |
|-------|--------------|------------|--------|
| Full operation | Healthy | Healthy | Normal operation |
| Cache degraded | Unreachable | Degraded | Serve from DB, alert |
| Cache disabled | Disabled | Healthy | Configured off, no alert |

---

## Verification

### Step-by-Step Verification

#### 1. Verify Redis Running

```bash
redis-cli ping
# Expected output: PONG

docker ps | grep redis  # If using Docker
# Should show redis-cache container running
```

#### 2. Verify Cache Configuration

```bash
# Check maxmemory setting
redis-cli CONFIG GET maxmemory
# Expected: maxmemory: 536870912 (512MB)

# Check eviction policy
redis-cli CONFIG GET maxmemory-policy
# Expected: maxmemory-policy: allkeys-lru

# Check persistence disabled
redis-cli CONFIG GET save
# Expected: save: (empty string)
```

#### 3. Test Cache Read/Write

```bash
# Set a test key
redis-cli SET test:key "hello" EX 30

# Get the key
redis-cli GET test:key
# Expected: "hello"

# Check TTL
redis-cli TTL test:key
# Expected: 30 (seconds remaining)
```

#### 4. Verify Cache Integration

```bash
# Make API request for a device
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Check if key was created in Redis
redis-cli KEYS "sensor:*"
# Should show: sensor:sensor-001:readings:10

# Get the cached data
redis-cli GET "sensor:sensor-001:readings:10"
# Should show JSON array of sensor readings

# Check TTL
redis-cli TTL "sensor:sensor-001:readings:10"
# Should show ~30 seconds
```

#### 5. Verify Cache Hit

```bash
# Make same request again (should hit cache)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Check API logs for "CACHE HIT"
# Or check health endpoint for cache stats
curl http://localhost:8080/health | jq '.checks.cache'
```

#### 6. Verify Cache Expiration

```bash
# Wait for TTL to expire
sleep 31

# Check if key is gone
redis-cli EXISTS "sensor:sensor-001:readings:10"
# Expected: 0 (key does not exist)

# Make request again (should miss cache and repopulate)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
```

#### 7. Verify Different Keys

```bash
# Request different limit (should create new key)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=50"

redis-cli KEYS "sensor:sensor-001:*"
# Should show both keys:
# sensor:sensor-001:readings:10
# sensor:sensor-001:readings:50
```

#### 8. Verify Graceful Degradation

```bash
# Stop Redis
docker stop redis-cache  # If using Docker
sudo systemctl stop redis  # If native

# Make API request (should still work, using DB)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
# Should return data successfully

# Check health endpoint
curl http://localhost:8080/health | jq '.checks.cache'
# Should show: status: "unhealthy" but overall status: "degraded"

# Start Redis again
docker start redis-cache
sudo systemctl start redis
```

---

## Troubleshooting

### Redis Connection Issues

**Problem:** `connection refused` on port 6379

**Solutions:**
```bash
# Check if Redis is running
docker ps | grep redis              # Docker
sudo systemctl status redis        # Native

# Check if port is in use
sudo lsof -i :6379

# Start Redis
docker start redis-cache            # Docker
sudo systemctl start redis          # Native

# Test connection manually
redis-cli ping
```

**Problem:** `authentication failed`

**Solutions:**
```bash
# If Redis has password set, update .env
REDIS_URL=redis://:password@localhost:6379

# Or disable authentication for local development
redis-cli CONFIG SET requirepass ""
```

### Cache Not Working

**Problem:** Every request is a cache miss

**Investigation:**
```bash
# 1. Check if caching is enabled
grep CACHE_ENABLED .env
# Should be: CACHE_ENABLED=true

# 2. Check if cache is being populated
redis-cli KEYS "sensor:*"
# Should show keys after API requests

# 3. Check cache key format
redis-cli KEYS "*"
# If keys exist but pattern doesn't match, check key building logic

# 4. Check if TTL is too short
redis-cli TTL "sensor:sensor-001:readings:10"
# Should be > 0
```

**Problem:** Cache keys exist but not being used

**Solutions:**
- Verify cache key format matches between Set and Get
- Check for case sensitivity issues
- Verify JSON serialization/deserialization is working
- Add debug logging to cache operations

### Memory Issues

**Problem:** Redis OOM (out of memory)

**Investigation:**
```bash
# Check memory usage
redis-cli INFO memory
# Look at: used_memory_human and used_memory_peak_human

# Check maxmemory setting
redis-cli CONFIG GET maxmemory

# Check number of keys
redis-cli DBSIZE
```

**Solutions:**
```bash
# Increase maxmemory
redis-cli CONFIG SET maxmemory 1gb

# Or reduce TTL (fewer keys accumulate)
# Update .env: CACHE_TTL=15s

# Check for key bloat (too many unique keys)
redis-cli --bigkeys --pattern "sensor:*"
```

### Performance Issues

**Problem:** Cache hits are slow (>10ms)

**Investigation:**
```bash
# Measure Redis latency
redis-cli --latency
# Run for 10 seconds, check average latency

# Check slow log
redis-cli SLOWLOG GET 10
```

**Solutions:**
- Check if Redis is on same machine as API (network latency)
- Verify no other heavy operations on same Redis instance
- Check system resources (CPU, RAM)

### Stale Data Concerns

**Problem:** Data is older than expected

**Investigation:**
```bash
# Check TTL of cached keys
redis-cli KEYS "sensor:*" | xargs -I {} redis-cli TTL {}

# Verify TTL is being set correctly
# Set a key and immediately check TTL
redis-cli SET test:key "test" EX 30
redis-cli TTL test:key
```

**Solutions:**
- Reduce TTL if data freshness is critical
- Implement cache invalidation for writes (if applicable)
- For this use case: 30s TTL is acceptable (historical data)

---

## Done Criteria

The cache setup phase is complete when:

- [ ] Redis 7+ running and accessible (Docker or native)
- [ ] `redis-cli ping` returns `PONG`
- [ ] Cache integration working in API
- [ ] Cache hits return in <10ms
- [ ] Cache misses populate cache correctly
- [ ] 30s TTL functioning (verified with TTL command)
- [ ] Cache hit rate monitorable (via health endpoint or logs)
- [ ] Graceful degradation working (API works when Redis is down)
- [ ] Health endpoint shows cache status
- [ ] All verification steps pass

---

## Next Steps

After cache setup is complete:

1. **[load-testing-setup.md](load-testing-setup.md)** — Execute performance tests
2. **[validation-checklist.md](validation-checklist.md)** — End-to-end verification
3. Run load tests to validate ≤500ms target with caching

---

## Related Documentation

- **[../architecture.md](../architecture.md)** — Caching layer design
- **[api-development.md](api-development.md)** — API layer implementation
- **[database-setup.md](database-setup.md)** — Database and indexing
- **[../testing.md](../testing.md)** — Test scenarios and expected results
