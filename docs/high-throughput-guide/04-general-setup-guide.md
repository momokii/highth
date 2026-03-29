# General Setup Guide for Any Use Case

This step-by-step guide shows how to adapt the high-throughput PostgreSQL + Golang pattern to your specific use case.

## Overview

This guide is organized into 7 steps:

1. **Identify your query pattern**
2. **Design your schema**
3. **Create your indexes**
4. **Implement caching strategy**
5. **Configure connection pools**
6. **Add monitoring**
7. **Validate performance**

## Step 1: Identify Your Query Pattern

### Common Query Patterns

| Pattern | Description | Example |
|---------|-------------|---------|
| **Exact-ID Lookup** | "Get recent N items for entity X" | User activity, transaction history |
| **Time-Range Query** | "Get data between date A and B" | Audit logs, event streams |
| **Aggregation Query** | "Get statistics for entity X" | Dashboard metrics, analytics |

### Decision Tree

```
What is your primary query pattern?

├─ "Get recent N items for entity X"
│  └─ Use composite index + covering index
│     Index: (entity_id, timestamp DESC) INCLUDE (columns...)
│     Cache: 30s TTL
│
├─ "Get data within time range"
│  └─ Use BRIN index on timestamp
│     Index: BRIN(timestamp)
│     Cache: Not recommended (low hit rate)
│
└─ "Get statistics/aggregations"
   └─ Use materialized view
      MV: SELECT entity_id, COUNT(*), AVG(value) GROUP BY entity_id
      Refresh: Every 15 minutes
```

### Define Your Requirements

Answer these questions:

1. **What is the primary entity?** (users, devices, transactions, etc.)
2. **How many entities?** (1K, 100K, 10M+)
3. **What is the data volume?** (rows per day, total rows)
4. **What is the query pattern?** (exact-ID, time-range, aggregation)
5. **What are the latency targets?** (p50, p95, p99)

**Example**: User Activity Logs

```
Entity: users
Volume: 1M users, 10M activities/day
Pattern: "Get recent 100 activities for user X"
Targets: p50 < 200ms, p95 < 500ms
```

## Step 2: Design Your Schema

### Schema Template

```sql
CREATE TABLE entity_readings (
    id              BIGSERIAL       PRIMARY KEY,
    entity_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    type            VARCHAR(20)     NOT NULL,
    value           DECIMAL(10,2)   NOT NULL,
    metadata        JSONB
);
```

### Adapting to Your Use Case

#### Example 1: User Activity Logs

```sql
CREATE TABLE user_activities (
    id              BIGSERIAL       PRIMARY KEY,
    user_id         VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    activity_type   VARCHAR(30)     NOT NULL,
    details         JSONB
);

-- Index
CREATE INDEX idx_user_activities_user_timestamp
ON user_activities (user_id, timestamp DESC);
```

#### Example 2: Transaction History

```sql
CREATE TABLE transactions (
    id              BIGSERIAL       PRIMARY KEY,
    account_id      VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    transaction_type VARCHAR(20)   NOT NULL,
    amount          DECIMAL(15,2)   NOT NULL,
    status          VARCHAR(20)     NOT NULL
);

-- Index
CREATE INDEX idx_transactions_account_timestamp
ON transactions (account_id, timestamp DESC);
```

#### Example 3: Event Logs

```sql
CREATE TABLE event_logs (
    id              BIGSERIAL       PRIMARY KEY,
    event_source    VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    event_type      VARCHAR(30)     NOT NULL,
    payload         JSONB
);

-- BRIN index for time-series
CREATE INDEX idx_event_logs_timestamp_brin
ON event_logs USING BRIN (timestamp);
```

### Data Type Selection Guide

| Use Case | Recommended Type | Why |
|----------|------------------|-----|
| **Primary Key** | BIGSERIAL | Sequential, 8 bytes, fast |
| **Entity ID** | VARCHAR(50) | Flexible, indexed efficiently |
| **Timestamp** | TIMESTAMPTZ | Timezone-aware, index-friendly |
| **Type/Category** | VARCHAR(20-30) | Short, indexed |
| **Amount/Value** | DECIMAL(15,2) | Precise, avoids float errors |
| **Metadata** | JSONB | Flexible, queryable |
| **Status/Flag** | VARCHAR(20) + CHECK | Data integrity |

### When to Denormalize

**Normalize when**:
- Data is updated frequently
- Storage is expensive
- Data integrity is critical

**Denormalize when**:
- Read-heavy workload (this use case)
- Query performance is priority
- Data is append-only (no updates)

**Recommendation**: Denormalize for this pattern.

## Step 3: Create Your Indexes

### Index Selection Guide

```
Is data append-only time-series?
├─ Yes → Use BRIN index
│  CREATE INDEX idx_timestamp_brin ON table USING BRIN (timestamp);
│
└─ No → Continue
   │
   ├─ Does query filter by entity_id first?
   │  ├─ Yes → Create composite index
   │  │  CREATE INDEX idx_entity_timestamp ON table (entity_id, timestamp DESC);
   │  │
   │  └─ Can query be satisfied from index alone?
   │     ├─ Yes → Add INCLUDE clause
   │     │  CREATE INDEX idx_entity_covering ON table (entity_id, timestamp DESC)
   │     │  INCLUDE (type, value, metadata);
   │     │
   │     └─ No → Use regular composite index
   │
   └─ Does query use type filter as well?
      └─ Add type to composite index
         CREATE INDEX idx_entity_type_timestamp ON table (entity_id, type, timestamp DESC);
```

### Creating Indexes

```sql
-- 1. Basic composite index
CREATE INDEX idx_readings_entity_timestamp
ON entity_readings (entity_id, timestamp DESC);

-- 2. Composite index with type filter
CREATE INDEX idx_readings_entity_type_timestamp
ON entity_readings (entity_id, type, timestamp DESC);

-- 3. Covering index (index-only scan)
CREATE INDEX idx_readings_entity_covering
ON entity_readings (entity_id, timestamp DESC)
INCLUDE (type, value, metadata);

-- 4. BRIN index for time-series
CREATE INDEX idx_readings_timestamp_brin
ON entity_readings USING BRIN (timestamp);

-- 5. Index on materialized view
CREATE MATERIALIZED VIEW mv_entity_stats AS
SELECT entity_id, COUNT(*), AVG(value)
FROM entity_readings
GROUP BY entity_id;

CREATE INDEX idx_mv_stats_entity_id
ON mv_entity_stats (entity_id);
```

### Verify Index Usage

```sql
-- Check query plan
EXPLAIN ANALYZE
SELECT * FROM entity_readings
WHERE entity_id = 'user-123'
ORDER BY timestamp DESC
LIMIT 100;

-- Look for:
-- - Index Scan or Index Only Scan (good)
-- - Seq Scan (bad - index not used)
```

## Step 4: Implement Caching Strategy

### Cache-Aside Pattern

```go
// 1. Check cache first
key := fmt.Sprintf("entity:%s:readings:%d", entityID, limit)
if err := cache.Get(ctx, key, &result); err == nil {
    return result, nil  // Cache hit
}

// 2. Query database
result, err := db.Query(ctx, entityID, limit)
if err != nil {
    return nil, err
}

// 3. Populate cache (fire and forget)
go cache.Set(ctx, key, result)

return result, nil
```

### Redis Configuration

```go
cache := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

// Test connection
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := cache.Ping(ctx).Err(); err != nil {
    log.Printf("Cache unavailable: %v", err)
    // Continue without cache (graceful degradation)
}
```

### Cache Key Strategy

```go
// Format: {entity}:{entity_id}:{query_type}:{params}

// Examples:
// entity:user-123:readings:100
// entity:user-123:readings:100:temperature
// entity:user-123:stats:daily

func cacheKey(entityID string, limit int, filterType string) string {
    if filterType != "" {
        return fmt.Sprintf("entity:%s:readings:%d:%s", entityID, limit, filterType)
    }
    return fmt.Sprintf("entity:%s:readings:%d", entityID, limit)
}
```

### TTL Selection

| Data Type | Recommended TTL | Rationale |
|-----------|-----------------|-----------|
| **Real-time data** | 5-15s | Balance freshness vs performance |
| **Activity logs** | 30-60s | Users rarely refresh instantly |
| **Statistics** | 5-15 minutes | Pre-computed, expensive |
| **Configuration** | 1-5 minutes | Changes infrequently |

### Graceful Degradation

```go
// Service should work even if cache is down
func (s *Service) GetReadings(ctx context.Context, entityID string, limit int) ([]Reading, error) {
    // Try cache
    var cached []Reading
    if s.cache != nil {
        if err := s.cache.Get(ctx, cacheKey(entityID, limit), &cached); err == nil {
            return cached, nil
        }
    }

    // Fall back to database
    readings, err := s.repo.Query(ctx, entityID, limit)
    if err != nil {
        return nil, err
    }

    // Try to populate cache (don't fail if this fails)
    if s.cache != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
            defer cancel()
            _ = s.cache.Set(ctx, cacheKey(entityID, limit), readings)
        }()
    }

    return readings, nil
}
```

## Step 5: Configure Connection Pools

### PostgreSQL Configuration

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=2GB
      -c effective_cache_size=6GB
      -c work_mem=16MB
      -c maintenance_work_mem=1GB
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
      -c wal_buffers=16MB
      -c checkpoint_completion_target=0.9
      -c max_worker_processes=8
      -c max_parallel_workers_per_gather=2
      -c max_parallel_workers=8
      -c bgwriter_delay=200ms
      -c bgwriter_lru_maxpages=100
```

#### PostgreSQL Parameters Explained

| Parameter | Value | Purpose |
|-----------|-------|---------|
| **Memory** |||
| `shared_buffers` | 2GB | PostgreSQL disk cache (25% of RAM on 8GB system) |
| `effective_cache_size` | 6GB | Planner's estimate of total cache (PG + OS file cache) |
| `work_mem` | 16MB | Memory per sort/hash operation (per query node, not total) |
| `maintenance_work_mem` | 1GB | Memory for VACUUM, CREATE INDEX, and other maintenance |
| **Connections** |||
| `max_connections` | 200 | Max concurrent DB connections (for connection pooling) |
| **WAL** |||
| `wal_buffers` | 16MB | Write-Ahead Log memory buffer |
| `checkpoint_completion_target` | 0.9 | Spread checkpoint I/O over 90% of interval (prevents spikes) |
| **Query Planner** |||
| `random_page_cost` | 1.1 | Cost of non-sequential disk access (lower = optimized for SSD) |
| `effective_io_concurrency` | 200 | Parallel I/O operations SSD can handle |
| **Parallelism** |||
| `max_worker_processes` | 8 | Max background workers (matches CPU cores) |
| `max_parallel_workers_per_gather` | 2 | Max parallel workers per single query |
| `max_parallel_workers` | 8 | Max parallel workers across all operations |
| **Background Writer** |||
| `bgwriter_delay` | 200ms | Delay between background writer rounds |
| `bgwriter_lru_maxpages` | 100 | Max buffers flushed per round (800KB per 200ms) |

> **For detailed explanations of each parameter** including trade-offs and what happens if values are too low or too high, see [01-postgresql-setup.md](./01-postgresql-setup.md#parameter-explanations).

### Golang Connection Pool

```go
config, _ := pgxpool.ParseConfig(databaseURL)

// Pool configuration
config.MaxConns = 50           // Maximum open connections
config.MinConns = 10           // Minimum idle connections
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 10 * time.Minute
config.HealthCheckPeriod = 30 * time.Second

pool, _ := pgxpool.NewWithConfig(ctx, config)
```

### Pool Size Calculation

```
Formula: (CPU cores × 2) + effective_spindle_count

Examples:
- 2 cores, HDD: (2 × 2) + 1 = 5
- 4 cores, SSD: (4 × 2) + 1 = 9 → 10
- 8 cores, NVMe: (8 × 2) + 1 = 17 → 20

Production: 50 connections (handles 1000+ RPS)
```

## Step 6: Add Monitoring

### Health Check Endpoint

```go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    results := make(map[string]map[string]interface{})

    // Check database
    dbStart := time.Now()
    dbErr := h.repo.Ping(ctx)
    results["database"] = map[string]interface{}{
        "status": map[bool]string{true: "healthy", false: "unhealthy"}[dbErr == nil],
        "latency_ms": time.Since(dbStart).Milliseconds(),
    }

    // Check cache
    cacheStart := time.Now()
    cacheErr := h.cache.Ping(ctx)
    results["cache"] = map[string]interface{}{
        "status": map[bool]string{true: "healthy", false: "unhealthy"}[cacheErr == nil],
        "latency_ms": time.Since(cacheStart).Milliseconds(),
    }

    // Determine overall status
    allHealthy := dbErr == nil && cacheErr == nil

    w.Header().Set("Content-Type", "application/json")
    statusCode := map[bool]int{true: 200, false: 503}[allHealthy]
    w.WriteHeader(statusCode)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": map[bool]string{true: "healthy", false: "unhealthy"}[allHealthy],
        "components": results,
    })
}
```

### Metrics Endpoint

```go
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
    stats := h.repo.Stats()

    metrics := map[string]interface{}{
        "database": map[string]interface{}{
            "open_connections": stats.OpenConnections,
            "idle_connections": stats.IdleConnections,
            "max_connections":  stats.MaxConnections,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}
```

### Logging

```go
// Add request logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status code
        ww := &responseWriter{ResponseWriter: w, status: 200}
        next.ServeHTTP(ww, r)

        duration := time.Since(start)
        log.Printf("%s %s %d %v",
            r.Method,
            r.URL.Path,
            ww.status,
            duration,
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code
    rw.ResponseWriter.WriteHeader(code)
}
```

## Step 7: Validate Performance

### Create k6 Test

```javascript
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const ENTITIES = ['entity-001', 'entity-002', /* ... */];

export const options = {
  scenarios: {
    exact_id_queries: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 10,
      maxVUs: 100,
    },
  },
  thresholds: {
    'http_req_duration': ['p(50)<300', 'p(95)<500', 'p(99)<800'],
    'http_req_failed': ['rate<0.01'],
  },
};

export default function () {
  const entityID = ENTITIES[Math.floor(Math.random() * ENTITIES.length)];
  const url = `${BASE_URL}/api/v1/readings?entity_id=${entityID}&limit=100`;

  const response = http.get(url);

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'has data': (r) => {
      try {
        return Array.isArray(r.json('data'));
      } catch {
        return false;
      }
    },
  });
}
```

### Run Tests

```bash
# Start services
docker-compose up -d

# Run tests
k6 run tests/performance.js

# With custom RPS
k6 run --env CUSTOM_RPS=200 tests/performance.js
```

### Analyze Results

```
Look for:
✓ p(95) < 500ms - Primary target met
✓ http_req_failed < 1% - Error rate acceptable
✓ response time is consistent - No spikes
```

### Iteration Cycle

```
1. Run baseline test (document results)
2. Make optimization (e.g., add index)
3. Run test again (compare results)
4. If improved: keep, document
5. If not improved: revert, try different approach
6. Repeat until targets met
```

## Use Case Examples

### Example 1: E-Commerce Order History

**Query**: "Get recent 50 orders for customer"

```sql
-- Schema
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    customer_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL,
    total DECIMAL(15,2) NOT NULL
);

-- Index
CREATE INDEX idx_orders_customer_timestamp
ON orders (customer_id, timestamp DESC)
INCLUDE (status, total);
```

### Example 2: Social Media Feed

**Query**: "Get recent 20 posts for user"

```sql
-- Schema
CREATE TABLE posts (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content TEXT,
    metadata JSONB
);

-- Index
CREATE INDEX idx_posts_user_timestamp
ON posts (user_id, timestamp DESC)
INCLUDE (content, metadata);
```

### Example 3: IoT Sensor Data

**Query**: "Get recent 100 readings for sensor"

```sql
-- Schema
CREATE TABLE sensor_readings (
    id BIGSERIAL PRIMARY KEY,
    sensor_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reading_type VARCHAR(20) NOT NULL,
    value DECIMAL(10,2) NOT NULL
);

-- Indexes
CREATE INDEX idx_sensor_readings_sensor_timestamp
ON sensor_readings (sensor_id, timestamp DESC)
INCLUDE (reading_type, value);

CREATE INDEX idx_sensor_readings_timestamp_brin
ON sensor_readings USING BRIN (timestamp);
```

## Troubleshooting

### Issue: p95 latency > 500ms

**Possible causes**:
- Missing index
- Insufficient cache hit rate
- Connection pool exhaustion
- Database overload

**Solutions**:
1. Check `EXPLAIN ANALYZE` output
2. Monitor cache hit rate (target > 80%)
3. Check connection pool stats
4. Increase `MaxConns` if needed

### Issue: High error rate under load

**Possible causes**:
- Database connection timeout
- Request queue overflow
- Resource exhaustion

**Solutions**:
1. Increase `max_connections` in PostgreSQL
2. Add PgBouncer for connection pooling
3. Implement rate limiting
4. Add horizontal scaling

### Issue: Cache not helping

**Possible causes**:
- Cache keys not consistent
- TTL too short
- Hot key distribution skewed

**Solutions**:
1. Verify cache key format
2. Increase TTL (30s → 60s)
3. Pre-populate cache for known hot keys

## Best Practices Summary

1. **Identify query pattern first** before designing schema
2. **Use BIGSERIAL primary keys** for time-series data
3. **Create appropriate indexes** (BRIN, composite, covering)
4. **Implement cache-aside pattern** with graceful degradation
5. **Configure connection pools** properly (50 max, 10 min)
6. **Add health checks and metrics** for monitoring
7. **Validate with k6** before production deployment
8. **Iterate on optimizations** based on test results

## Next Steps

- [Example Schema](./examples/schema.sql) - Complete schema example
- [Example Configuration](./examples/docker-compose.yml) - Docker setup
- [PostgreSQL Setup](./01-postgresql-setup.md) - Database details
- [Golang API Setup](./02-golang-api-setup.md) - API implementation
