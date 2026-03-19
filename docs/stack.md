# Technology Stack

This document provides a comprehensive breakdown of the technology stack, with detailed justifications for each component choice.

## Table of Contents

- [Stack Overview](#stack-overview)
- [Database Engine](#database-engine)
- [API Framework](#api-framework)
- [Caching Solution](#caching-solution)
- [Connection Pooling](#connection-pooling)
- [Load Testing](#load-testing)
- [Reverse Proxy](#reverse-proxy)
- [Containerization](#containerization)

---

## Stack Overview

| Component | Technology | Version |
|-----------|-----------|---------|
| Database | PostgreSQL | 16+ |
| API Framework | Go + chi router | Go 1.21+, chi v5 |
| Caching | Redis | 7.x |
| Connection Pooling | pgx | v5 |
| Load Testing | Vegeta | latest |
| Reverse Proxy | Nginx | stable |
| Container | Docker + Docker Compose | latest |

---

## Database Engine

### Choice: PostgreSQL 16+

### Role in System

PostgreSQL serves as the primary data store for all sensor readings, handling:
- High-volume inserts (10k+ sensors reporting continuously)
- Time-range queries for data analytics
- Device-specific lookups (primary query pattern)
- JSONB data for flexible metadata storage

### Why PostgreSQL over Alternatives?

| Aspect | PostgreSQL | TimescaleDB | MongoDB | InfluxDB |
|--------|------------|-------------|---------|----------|
| **BRIN indexes** | ✅ Native | ✅ Via extension | ❌ | ❌ |
| **JSONB support** | ✅ Excellent | ✅ Via extension | ✅ Native | ⚠️ Limited |
| **ACID guarantees** | ✅ Full | ✅ Full | ⚠️ Document-level | ⚠️ Limited |
| **Maturity** | ✅ 35+ years | ⚠️ Younger | ✅ Mature | ⚠️ Newer |
| **Hosting options** | ✅ Everywhere | ⚠️ Fewer | ✅ Everywhere | ⚠️ Fewer |
| **Learning curve** | ✅ Moderate | ⚠️ Steeper | ⚠️ Different model | ⚠️ Different model |

### Specific PostgreSQL Features for This Use Case

#### 1. BRIN Indexes

BRIN (Block Range INdex) is critical for time-series data at scale:

```
Index size comparison at 50M rows:
- B-tree index on timestamp: ~2-3 GB
- BRIN index on timestamp: ~20-30 MB

Space savings: ~100x
```

#### 2. Covering Indexes (INCLUDE clause)

PostgreSQL 11+ supports covering indexes that include non-key columns:

```sql
CREATE INDEX idx_device_covering
    ON sensor_readings (device_id, timestamp DESC)
    INCLUDE (reading_type, value, unit);
```

This enables **Index-Only Scans** that eliminate heap access.

#### 3. JSONB for Flexible Metadata

```sql
-- Store varying device-specific data
UPDATE sensor_readings
SET metadata = metadata || '{"battery_level": 87, "firmware": "2.1.0"}'
WHERE device_id = 'sensor-001';

-- Query within JSONB
SELECT * FROM sensor_readings
WHERE metadata->>'battery_level'::int < 20;
```

#### 4. Declarative Partitioning

Built-in support for table partitioning (available without extensions):

```sql
CREATE TABLE sensor_readings (...)
PARTITION BY RANGE (timestamp);
```

### Performance Characteristics at 50M Rows

| Operation | Expected Performance |
|-----------|---------------------|
| Point lookup (device_id) | 50-200ms |
| Time-range query (with BRIN) | 100-400ms |
| Insert (single) | <1ms |
| Insert (batch 1000) | 50-100ms |

### Known Limitations

1. **Connection scaling** — PostgreSQL's process-per-connection model limits concurrent connections
   - **Mitigation:** Connection pooling via pgx

2. **Write amplification** — Indexes slow down inserts
   - **Mitigation:** BRIN indexes minimize this; batch inserts

3. **VACUUM overhead** — Append-only workloads still require autovacuum
   - **Mitigation:** Proper autovacuum tuning for time-series data

### Alternatives Considered

#### TimescaleDB

**Why not chosen:**
- Adds complexity for a portfolio project
- BRIN indexes in vanilla PostgreSQL suffice for our scale
- Hosting options more limited than vanilla PostgreSQL

**When would it be better?**
- At 500M+ rows with aggressive time-range queries
- When needing automatic partitioning and retention policies
- For built-in time-series SQL functions

#### MongoDB

**Why not chosen:**
- Different query model (document vs relational)
- Less mature tooling for performance optimization
- JSONB in PostgreSQL covers the flexibility need

**When would it be better?**
- When data structure varies wildly per document
- When horizontal scaling via sharding is the primary concern

---

## API Framework

### Choice: Go + chi Router

### Role in System

The Go application handles:
- HTTP request routing and handling
- Request validation and error handling
- Business logic coordination
- Database and cache interaction
- Concurrent request processing

### Why Go over Alternatives?

| Aspect | Go | Python (FastAPI) | Node.js (Express) | Java (Spring) |
|--------|----|------------------|-------------------|---------------|
| **Performance** | ✅ Excellent | ⚠️ Good | ⚠️ Good | ⚠️ Good |
| **Concurrency model** | ✅ Goroutines | ⚠️ Async/await | ✅ Event loop | ⚠️ Threads |
| **Memory usage** | ✅ Low | ⚠️ Higher | ⚠️ Moderate | ❌ High |
| **Deployment** | ✅ Single binary | ❌ Runtime deps | ❌ Runtime deps | ❌ JVM |
| **Type safety** | ✅ Compile-time | ⚠️ Runtime checks | ❌ Loose typing | ✅ Compile-time |
| **Startup time** | ✅ Instant | ⚠️ Moderate | ✅ Fast | ❌ Slow |

### Specific Go Benefits for This Use Case

#### 1. Goroutines for Concurrency

Go's goroutines enable handling thousands of concurrent requests efficiently:

```go
func (h *Handler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
    // Each request runs in its own goroutine
    // No callback hell; looks like synchronous code
    readings, err := h.service.GetReadings(ctx, deviceID, limit)
    // ...
}
```

#### 2. Single Binary Deployment

```
$ go build -o sensor-api
$ ./sensor-api

# No runtime to install, no dependency hell
```

**Benefits:**
- Simple Docker image (scratch base possible)
- Fast cold starts
- Easy deployment to any Linux host

#### 3. Excellent Standard Library

Go's standard library includes production-ready:
- HTTP server (`net/http`)
- JSON encoding/decoding
- Context for cancellation
- SQL database interface

#### 4. Static Typing

Compile-time type checking catches bugs before runtime:

```go
// Won't compile if types don't match
func GetReadings(ctx context.Context, deviceID string, limit int) ([]Reading, error)
```

### Why chi Router?

chi is a lightweight, idiomatic HTTP router for Go:

```go
r := chi.NewRouter()
r.Get("/api/v1/sensor-readings", handler.GetSensorReadings)
r.Get("/health", handler.HealthCheck)
```

**Benefits over alternatives:**

| Router | chi | gin | echo | gorilla/mux |
|--------|-----|-----|------|-------------|
| **Middleware** | ✅ Composable | ✅ Built-in | ✅ Built-in | ⚠️ Manual |
| **Performance** | ✅ Excellent | ✅ Excellent | ✅ Excellent | ⚠️ Good |
| **API simplicity** | ✅ Idiomatic | ⚠️ Custom | ⚠️ Custom | ✅ Idiomatic |
| **Context support** | ✅ Native | ✅ Native | ✅ Native | ⚠️ Added |

### Performance Characteristics

| Metric | Expected Value |
|--------|----------------|
| Request overhead | <1ms |
| Memory per request | ~5-10 KB |
| Concurrent requests | 10,000+ (limited by config) |
| JSON marshaling | <100μs for typical response |

### Known Limitations

1. **Error handling** — Go's error handling is verbose
   - **Mitigation:** Use error wrapping and helper functions

2. **Generics** — Go 1.18+ has generics, but ecosystem is still adapting
   - **Impact:** Minimal for this use case

3. **Package management** — `go.mod` is good but newer than npm/pip
   - **Impact:** Minimal; dependency resolution is reliable

### Alternatives Considered

#### Python + FastAPI

**Why not chosen:**
- Runtime dependencies required
- GIL limits true parallelism (though asyncio helps)
- Type hints are optional, not enforced

**When would it be better?**
- For rapid prototyping
- When team has strong Python background
- For data science/ML integration

#### Node.js + Express

**Why not chosen:**
- Loose typing leads to runtime errors
- Callback hell (even with async/await)
- NPM dependency management complexity

**When would it be better?**
- For frontend/fullstack teams
- When JSON is the primary data format
- For serverless deployments (cold start matters)

---

## Caching Solution

### Choice: Redis

### Role in System

Redis provides:
- High-speed cache for frequently accessed device readings
- TTL-based automatic expiration
- LRU eviction policy when memory is full
- Sub-millisecond read latency

### Why Redis over Alternatives?

| Aspect | Redis | Memcached | In-memory Go map |
|--------|-------|-----------|------------------|
| **Persistence** | ✅ Optional RDB/AOF | ❌ None | ❌ None |
| **Data structures** | ✅ Rich (strings, lists, sets, etc.) | ⚠️ Just strings | ⚠️ Go types only |
| **TTL support** | ✅ Per-key | ✅ Per-key | ⚠️ Manual |
| **Eviction policy** | ✅ Configurable | ⚠️ LRU only | ⚠️ Manual |
| **Distributed** | ✅ Cluster mode | ✅ Replication | ❌ Single process |
| **Maturity** | ✅ Very mature | ✅ Mature | N/A |

### Specific Redis Features for This Use Case

#### 1. TTL-Based Expiration

```go
// Cache automatically expires after 30 seconds
err := redis.Set(ctx, "sensor:sensor-001:readings:10", data, 30*time.Second)
```

**Benefits:**
- No manual cache invalidation logic
- Fresh data automatically fetched after TTL
- Simple mental model

#### 2. LRU Eviction

When memory is full, Redis automatically evicts least-recently-used keys:

```
redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
```

**Benefits:**
- No manual cache management
- Hot keys stay in cache, cold keys evicted
- Predictable memory usage

#### 3. String Operations

We use Redis strings to cache JSON-serialized responses:

```go
// SET with expiration
SET sensor:sensor-001:readings:10 '{"data":[...],"meta":{...}}' EX 30

// GET
GET sensor:sensor-001:readings:10
```

### Performance Characteristics

| Operation | Latency | Throughput |
|-----------|---------|------------|
| GET | <1ms | 100,000+ ops/sec |
| SET | <1ms | 80,000+ ops/sec |
| MGET (batch) | <5ms for 10 keys | 50,000+ ops/sec |

### Known Limitations

1. **Single-threaded** — Redis is largely single-threaded
   - **Impact:** One Redis instance maxes out at ~100k ops/sec on a single core
   - **Mitigation:** Redis Cluster for horizontal scaling (not needed at our scale)

2. **Memory-only** — All data must fit in RAM
   - **Impact:** Cache size limited by available memory
   - **Mitigation:** Eviction policy ensures cache stays within bounds

3. **Network latency** — Even local Redis adds ~0.5-2ms
   - **Impact:** Cannot beat in-process cache
   - **Mitigation:** Acceptable tradeoff for distributed caching

### Alternatives Considered

#### Memcached

**Why not chosen:**
- No persistence (even optional)
- Fewer data structures
- Less active development

**When would it be better?**
- For simple string caching only
- When maximum ops/sec is the only concern
- When persistence is never needed

#### In-Memory Go Map

**Why not chosen:**
- Not shareable across multiple processes
- No built-in TTL
- Manual eviction required
- No persistence option

**When would it be better?**
- For single-instance deployments
- When cache can be lost on restart
- For absolute minimum latency

---

## Connection Pooling

### Choice: pgx (stdlib-style)

### Role in System

pgx provides:
- PostgreSQL connection pooling
- Prepared statement caching
- Binary protocol for faster data transfer
- Context-based cancellation

### Why pgx over Alternatives?

| Aspect | pgx | database/sql | pgxpool |
|--------|-----|--------------|---------|
| **Performance** | ✅ Best | ⚠️ Good | ✅ Best |
| **Connection pool** | ✅ Built-in (pgxpool) | ⚠️ External | ✅ Built-in |
| **Binary protocol** | ✅ Native | ❌ Text only | ✅ Native |
| **Prepared statements** | ✅ Automatic | ⚠️ Manual | ✅ Automatic |
| **Compatibility** | ✅ PostgreSQL-specific | ✅ Database-agnostic | ✅ PostgreSQL-specific |

### Specific pgx Features for This Use Case

#### 1. Automatic Prepared Statements

```go
// First execution: prepares and executes
rows, _ := pool.Query(ctx, "SELECT * FROM sensor_readings WHERE device_id = $1 LIMIT $2", deviceID, limit)

// Subsequent executions: uses prepared statement (cached per connection)
rows, _ := pool.Query(ctx, "SELECT * FROM sensor_readings WHERE device_id = $1 LIMIT $2", deviceID, limit)
```

**Benefits:**
- Query planning done once, executed many times
- Reduced parsing overhead
- Protection against SQL injection

#### 2. Binary Protocol

pgx uses PostgreSQL's binary protocol for data transfer:

```
Text protocol: '123.456' -> string -> parse -> float64 (slow)
Binary protocol: bytes -> float64 (fast)
```

**Benefits:**
- Faster data transfer
- No string parsing overhead
- More accurate for numeric types

#### 3. Connection Pool

```go
config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute

pool, _ := pgxpool.ConnectConfig(ctx, config)
```

**Benefits:**
- Reuses connections (no connection overhead per request)
- Bounds database connections (prevents overload)
- Health checks and automatic reconnection

### Performance Characteristics

| Operation | Latency |
|-----------|---------|
| Connection from pool | <1ms |
| Query execution (with index) | 50-200ms |
| Binary data transfer | 10-50% faster than text |

### Known Limitations

1. **PostgreSQL-specific** — Not portable to other databases
   - **Impact:** Changing database requires changing driver
   - **Mitigation:** Acceptable for this project; PostgreSQL is chosen

2. **Connection pool tuning required** — Default settings may not be optimal
   - **Mitigation:** Provide recommended settings in documentation

### Alternatives Considered

#### database/sql (standard library)

**Why not chosen:**
- Text protocol only (slower)
- Manual prepared statement management
- External connection pool required (e.g., pgxpool wrapper)

**When would it be better?**
- When database portability is important
- For simple CRUD apps

---

## Load Testing

### Choice: Vegeta

### Role in System

Vegeta provides:
- HTTP load testing
- Attack-based simulation (realistic patterns)
- Built-in percentile metrics (p50, p95, p99)
- Simple CLI interface

### Why Vegeta over Alternatives?

| Aspect | Vegeta | Locust | JMeter | k6 |
|--------|--------|--------|--------|-----|
| **Language** | Go | Python | Java | JavaScript |
| **Distributed** | ✅ Yes | ✅ Yes | ⚠️ Complex | ✅ Yes |
| **Metrics** | ✅ Percentiles | ✅ Percentiles | ✅ Percentiles | ✅ Percentiles |
| **CLI** | ✅ Simple | ⚠️ Python code | ⚠️ GUI heavy | ⚠️ JS code |
| **Attack model** | ✅ Native | ✅ Custom | ⚠️ Thread-based | ✅ Custom |
| **Output formats** | ✅ Many | ⚠️ Limited | ⚠️ JMX only | ✅ JSON/HTML |

### Specific Vegeta Features for This Use Case

#### 1. Attack-Based Testing

```bash
# Attack the API for 30 seconds at 100 requests per second
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=30s -rate=100 | \
  vegeta report -type=text
```

**Benefits:**
- Realistic load simulation (not just sequential requests)
- Easy to specify attack parameters
- CLI-friendly for CI/CD integration

#### 2. Built-in Percentiles

```
Requests      [total, rate]            3000, 100.10
Duration      [total, attack, wait]    30s, 29.97s, 29.18ms
Latencies     [mean, 50, 95, 99, max]  185ms, 167ms, 289ms, 401ms, 1.2s
Bytes In      [total, mean]            4500000, 1500.00
Bytes Out     [total, mean]            0, 0.00
Success       [ratio]                  100.00%
Status Codes  [code:count]             200:3000
```

**Benefits:**
- Direct visibility into p50, p95, p99 latencies
- Easy to verify against ≤500ms target
- No manual percentile calculation

#### 3. Multiple Output Formats

```bash
# Text report
vegeta report -type=text results.bin > report.txt

# JSON for programmatic analysis
vegeta report -type=json results.bin > metrics.json

# Histogram for latency distribution
vegeta report -type=histogram results.bin
```

### Performance Characteristics

| Metric | Capability |
|--------|------------|
| Max requests/sec | 50,000+ (single instance) |
| Concurrent connections | 10,000+ |
| Memory usage | ~100 MB for 1M requests |
| Distributed testing | Yes (multiple instances) |

### Known Limitations

1. **HTTP only** — Cannot test WebSocket or gRPC
   - **Impact:** Not relevant for this REST API project

2. **Go binary required** — Must be installed separately
   - **Impact:** Minimal; easy to install via `go install`

### Alternatives Considered

#### Locust

**Why not chosen:**
- Requires Python code (not just CLI)
- Heavier than Vegeta for simple HTTP testing
- More setup required

**When would it be better?**
- For complex user journey simulation
- When Python is already part of the stack
- For web UI monitoring during tests

---

## Reverse Proxy

### Choice: Nginx

### Role in System

Nginx provides:
- SSL termination
- Rate limiting
- Connection pooling to backend
- Static file serving
- HTTP/2 support

### Why Nginx over Alternatives?

| Aspect | Nginx | HAProxy | Caddy | Traefik |
|--------|-------|---------|-------|---------|
| **SSL termination** | ✅ | ✅ | ✅ Auto | ✅ Auto |
| **Rate limiting** | ✅ | ✅ | ⚠️ Basic | ✅ |
| **Static files** | ✅ Excellent | ❌ No | ✅ Good | ⚠️ Basic |
| **HTTP/2** | ✅ | ⚠️ Limited | ✅ | ✅ |
| **Configuration** | ⚠️ Manual | ⚠️ Manual | ✅ Auto | ✅ Auto |
| **Maturity** | ✅ Very mature | ✅ Mature | ⚠️ Newer | ⚠️ Newer |

### Specific Nginx Features for This Use Case

#### 1. Rate Limiting

```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;

location /api/ {
    limit_req zone=api_limit burst=20 nodelay;
    proxy_pass http://api_backend;
}
```

**Benefits:**
- Protects against abusive clients
- Prevents DoS attacks
- Fair resource allocation

#### 2. Connection Pooling

```nginx
upstream api_backend {
    server 127.0.0.1:8080;
    keepalive 32;
}
```

**Benefits:**
- Fewer connections to Go application
- Better resource utilization
- Reduced connection overhead

#### 3. HTTP/2 Support

```nginx
listen 443 ssl http2;
```

**Benefits:**
- Multiplexing (multiple requests over one connection)
- Header compression (HPACK)
- Better performance for concurrent requests

### Known Limitations

1. **Manual configuration** — No automatic service discovery
   - **Impact:** Must update config when backend changes
   - **Mitigation:** Acceptable for single-backend deployment

### Alternatives Considered

#### Caddy

**Why not chosen:**
- Less mature than Nginx
- Fewer configuration options
- Smaller community

**When would it be better?**
- When automatic HTTPS is critical
- For simple setups with auto-configuration

---

## Containerization

### Choice: Docker + Docker Compose

### Role in System

Docker provides:
- Reproducible development environment
- Easy local testing
- Consistent deployment
- Resource isolation

### Why Docker over Alternatives?

| Aspect | Docker | Podman | Kubernetes |
|--------|--------|--------|------------|
| **Complexity** | ✅ Simple | ✅ Simple | ❌ Complex |
| **Local development** | ✅ Excellent | ✅ Excellent | ⚠️ Overkill |
| **Compose support** | ✅ Native | ✅ Podman Compose | ❌ No |
| **Community** | ✅ Large | ⚠️ Growing | ✅ Large |
| **Orchestration** | ⚠️ Swarm | ❌ None | ✅ Native |

### Specific Docker Features for This Use Case

#### 1. Docker Compose

```yaml
services:
  postgres:
    image: postgres:16-alpine
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine

  api:
    build: .
    depends_on:
      - postgres
      - redis
```

**Benefits:**
- Single command to start entire stack
- Reproducible across machines
- Easy to reset state

#### 2. Multi-stage Builds

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o sensor-api ./cmd/api

# Runtime stage
FROM alpine:latest
COPY --from=builder /app/sensor-api /sensor-api
CMD ["/sensor-api"]
```

**Benefits:**
- Small final image (~20 MB)
- No build tools in runtime image
- Faster deployment

### Known Limitations

1. **Resource overhead** — Containers add some overhead
   - **Impact:** Minimal for this use case
   - **Mitigation:** Acceptable tradeoff for reproducibility

### Alternatives Considered

#### Kubernetes

**Why not chosen:**
- Overkill for local development
- Complex setup
- Steeper learning curve

**When would it be better?**
- For production deployment with multiple services
- When auto-scaling is required
- For multi-node deployments

---

## Related Documentation

- [architecture.md](architecture.md) — System architecture and design decisions
- [api-spec.md](api-spec.md) — API contract and endpoint specifications
- [testing.md](testing.md) — Load testing methodology and scenarios
