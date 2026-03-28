# Performance Targets and Validation

This guide covers defining, measuring, and validating performance targets for high-throughput PostgreSQL + Golang systems.

## Overview

Performance targets are critical for ensuring your system meets user expectations. This document covers:

1. **Understanding latency metrics** (p50, p95, p99)
2. **Setting performance targets** for your use case
3. **Load testing with k6**
4. **Key metrics to monitor**
5. **Validating performance in production**

## Understanding Latency Metrics

### Percentiles Explained

Latency is not uniform - some requests are fast, some are slow. Percentiles help understand the distribution:

| Metric | Definition | Example | Use Case |
|--------|------------|---------|----------|
| **p50 (Median)** | 50% of requests complete faster | 200ms | Typical user experience |
| **p95** | 95% of requests complete faster | 400ms | **Primary performance target** |
| **p99** | 99% of requests complete faster | 800ms | Worst-case tolerance |
| **p99.9** | 99.9% of requests complete faster | 1200ms | Extreme outliers |

### Why Median (p50) Matters

```
Average (mean):  Can be skewed by outliers
Median (p50):    Represents "typical" experience
```

**Example**:
- 99 requests: 10ms each
- 1 request: 5000ms

```
Average: (99 × 10 + 5000) / 100 = 59.9ms
Median: 10ms
```

The median better represents typical user experience.

### Why p95 is the Primary Target

Most systems target **p95 latency** because:
- It represents 95% of user experiences
- Balances optimization cost vs user satisfaction
- Industry standard for SLA definitions
- Catches most performance issues without over-optimizing

## Setting Performance Targets

### Industry Benchmarks

| Use Case | p50 Target | p95 Target | p99 Target |
|----------|-----------|-----------|-----------|
| **API calls** | < 200ms | < 500ms | < 1000ms |
| **Database queries** | < 50ms | < 200ms | < 500ms |
| **Cache hits** | < 5ms | < 20ms | < 50ms |
| **Page loads** | < 1s | < 2s | < 3s |

### Target Recommendations

Based on the reference implementation (50M rows):

```
Primary Goal: p95 ≤ 500ms for exact-ID queries

Breakdown:
├─ Hot key (cached): p95 ≤ 20ms
├─ Cold key (database): p95 ≤ 400ms
└─ API overhead: ≤ 20ms
```

### Latency Budget

Allocate your 500ms budget across components:

| Component | Budget | Actual | Status |
|-----------|--------|-------|--------|
| **API Handler** | 20ms | 5ms | ✅ Under budget |
| **Service Layer** | 10ms | 1ms | ✅ Under budget |
| **Cache Hit** | 20ms | 5ms | ✅ Under budget |
| **Database Query** | 400ms | 50-200ms | ✅ Under budget |
| **Buffer** | 50ms | - | ✅ Available |
| **Total** | 500ms | 50-200ms | ✅ Target met |

## Load Testing with k6

### Why k6?

| Feature | k6 | Alternatives |
|---------|-----|-------------|
| **JavaScript-based** | ✅ Easy to write | Various (Lua, Python) |
| **Built-in percentiles** | ✅ Native support | Limited |
| **Docker integration** | ✅ Container-ready | Mixed |
| **Cloud execution** | ✅ k6 Cloud | Vendor-specific |
| **Free and open-source** | ✅ No cost | Some paid |

### Installation

```bash
# macOS
brew install k6

# Linux
sudo apt-get install k6

# Docker
docker pull grafana/k6:latest
```

### k6 Test Scenario Structure

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// Custom metrics
const entityQueryLatency = new Trend('entity_query_latency');

// Configuration
export const options = {
  scenarios: {
    exact_id_queries: {
      executor: 'constant-arrival-rate',
      rate: 100,           // 100 requests per second
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 10,
      maxVUs: 100,
    },
  },
  thresholds: {
    'http_req_duration': ['p(50)<300', 'p(95)<500', 'p(99)<800'],
    'http_req_failed': ['rate<0.01'],  // < 1% error rate
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const ENTITIES = ['entity-000001', 'entity-000002', /* ... */];

export default function () {
  const entityId = ENTITIES[Math.floor(Math.random() * ENTITIES.length)];
  const url = `${BASE_URL}/api/v1/entity-readings?entity_id=${entityId}&limit=100`;

  const startTime = Date.now();
  const response = http.get(url, {
    tags: { name: 'EntityQuery' },
  });
  const duration = Date.now() - startTime;

  entityQueryLatency.add(duration);

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'has data': (r) => {
      try {
        const body = r.json();
        return Array.isArray(body.data) && body.data.length > 0;
      } catch {
        return false;
      }
    },
  });

  sleep(Math.random() * 0.1);  // 0-100ms think time
}
```

### Test Scenarios

#### 1. Hot Key Pattern (Zipf Distribution)

Tests cache effectiveness when 20% of entities get 80% of traffic:

```javascript
const HOT_ENTITIES = ['entity-000001', 'entity-000002', /* ... 20 entities */];
const COLD_ENTITIES = ['entity-000021', 'entity-000022', /* ... 80 entities */];

function zipfEntity() {
  // 80% chance for hot entity
  if (Math.random() < 0.8) {
    return HOT_ENTITIES[Math.floor(Math.random() * HOT_ENTITIES.length)];
  }
  return COLD_ENTITIES[Math.floor(Math.random() * COLD_ENTITIES.length)];
}

export default function () {
  const entityId = zipfEntity();
  // ... query with entityId
}
```

#### 2. Time-Range Queries

Tests BRIN index performance:

```javascript
export default function () {
  const timeRanges = [
    '1h',   // Last hour
    '24h',  // Last day
    '7d',   // Last week
  ];

  const range = timeRanges[Math.floor(Math.random() * timeRanges.length)];
  const url = `${BASE_URL}/api/v1/entity-readings?entity_id=${entityId}&time_range=${range}`;
  // ...
}
```

#### 3. Mixed Workload

Tests realistic API usage:

```javascript
const WORKLOAD_MIX = {
  HEALTH_CHECK: 0.10,    // 10% health checks
  STATS: 0.20,           // 20% statistics queries
  ENTITY_READINGS: 0.70, // 70% entity queries
};

function selectWorkload() {
  const rand = Math.random();
  if (rand < WORKLOAD_MIX.HEALTH_CHECK) {
    return 'health_check';
  } else if (rand < WORKLOAD_MIX.HEALTH_CHECK + WORKLOAD_MIX.STATS) {
    return 'stats';
  }
  return 'entity_readings';
}
```

### Running k6 Tests

```bash
# Local testing
k6 run tests/scenarios/exact-id-queries.js

# With custom RPS
k6 run --env CUSTOM_RPS=200 tests/scenarios/exact-id-queries.js

# With custom duration
k6 run --env CUSTOM_DURATION=10m tests/scenarios/exact-id-queries.js

# With remote target
k6 run --env TARGET_URL=https://api.example.com tests/scenarios/exact-id-queries.js

# Docker
docker run --rm -i --network host \
  grafana/k6:latest run \
  -e TARGET_URL=http://localhost:8080 \
  - < tests/scenarios/exact-id-queries.js
```

### Interpreting k6 Results

```
✓ status is 200
✓ response time < 500ms
✓ has data

checks.........................: 100.00% ✓ 30000      ✗ 0
data_received..................: 15 MB  50 kB/s
data_sent......................: 2.0 MB 6.7 kB/s
http_req_blocked...............: avg=1ms    min=1µs    med=4µs    max=50ms   p(95)=10ms
http_req_connecting............: avg=10ms   min=0s     med=0s     max=100ms  p(95)=50ms
http_req_duration..............: avg=50ms   min=2ms    med=10ms   max=500ms  p(95)=100ms p(99)=200ms
{ expected_response:true }...: avg=50ms   min=2ms    med=10ms   max=500ms  p(95)=100ms p(99)=200ms
http_req_failed................: 0.00%   ✓ 0        ✗ 30000
http_req_receiving.............: avg=5ms    min=10µs   med=1ms    max=100ms  p(95)=10ms
http_req_sending...............: avg=1ms    min=5µs    med=20µs   max=50ms   p(95)=5ms
http_req_tls_handshaking.......: avg=0s     min=0s     med=0s     max=0s     p(95)=0s
http_req_waiting...............: avg=44ms   min=1ms    med=8ms    max=450ms  p(95)=90ms  p(99)=180ms
http_reqs......................: 30000   100.000207/s
iteration_duration.............: avg=151ms  min=12ms   med=100ms  max=600ms  p(95)=300ms p(99)=400ms
iterations.....................: 30000   100.000207/s
vus............................: 10      min=10     max=100
vus_max........................: 100     min=100    max=100
```

**Key metrics**:
- `http_req_duration`: Overall request latency
- `p(95)`: 95th percentile (should be < 500ms)
- `http_req_failed`: Error rate (should be < 1%)

## Key Metrics to Monitor

### Database Metrics

```sql
-- Connection pool stats
SELECT
    count(*) as total_connections,
    count(*) FILTER (WHERE state = 'active') as active_connections,
    count(*) FILTER (WHERE state = 'idle') as idle_connections
FROM pg_stat_activity
WHERE datname = 'app_db';

-- Index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
ORDER BY idx_scan ASC;  -- Unused indexes first

-- Table size
SELECT
    pg_size_pretty(pg_total_relation_size('entity_readings')) as total_size,
    pg_size_pretty(pg_relation_size('entity_readings')) as table_size,
    pg_size_pretty(pg_total_relation_size('entity_readings') - pg_relation_size('entity_readings')) as index_size;
```

### Cache Metrics

```bash
# Redis info
redis-cli INFO stats

# Key metrics:
# - hits: Cache hits
# - misses: Cache misses
# - hit_rate: hits / (hits + misses)
# - memory_used: Current memory usage
# - evicted_keys: Number of keys evicted
```

**Target**: Hit rate > 80% for hot key pattern

### API Metrics

Add a `/metrics` endpoint:

```go
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
    stats := h.service.Stats()

    metrics := map[string]interface{}{
        "database": map[string]interface{}{
            "open_connections": stats.DB.OpenConnections,
            "idle_connections": stats.DB.IdleConnections,
            "max_connections":  stats.DB.MaxConnections,
        },
        "cache": map[string]interface{}{
            "enabled":    stats.Cache.Enabled,
            "hit_rate":   stats.Cache.HitRate,
            "memory_use": stats.Cache.MemoryUsed,
        },
        "requests": map[string]interface{}{
            "total":      stats.Requests.Total,
            "successful": stats.Requests.Successful,
            "failed":     stats.Requests.Failed,
        },
    }

    json.NewEncoder(w).Encode(metrics)
}
```

## Validating Performance in Production

### A/B Testing

Deploy new version alongside old, compare performance:

```
Version A (baseline): 10% of traffic
Version B (new):      90% of traffic

Measure:
- p50, p95, p99 latency
- Error rate
- Throughput
- Database load
- Cache hit rate
```

### Gradual Rollout

```
1. Deploy to 1 instance (10% traffic)
2. Monitor for 15 minutes
3. If metrics are good, deploy to 50% traffic
4. Monitor for 30 minutes
5. If metrics are good, deploy to 100% traffic
```

### Alert Thresholds

Configure alerts for:

| Metric | Warning | Critical |
|--------|---------|----------|
| **p95 latency** | > 400ms | > 500ms |
| **Error rate** | > 0.5% | > 1% |
| **Cache hit rate** | < 70% | < 50% |
| **DB connections** | > 80% | > 90% |

### Performance Regression Testing

Run k6 tests on every deployment:

```yaml
# .github/workflows/performance.yml
name: Performance Tests

on: [pull_request, push]

jobs:
  k6:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run k6
        run: |
          docker-compose up -d
          ./tests/run-benchmarks.sh --rps 100 --duration 5m
```

## Performance Optimization Checklist

### Before Testing

- [ ] Database indexes created (BRIN, composite, covering)
- [ ] Connection pool configured (50 max, 10 min)
- [ ] Redis caching enabled (30s TTL)
- [ ] Materialized views refreshed
- [ ] Database statistics updated (ANALYZE)
- [ ] Sufficient test data (≥ 1M rows for realistic tests)

### During Testing

- [ ] Monitor database connection pool
- [ ] Monitor cache hit rate
- [ ] Monitor error rate
- [ ] Check for N+1 queries
- [ ] Verify index usage with EXPLAIN ANALYZE

### After Testing

- [ ] Compare p50, p95, p99 against targets
- [ ] Identify slow queries (≥ 500ms)
- [ ] Check for unused indexes
- [ ] Review cache effectiveness
- [ ] Document optimization results

## Common Performance Issues

### Issue: High p95 Despite Good p50

**Cause**: Tail latency from cache misses or slow queries

**Solution**:
- Increase cache TTL (30s → 60s)
- Add covering indexes for hot queries
- Pre-populate cache for known hot keys

### Issue: Increasing Latency Over Time

**Cause**: Connection pool exhaustion or memory bloat

**Solution**:
- Increase `MaxConns`
- Reduce `MaxConnLifetime`
- Add connection pool monitoring

### Issue: High Error Rate Under Load

**Cause**: Database overload or timeout

**Solution**:
- Increase `max_connections` in PostgreSQL
- Add connection pool (PgBouncer)
- Implement rate limiting
- Add request queuing

## Best Practices Summary

1. **Target p95 ≤ 500ms** for exact-ID queries
2. **Use k6 for load testing** with realistic scenarios
3. **Monitor database, cache, and API metrics** continuously
4. **Set up alerts** for performance degradation
5. **Run performance tests on every deployment**
6. **Test with production data volume** (≥ 1M rows)
7. **Use gradual rollout** for new versions
8. **Document performance baselines** and regressions

## Next Steps

- [General Setup Guide](./04-general-setup-guide.md) - Complete implementation guide
- [PostgreSQL Setup](./01-postgresql-setup.md) - Database configuration
- [Golang API Setup](./02-golang-api-setup.md) - API layer implementation
