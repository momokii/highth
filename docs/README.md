# High-Performance IoT Sensor Query System

A portfolio-grade demonstration of a properly architected system that can handle large-scale data lookups within strict latency thresholds under realistic, production-like conditions.

## Overview

This project proves that a well-designed schema and tech stack can meet a **≤500ms average query response time** at scale (tens of millions of records), using a genuine real-world use case: **IoT sensor telemetry**.

**This is NOT a raw benchmark.** The goal is to demonstrate:

1. A production-ready architecture that achieves the performance target at scale
2. Generalizable design principles that apply regardless of specific database structure
3. Proper API-first data access (no direct database queries from clients)

## Quick Start

### Reading Order

For the best understanding of this architecture, read the documentation in this order:

1. **[README.md](README.md)** ← You are here (Project overview and quick start)
2. **[architecture.md](architecture.md)** ← Database schema, indexing strategy, caching layer
3. **[stack.md](stack.md)** ← Complete tech stack with comprehensive justifications
4. **[api-spec.md](api-spec.md)** ← Complete API contract and endpoint specifications
5. **[testing.md](testing.md)** ← Test plan, scenarios, and pass/fail criteria
6. **[ui-consideration.md](ui-consideration.md)** ← Portfolio value assessment

### At a Glance

| Aspect | Details |
|--------|---------|
| **Use Case** | IoT sensor telemetry (temperature, humidity, pressure, etc.) |
| **Query Pattern** | "Get the last N readings for device X" |
| **Dataset Scale** | 50M rows primary; designed to scale to 100M+ |
| **Performance Target** | p50 ≤ 500ms, p95 ≤ 800ms |
| **Concurrent Users** | 50+ concurrent clients |
| **Primary Technology** | Go + PostgreSQL + Redis + chi router |

## Use Case Domain: IoT Sensor Telemetry

### Why This Domain?

IoT sensor telemetry is an ideal domain for a high-performance query system because it:

1. **Naturally generates high-volume data** — 10,000 sensors reporting once per minute = 14.4 million records daily
2. **Has a natural repeating identifier** — `device_id` appears across millions of rows
3. **Is genuinely used at production scale** — AWS IoT Core, Azure IoT Hub, industrial IoT platforms
4. **Exhibits realistic data distribution** — Some devices are more active than others (hot keys)
5. **Has skewed query patterns** — Critical infrastructure sensors are queried more frequently

### Real-World Examples

This query pattern is used by:

- **Smart city infrastructure** — Monitoring thousands of environmental sensors
- **Industrial IoT** — Factory equipment monitoring (Siemens MindSphere, GE Predix)
- **Agricultural monitoring** — Soil sensors across large farms
- **Building management** — HVAC and energy sensors in commercial buildings

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT LAYER                                    │
│                    (Load Test Tool / Monitoring Dashboard)                  │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │ HTTP/REST
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API GATEWAY (Nginx)                               │
│                    SSL Termination, Rate Limiting, Caching                  │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         GO HTTP SERVICE (chi router)                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐ │
│  │  Handler Layer  │  │  Service Layer  │  │      Repository Layer       │ │
│  │  (Endpoints)    │──│  (Business)     │──│  (DB Abstraction)           │ │
│  │                 │  │                 │  │                             │ │
│  │  - /sensor/... │  │  - Validation   │  │  - pgx connection pool      │ │
│  │  - /health     │  │  - Cache logic  │  │  - Prepared statements      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────────┘ │
│                                                                              │
│  Goroutine Pool (workers) │ Cache Layer (Redis go-redis)                    │
└─────────────────────┬────────────────────────────────┬──────────────────────┘
                      │                                │
                      ▼                                ▼
┌───────────────────────────────────┐    ┌───────────────────────────────────┐
│         PostgreSQL 16+            │    │           Redis                   │
│  ┌─────────────────────────────┐ │    │  ┌─────────────────────────────┐ │
│  │   sensor_readings Table     │ │    │  │  LRU Cache with TTL         │ │
│  │   (BRIN indexed by ts)      │ │    │  │  (device_id -> readings)    │ │
│  │   Partition-ready by month  │ │    │  │                             │ │
│  └─────────────────────────────┘ │    │  └─────────────────────────────┘ │
└───────────────────────────────────┘    └───────────────────────────────────┘
```

## Performance Targets

| Metric | Target | Rationale |
|--------|--------|-----------|
| **p50 latency** | ≤ 500ms | Primary requirement; achievable with proper indexing and caching |
| **p95 latency** | ≤ 800ms | Accounts for cache misses and hot key scenarios |
| **Concurrent users** | 50+ | Realistic load for IoT monitoring dashboards |
| **Dataset scale** | 50M rows | Proves architecture at production scale |

## Key Design Decisions

| Decision | Why |
|----------|-----|
| **PostgreSQL 16+** | BRIN indexes for time-series, JSONB flexibility, proven at scale |
| **Go + chi router** | Compiled performance, goroutines for concurrency, simple deployment |
| **pgx driver** | Best Go PostgreSQL driver with connection pooling and binary protocol |
| **Redis caching** | 30s TTL balances freshness vs performance; 5-15ms cache hits |
| **k6 for testing** | Modern JavaScript-based testing with Docker integration, excellent metrics |
| **Automated migrations** | Schema versioning with tracking and safe application |

## Documentation Structure

```
/docs
├── README.md                    ← This file (Project overview)
├── architecture.md              ← Schema design, indexing strategy, caching
├── stack.md                     ← Full tech stack with justifications
├── api-spec.md                  ← Complete API contract
├── testing.md                   ← Test plan, scenarios, pass/fail criteria
├── ui-consideration.md          ← Portfolio value assessment
├── implementation/              ← Step-by-step implementation guide
│   ├── README.md                ← Implementation overview
│   ├── plan.md                  ← Master phased plan
│   ├── dev-environment.md       ← Tool installation
│   ├── database-setup.md        ← Database provisioning
│   ├── data-generation.md       ← Generate 50M test dataset
│   ├── api-development.md       ← Build the Go API
│   ├── cache-setup.md           ← Redis caching integration
│   ├── load-testing-setup.md    ← k6 load testing execution
│   └── validation-checklist.md  ← End-to-end verification
└── future-enhancements/         ← Features designed but not yet implemented
    ├── README.md                ← Future enhancements overview
    ├── 01-nginx-reverse-proxy.md ← Nginx implementation guide
    ├── 02-metadata-jsonb-column.md ← Adding metadata column
    ├── 03-partitioning-strategy.md ← Table partitioning for 100M+ scale
    └── 04-schema-type-corrections.md ← Schema type alignment options
```

## Migration System

The project includes an **automated migration system** that tracks and applies database schema changes safely.

### Features

- **Version tracking**: Each migration is numbered (001, 002, 003, etc.)
- **Automatic application**: Only runs pending migrations (safe to re-run)
- **Checksums**: Validates migration integrity before applying
- **Backfill support**: Can backfill migrations for existing databases
- **Incremental materialized views**: Optimized refresh for large datasets

### Usage

```bash
# Run all pending migrations
./scripts/run_migrations.sh

# Run specific migration
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/006_covering_index.sql

# Check migration status
./scripts/run_migrations.sh --status
```

### Migration Files

| Migration | Purpose | When to Run |
|-----------|---------|-------------|
| 001_init_schema.sql | Base table creation | First setup |
| 002_advanced_indexes.sql | BRIN and composite indexes | After data generation |
| 003_seed_data.sql | Initial seed data | First setup |
| 004_materialized_views.sql | Dashboard performance views | After data generation |
| 005_incremental_mv_refresh.sql | Optimized MV refresh functions | After MV creation |
| 006_covering_index.sql | Index-only scan optimization | Production deployment |

## Project Scope

### What This Project Is

- A demonstration of production-grade architecture for high-volume queries
- Comprehensive documentation of design decisions and tradeoffs
- A portfolio piece showcasing backend engineering and performance optimization
- A reference implementation for time-series query patterns

### What This Project Is NOT

- A raw benchmark measuring isolated database performance
- A complete production deployment (no monitoring, observability, etc.)
- A frontend/UI project (see [ui-consideration.md](ui-consideration.md) for rationale)

## Generalizability

This architecture is **schema-agnostic** — the core design principles apply regardless of specific table structure:

1. The composite index pattern `(identifier, timestamp DESC)` works for any repeating identifier
2. BRIN indexing applies to any append-only time-series data
3. The API-first approach decouples schema from consumers
4. Caching strategy is key-based and independent of data structure

For a deeper discussion, see the [Generalizability Argument](architecture.md#generalizability) in [architecture.md](architecture.md).

## Hardware Considerations

The 50M row target is achievable on modest hardware:

- **Minimum:** 8GB RAM, SSD storage (no HDD)
- **Recommended:** 16GB RAM, NVMe SSD
- **Optional:** Read replica for further scaling

Performance will ultimately be bounded by:
- Network latency (~50-100ms minimum)
- Disk I/O for cold queries
- Connection pool limits under extreme load

These limitations are acknowledged honestly in the documentation.

## Next Steps

1. Read [architecture.md](architecture.md) to understand the database design and indexing strategy
2. Review [stack.md](stack.md) for comprehensive technology justifications
3. Examine [api-spec.md](api-spec.md) for the complete API contract
4. Study [testing.md](testing.md) to understand the validation methodology
