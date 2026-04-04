# Master Implementation Plan

This document provides the complete phased implementation roadmap for building the High-Performance IoT Sensor Query System.

## Table of Contents

- [Phase Overview](#phase-overview)
- [Phase 0: Environment & Tooling](#phase-0-environment--tooling)
- [Phase 1: Database Provisioning](#phase-1-database-provisioning)
- [Phase 2: Data Generation](#phase-2-data-generation)
- [Phase 3: API Development](#phase-3-api-development)
- [Phase 4: Caching Integration](#phase-4-caching-integration)
- [Phase 5: Load Testing](#phase-5-load-testing)
- [Phase 6: Results Analysis](#phase-6-results-analysis)
- [Done Criteria](#done-criteria)

---

## Phase Overview

### Phase Structure Template

Each phase follows this structure:

| Element | Description |
|---------|-------------|
| **Goal** | Specific, measurable objective |
| **Entry Criteria** | What must be true before starting |
| **Exit Criteria** | What must be true before proceeding |
| **Deliverables** | Concrete outputs produced |
| **Risk/Mitigation** | Likely blockers and solutions |

### Estimated Effort

| Phase | Time | Prerequisites |
|-------|------|---------------|
| Phase 0: Environment & Tooling | 2-3 hours | None |
| Phase 1: Database Provisioning | 2-4 hours | Phase 0 |
| Phase 2: Data Generation | 1-3 hours | Phase 1 |
| Phase 3: API Development | 4-8 hours | Phase 1, Phase 2 |
| Phase 4: Caching Integration | 1-2 hours | Phase 3 |
| Phase 5: Load Testing | 2-4 hours | Phase 4 |
| Phase 6: Results Analysis | 1-2 hours | Phase 5 |
| **Total** | **13-26 hours** | **~2-4 days** |

---

## Phase 0: Environment & Tooling

### Goal

Establish a working development environment with all required tools installed and configured.

### Entry Criteria

- Machine with at least 8GB RAM and SSD storage
- Linux or macOS operating system
- Internet connection for downloading tools
- sudo/admin access for package installation

### Exit Criteria

- [ ] Go 1.21+ installed and verified
- [ ] Docker and Docker Compose installed
- [ ] PostgreSQL client tools (`psql`) installed
- [ ] Redis client tools (`redis-cli`) installed
- [ ] Vegeta installed and accessible in PATH
- [ ] Project directory structure created
- [ ] `.env.example` file created with all required variables

### Deliverables

1. Configured development machine
2. `.env.example` file with environment variable templates
3. Initialized `go.mod` file
4. Project folder structure

### Key Setup Commands

```bash
# Verify Go (requires 1.21+)
go version

# Verify Docker
docker --version
docker-compose --version

# Verify PostgreSQL client
psql --version

# Verify Redis client
redis-cli --version

# Verify Vegeta
vegeta --version

# Create project structure
mkdir -p cmd/api internal/{handler,service,repository,cache,model,config} pkg
touch go.mod .env.example
```

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Version conflicts | Use version pinning in go.mod; document exact versions in this guide |
| Docker resource limits | Allocate at least 4GB RAM to Docker Desktop |
| PATH issues | Add Go bin directory to PATH in `.bashrc` or `.zshrc` |
| WSL2 issues (Windows) | Use WSL2 with systemd enabled; avoid WSL1 |

### Detailed Instructions

See **[dev-environment.md](dev-environment.md)** for complete installation instructions.

---

## Phase 1: Database Provisioning & Schema

### Goal

Provision PostgreSQL 16+ with optimized schema and indexes for 50M+ rows.

### Entry Criteria

- Phase 0 complete
- Docker available or PostgreSQL 16+ installed
- At least 10GB free disk space

### Exit Criteria

- [ ] PostgreSQL 16+ running and accessible
- [ ] Database `sensor_db` created
- [ ] Table `sensor_readings` created with correct schema
- [ ] BRIN index on `timestamp` created
- [ ] Composite B-tree index on `(device_id, timestamp DESC)` created
- [ ] Covering index created with INCLUDE clause
- [ ] `ANALYZE` run on the table
- [ ] Connection string tested and verified

### Deliverables

1. Running PostgreSQL instance (Docker or native)
2. `init.sql` file with complete schema DDL
3. Tested database connection string
4. Verified index creation

### Schema Overview

```sql
CREATE TABLE sensor_readings (
    id              BIGSERIAL       PRIMARY KEY,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(30)     NOT NULL,
    value           NUMERIC(15,6)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL,
    metadata        JSONB
);
```

### Index Creation Order

**Important:** Create indexes in this order for optimal performance:

1. **BRIN index** (fastest) — For time-range queries
2. **Composite B-tree** (medium) — For device lookups
3. **Covering index** (slowest) — For index-only scans

### PostgreSQL Configuration

Key parameters for 50M row performance:

```ini
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 16MB
maintenance_work_mem = 128MB
random_page_cost = 1.1
effective_io_concurrency = 200
```

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Insufficient disk space | Check available space before starting; need ~30GB total |
| Index creation timeout | Increase `maintenance_work_mem` for covering index |
| Configuration not applied | Verify with `SHOW ALL;` after startup |
| Connection refused | Verify port 5432 is not in use; check Docker logs |

### Detailed Instructions

See **[database-setup.md](database-setup.md)** for complete database provisioning steps.

---

## Phase 2: Data Generation

### Goal

Generate 50M rows of realistic test data with non-uniform identifier distribution (Zipf-like).

### Entry Criteria

- Phase 1 complete
- Database ready for inserts
- At least 20GB free disk space

### Exit Criteria

- [ ] 50,000,000 rows inserted into `sensor_readings`
- [ ] Distribution verified (median ~40K readings/device, max ~200K)
- [ ] `ANALYZE` run after insertion
- [ ] Indexes verified as functional
- [ ] Index sizes documented

### Deliverables

1. Data generation script (Go or SQL)
2. Verification report of data distribution
3. Total row count confirmed

### Distribution Model

**Zipf-like distribution for realistic hot keys:**

| Percentile | Devices | Readings/Device | Total | % of Data |
|-----------|---------|-----------------|-------|-----------|
| Top 1% | 10 | 200,000 | 2,000,000 | 4% |
| Top 5% | 50 | 150,000 | 7,500,000 | 15% |
| Top 20% | 200 | 75,000 | 15,000,000 | 30% |
| Middle 40% | 400 | 40,000 | 16,000,000 | 32% |
| Bottom 40% | 400 | 12,500 | 5,000,000 | 10% |
| **Total** | **1,000** | **50,000 avg** | **50,000,000** | **100%** |

### Generation Strategy

**Batch insertion** is critical for performance:
- Batch size: 1000 rows per transaction
- Disable autovacuum during load (temporarily)
- Disable synchronous commit (data is test data, can be lost)
- Use copy command for fastest insertion (optional alternative)

### Estimated Generation Time

| Hardware | Expected Time |
|----------|---------------|
| HDD + 2 cores | 4-6 hours (not recommended) |
| SSD + 4 cores | 1-2 hours |
| SSD + 8 cores | 30-60 minutes |
| NVMe + 8 cores | 20-40 minutes |

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Generation takes too long | Use batch inserts; disable autovacuum; use SSD |
| Disk space exhaustion | Monitor with `df -h`; stop before full if needed |
| Distribution not realistic | Use proper Zipf distribution; verify with percentiles |
| Memory exhaustion | Reduce batch size to 500; monitor RAM usage |
| Database connection drops | Use connection pool with adequate timeout |

### Detailed Instructions

See **[data-generation.md](data-generation.md)** for complete data generation strategy.

---

## Phase 3: API Development

### Goal

Build complete Go API with chi router, pgx connection pooling, and caching integration points.

### Entry Criteria

- Phase 1 complete (database ready)
- Phase 2 complete (data loaded, or can be done in parallel)
- Go 1.21+ installed

### Exit Criteria

- [ ] API server runs on port 8080
- [ ] GET `/api/v1/sensor-readings` returns data
- [ ] GET `/api/v1/sensor-readings/{id}` returns single reading by primary key
- [ ] GET `/api/v1/sensor-readings/{id}` returns single reading by primary key
- [ ] GET `/health` returns database/cache status
- [ ] Connection pooling configured (25 max, 5 min)
- [ ] Request validation implemented
- [ ] Error handling returns proper HTTP status codes
- [ ] All code compiles without errors
- [ ] `.env` file configuration working

### Deliverables

1. `cmd/api/main.go` — Application entry point
2. `internal/handler/` — HTTP request handlers
3. `internal/service/` — Business logic layer
4. `internal/repository/` — Database queries
5. `internal/cache/` — Redis client wrapper
6. `internal/model/` — Data structures
7. `internal/config/` — Configuration loading
8. `go.mod` and `go.sum` — Dependencies

### Core Dependencies

```go
require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/jackc/pgx/v5 v5.5.1
    github.com/redis/go-redis/v9 v9.4.0
    github.com/google/uuid v1.5.0
)
```

### Request Flow

```
Request → Handler → Validation → Cache Check → Repository (if miss) → Response
                              ↓ Hit
                         Return Cached
```

### Error Handling

| Error | HTTP Status | Error Code |
|-------|-------------|------------|
| Invalid parameter | 400 | INVALID_PARAMETER |
| Device not found | 404 | DEVICE_NOT_FOUND |
| Database error | 500 | INTERNAL_ERROR |
| Cache error | Log only, continue | N/A |

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Connection pool exhaustion | Configure MaxConn appropriately; monitor pool stats |
| Context timeout on slow queries | Use `context.WithTimeout()` for DB operations |
| Memory leaks | Ensure `defer rows.Close()` in all code paths |
| Import errors | Verify `go.mod` has correct module path |

### Detailed Instructions

See **[api-development.md](api-development.md)** for complete API build-out guide.

---

## Phase 4: Caching Integration

### Goal

Integrate Redis caching with write-through pattern and 30s TTL.

### Entry Criteria

- Phase 3 complete (API functional)
- Redis 7+ available (Docker or native)

### Exit Criteria

- [ ] Redis caching enabled and working
- [ ] Cache hits return in <10ms
- [ ] Cache misses populate cache correctly
- [ ] 30s TTL functioning (verify with TTL command)
- [ ] Cache hit rate monitorable (logs or metrics)
- [ ] Graceful degradation when Redis unavailable

### Deliverables

1. `internal/cache/redis_cache.go` implementation
2. Cache key pattern documented and consistent
3. TTL configuration in environment variables
4. Cache hit rate metrics/logging

### Cache Key Pattern

```
Format: sensor:{device_id}:readings:{limit}[:{reading_type}]

Examples:
- sensor:sensor-001:readings:10
- sensor:sensor-002:readings:50:temperature
```

### Cache Strategy

- **TTL:** 30 seconds
- **Population:** Write-through on cache miss
- **Invalidation:** Time-based (TTL) only
- **Fallback:** Serve from database if cache unavailable
- **Eviction:** LRU (allkeys-lru policy)

### Performance Targets

| Operation | Target Latency |
|-----------|----------------|
| Cache hit | <10ms |
| Cache miss (warm DB) | 200-400ms |
| Cache miss (cold DB) | 200-600ms |

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Redis unavailable | Log error; serve from DB; alert via health check |
| Stale data concerns | 30s TTL is acceptable for IoT monitoring |
| Cache stampede | Not an issue for read-heavy workload |
| Memory exhaustion | LRU eviction handles automatically |

### Detailed Instructions

See **[cache-setup.md](cache-setup.md)** for complete caching integration guide.

---

## Phase 5: Load Testing

### Goal

Execute all 6 test scenarios and collect performance metrics to validate ≤500ms target.

### Entry Criteria

- Phase 4 complete (full system running)
- Vegeta installed
- Dataset at target scale (50M rows)
- At least 1GB free RAM for load testing

### Exit Criteria

- [ ] All 6 test scenarios executed
- [ ] Results saved in timestamped directory
- [ ] Pass/fail status determined for each scenario
- [ ] Performance degradation documented (if scale testing)
- [ ] Test summary report created

### Deliverables

1. `test-runner.sh` script
2. Results in `./test-results/{timestamp}/`
3. Pass/fail determination for each scenario
4. Performance metrics summary

### Test Execution Order

1. **Health Check** — System sanity verification
2. **Cold Start** — Baseline without cache (flush Redis first)
3. **Baseline** — Single-thread minimum latency
4. **Concurrent** — PRIMARY TEST: 50 users, 60 seconds
5. **Hot Device** — 90% queries to same device
6. **Large N** — Request 500 records (max limit)
7. **PK Lookup** — Single reading by primary key ID (B-tree index scan)
8. **Scale** — If multiple dataset sizes available

### Pass/Fail Criteria

| Scenario | p50 Target | p95 Target | Notes |
|----------|------------|------------|-------|
| Baseline | ≤50ms | ≤100ms | Cache warm |
| Concurrent | ≤500ms | ≤800ms | Primary validation |
| Hot Device | ≤500ms | ≤600ms | No outliers |
| Cold Start | ≤600ms | ≤1000ms | Higher tolerance |
| Large N | ≤500ms | ≤800ms | 500 records |
| PK Lookup | ≤50ms | ≤100ms | Primary key query, tightest thresholds |

### Metrics to Collect

For each test:
- p50, p95, p99 latency
- Requests per second
- Error rate (%)
- Cache hit rate (from API)

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| API crashes under load | Set connection timeout; check logs |
| Database connection exhaustion | Monitor `pg_stat_activity` |
| Inconsistent results | Run each test 2-3 times; use median |
| Cache interference | Flush Redis between tests |

### Detailed Instructions

See **[load-testing-setup.md](load-testing-setup.md)** for complete test execution guide.

---

## Phase 6: Results Analysis

### Goal

Analyze test results, document performance against targets, and provide recommendations.

### Entry Criteria

- Phase 5 complete
- All test results available in `./test-results/`

### Exit Criteria

- [ ] Performance summary document created
- [ ] Pass/fail determination documented
- [ ] Root cause analysis for any failures
- [ ] Recommendations documented
- [ ] Lessons learned captured

### Deliverables

1. `docs/results/performance-report.md`
2. Comparison tables (expected vs actual)
3. Latency distribution (ASCII charts)
4. Conclusions and recommendations

### Success Criteria

**Project succeeds if:**
1. Concurrent load test passes (p50 ≤ 500ms, p95 ≤ 800ms)
2. Hot device test passes (no outliers >2x p95)
3. Scale test shows acceptable degradation
4. Error rate ≤ 1%

**Project fails gracefully if:**
1. Targets missed but documented honestly
2. Root cause identified
3. Mitigation attempted and documented
4. Architecture principles remain sound

### Investigation Priority

If targets are missed, investigate in this order:

1. **Database query plan** — Is covering index being used?
2. **Cache effectiveness** — What is hit rate?
3. **Connection pool** — Are connections exhausted?
4. **Hot keys** — Is one device causing issues?
5. **System resources** — CPU, RAM, disk I/O saturation?

### Documentation Structure

```
## Performance Summary
### Target Achievement
### Test Results Summary
### System Configuration
### Latency Analysis
### Conclusions
### Recommendations
```

### Risk/Mitigation

| Risk | Mitigation |
|------|------------|
| Results inconclusive | Run tests multiple times; use median |
| Hardware limitations | Document honestly; don't overclaim |
| Unexpected failures | Check logs; review system metrics |

### Related Documentation

See **[../testing.md](../testing.md)** for testing methodology context.

---

## Done Criteria

### Full System Readiness

The system is complete when:

#### Database
- [ ] PostgreSQL 16+ running
- [ ] Database `sensor_db` exists
- [ ] Table `sensor_readings` with 50M rows
- [ ] All 3 indexes created and verified
- [ ] `ANALYZE` run on table

#### API
- [ ] Go server runs on port 8080
- [ ] `/api/v1/sensor-readings` endpoint functional
- [ ] `/health` endpoint returns status
- [ ] Connection pooling configured (25 max, 5 min)
- [ ] Request validation working
- [ ] Error handling returning proper codes

#### Cache
- [ ] Redis 7+ running
- [ ] Cache integration working
- [ ] 30s TTL configured
- [ ] Cache hits returning <10ms
- [ ] Graceful degradation when Redis down

#### Testing
- [ ] All 6 scenarios executed
- [ ] Results saved and documented
- [ ] Pass/fail determined
- [ ] p50 ≤ 500ms (or documented why not)
- [ ] p95 ≤ 800ms (or documented why not)

#### Documentation
- [ ] All implementation docs complete
- [ ] Performance report created
- [ ] Lessons learned documented

### Quick Verification Commands

```bash
# Database
psql "postgres://sensor_user:password@localhost:5432/sensor_db" \
  -c "SELECT count(*) FROM sensor_readings;"

# API
curl http://localhost:8080/health
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Cache
redis-cli DBSIZE
redis-cli GET "sensor:sensor-001:readings:10"

# Load Test
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" | \
  vegeta attack -duration=10s -rate=1 | vegeta report -type=text
```

---

## Related Documentation

- **[../README.md](../README.md)** — Project overview
- **[../architecture.md](../architecture.md)** — System architecture
- **[../stack.md](../stack.md)** — Technology stack
- **[../api-spec.md](../api-spec.md)** — API contract
- **[../testing.md](../testing.md)** — Test methodology
