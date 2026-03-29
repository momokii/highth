# High-Throughput PostgreSQL + Golang Guide

A production-ready guide to building a PostgreSQL + Golang stack that achieves **≤500ms median latency** for exact-ID queries at scale.

## Overview

This guide demonstrates how to architect and implement a high-throughput data system using PostgreSQL and Golang. The patterns and techniques described here have been validated at production scale, handling **50M+ rows** with sub-10ms median latency under load.

### Performance Targets

| Metric | Target | Purpose |
|--------|--------|---------|
| **Median (p50)** | < 300ms | Typical user experience |
| **95th Percentile (p95)** | ≤ 500ms | Primary performance goal |
| **99th Percentile (p99)** | < 800ms | Worst-case tolerance |
| **Error Rate** | < 1% | Reliability threshold |

### What You'll Learn

- How to configure PostgreSQL for high-throughput workloads
- Proper indexing strategies for exact-ID queries
- Golang connection pool configuration for optimal performance
- Caching strategies using Redis for hot data
- Performance testing methodologies with k6
- Step-by-step setup for any use case

## Quick Start

1. **[PostgreSQL Setup](./01-postgresql-setup.md)** - Database configuration and indexing
2. **[Golang API Setup](./02-golang-api-setup.md)** - API layer and connection pooling
3. **[Performance Targets](./03-performance-targets.md)** - Metrics and validation
4. **[General Setup Guide](./04-general-setup-guide.md)** - Adapt to your use case
5. **[Configuration Adjustment Guide](./05-configuration-adjustment-guide.md)** - Adjust for your hardware

## Target Audience

This guide is for developers and architects who:

- Are building applications with high-throughput data requirements
- Need to serve exact-ID queries consistently under 500ms
- Want to leverage PostgreSQL's advanced features (BRIN, covering indexes, materialized views)
- Prefer Golang for its concurrency model and performance
- Need production-ready patterns, not just theoretical concepts

### Use Cases

While the reference implementation uses IoT sensor data, these patterns apply to:

- **User activity logs** - "Get recent actions for user X"
- **Transaction history** - "Get last N transactions for account Y"
- **Event streams** - "Get events for entity Z within time range"
- **Time-series data** - Any append-only data with identifier queries
- **Analytics platforms** - Real-time statistics on large datasets

## Core Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client Layer                        │
│                   (Web, Mobile, API)                        │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                      API Layer (Go)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Handler    │  │   Service    │  │   Repository     │  │
│  │  (HTTP/REST) │  │ (Business    │  │   (Data Access)  │  │
│  │              │  │  Logic)      │  │                   │  │
│  └──────────────┘  └──────┬───────┘  └────────┬──────────┘  │
└────────────────────────────┼──────────────────┼─────────────┘
                               │                  │
                 ┌─────────────┴────────┐        │
                 │                      │        │
                 ▼                      ▼        ▼
        ┌─────────────────┐    ┌────────────────┐
        │  Redis Cache    │    │  PostgreSQL    │
        │  (Hot Data)     │    │  (Primary DB)  │
        │                 │    │                │
        │  - 30s TTL      │    │  - BRIN Index  │
        │  - LRU Eviction │    │  - Covering    │
        │                 │    │  - Materialized│
        └─────────────────┘    │    Views       │
                               └────────────────┘
```

## Key Performance Optimizations

### 1. PostgreSQL Indexing Strategy

| Index Type | Use Case | Size vs B-tree | Performance |
|------------|----------|----------------|-------------|
| **BRIN** | Append-only time-series | 99% smaller | Excellent for time-range queries |
| **Composite** | Multi-column queries | Same size | Optimizes "get recent N for entity X" |
| **Covering** | Index-only scans | Slightly larger | 2-5x faster (no heap access) |
| **Materialized View** | Aggregations | Separate storage | 100x faster than real-time |

### 2. Connection Pooling

**PostgreSQL Side:**
- `max_connections`: 200
- `shared_buffers`: 2GB (25% of RAM)
- Connection overhead: ~10-50ms per new connection

**Application Side (pgx):**
- `MaxConns`: 50
- `MinConns`: 10
- `MaxConnLifetime`: 1h
- `MaxConnIdleTime`: 10m
- `HealthCheckPeriod`: 30s

### 3. Caching Strategy (Redis)

- **Pattern**: Cache-aside (check cache → query DB → populate cache)
- **TTL**: 30 seconds (balance freshness vs performance)
- **Eviction**: LRU (least recently used)
- **Impact**: 5-15ms (cache hit) vs 200-400ms (database hit)

### 4. Query Optimization

**Index-only scans with covering indexes:**
```sql
-- Covering index eliminates heap access
CREATE INDEX idx_covering ON readings
(entity_id, timestamp DESC)
INCLUDE (value, type, metadata);
```

**Materialized views for statistics:**
```sql
-- Pre-computed aggregations
CREATE MATERIALIZED VIEW mv_entity_stats AS
SELECT entity_id, COUNT(*), AVG(value)
FROM readings
GROUP BY entity_id;
```

## Prerequisites

### Software Requirements

| Component | Minimum Version | Recommended |
|-----------|-----------------|-------------|
| PostgreSQL | 14+ | 16 (BRIN improvements) |
| Golang | 1.20+ | 1.21+ |
| Redis | 6+ | 7 (recommended) |
| k6 | - | Latest (for testing) |

### Hardware Recommendations

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| RAM | 4GB | 8GB+ |
| CPU | 2 cores | 4+ cores |
| Storage | SSD | NVMe SSD |
| Network | 1Gbps | 10Gbps |

### Knowledge Assumptions

This guide assumes you are familiar with:

- **PostgreSQL basics**: Tables, indexes, simple queries
- **Golang fundamentals**: Interfaces, goroutines, error handling
- **HTTP/REST APIs**: Request/response patterns, status codes
- **Database concepts**: ACID, transactions, connection pooling

## Document Navigation

### By Topic

- **Database Design** → [PostgreSQL Setup](./01-postgresql-setup.md)
- **API Development** → [Golang API Setup](./02-golang-api-setup.md)
- **Performance Testing** → [Performance Targets](./03-performance-targets.md)
- **Implementation** → [General Setup Guide](./04-general-setup-guide.md)

### By Use Case

- **"I'm starting from scratch"** → Start with [General Setup Guide](./04-general-setup-guide.md)
- **"I need to optimize my database"** → Read [PostgreSQL Setup](./01-postgresql-setup.md)
- **"My API is too slow"** → Read [Golang API Setup](./02-golang-api-setup.md)
- **"I need to prove performance"** → Read [Performance Targets](./03-performance-targets.md)

## Example Implementation

This guide is based on a production-validated reference implementation available in the parent repository. The example demonstrates:

- 50M+ row dataset with sub-10ms median latency
- BRIN indexes for time-series optimization
- Covering indexes for index-only scans
- Redis caching with 30s TTL
- Materialized views for analytics queries
- k6 load testing scenarios

See the [examples](./examples/) folder for schema and configuration files.

## Performance Results (Actual)

From the reference implementation with 50M rows:

| Scenario | p50 | p95 | p99 | Throughput |
|----------|-----|-----|-----|------------|
| **Hot device** | 2ms | 7ms | 15ms | 450+ RPS |
| **Cold device** | 3ms | 12ms | 25ms | 450+ RPS |
| **Time-range** | 5ms | 20ms | 40ms | 300+ RPS |
| **Mixed workload** | 2ms | 8ms | 18ms | 470+ RPS |

**Note**: Results include cache hits. Cold cache performance: 50-200ms depending on query complexity.

## Next Steps

1. Read through all four main documents
2. Review the [example files](./examples/)
3. Adapt the patterns to your use case
4. Run the k6 test scenarios to validate performance
5. Iterate on indexing and caching based on your workload

## Contributing

This guide is maintained as part of the Higth project. For questions or improvements, please refer to the main repository.

## License

This documentation is provided as-is for educational and production use.
