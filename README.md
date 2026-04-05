# Higth - High-Performance Time-Series Query System

**A production-grade demonstration that PostgreSQL can query 50M+ time-series rows in ≤500ms through proper schema design, caching, and architectural patterns.**

---

## Why Higth Exists

Traditional database setups choke on time-series data at scale. A simple "get last 100 readings for device X" query can take 5-10 seconds on 50 million rows.

**Higth proves this can be solved** through:
1. **Schema optimization**: BRIN indexes (100× smaller than B-tree), covering indexes, materialized views
2. **Smart caching**: Redis LRU with 30s TTL provides 100× cache hit speedup
3. **Clean architecture**: Handler → Service → Repository separation (testable, maintainable)

**Result**: ≤500ms median query time on 50M+ rows.

## Why Sensor Data?

IoT sensor telemetry is the **ideal test case** because it's:
- **High volume**: Real systems process 14M+ records daily (10K sensors × 1 reading/minute)
- **Hot keys**: 20% of devices get 80% of queries (realistic cache testing)
- **Time-series**: Append-only data (BRIN index benefits)
- **Real-world**: Smart cities, industrial IoT, agriculture monitoring
- **Universal pattern**: The `(entity_id, timestamp DESC)` structure applies to:
  - User activity logs (`user_id`, `created_at`)
  - Transaction histories (`account_id`, `transaction_time`)
  - Error tracking (`session_id`, `error_timestamp`)
  - Audit trails (`resource_id`, `audit_timestamp`)

## What Gets Proven

| Optimization | Impact | Generalizable |
|-------------|--------|---------------|
| **BRIN Indexes** | 99% smaller than B-tree, perfect for time-series | Any append-only time-series data |
| **Covering Indexes** | Index-only scans eliminate table lookups | Any frequently-accessed columns |
| **Materialized Views** | 100-200× faster dashboard queries | Any aggregation/reporting workload |
| **Redis LRU Cache** | 100× speedup on cache hits | Any read-heavy workload with hot keys |
| **Connection Pooling** | 10K+ concurrent requests | Any high-traffic API |

## Production Readiness

**This is production-quality code**. Converting to a real project requires only:
1. Change `sensor_readings` table structure → your domain entities
2. Change API endpoints → your domain operations
3. Everything else works as-is

**What's included:**
- ✅ REST API with proper error handling
- ✅ Optimized database schema (BRIN, MVs, covering indexes)
- ✅ Redis caching layer (LRU, TTL, cache-aside pattern)
- ✅ Health checks and Prometheus metrics
- ✅ Comprehensive testing (k6 scenarios)
- ✅ Migration system with tracking
- ✅ Docker orchestration

**What's NOT included (add if needed):**
- ❌ Authentication/authorization (domain-specific)
- ❌ Horizontal scaling (connection pool sufficient for most workloads)
- ❌ Monitoring dashboard (use Grafana/Loki instead)

**Scaling considerations:**
- System is **CPU-bound, not I/O-bound** (BRIN indexes minimize disk I/O)
- Add read replicas for query scaling
- Tune connection pool for write scaling
- SSD/NVME required for large datasets

---

## Table of Contents

- [Repository Structure](#repository-structure)
- [What is Higth?](#what-is-higth)
- [Quick Start (15 Minutes)](#quick-start-15-minutes)
- [Detailed Setup Guide](#detailed-setup-guide)
- [Understanding the System](#understanding-the-system)
- [Running Experiments](#running-experiments)
- [Benchmark Testing](#benchmark-testing)
- [Materialized Views](#materialized-views)
- [Understanding Your Results](#understanding-your-results)
- [Generating More Data](#generating-more-data)
- [Reference Guide](#reference-guide)
- [Troubleshooting](#troubleshooting)

---

## Repository Structure

```
highth/
├── cmd/                        # Application entry points
│   └── api/
│       └── main.go             # API server entry point
│
├── internal/                   # Private application code (not importable by other apps)
│   ├── cache/                  # Caching layer
│   │   └── redis_cache.go      # Redis cache implementation (LRU, TTL)
│   │
│   ├── config/                 # Configuration management
│   │   └── config.go           # Environment variable loading & app config
│   │
│   ├── handler/                # HTTP request handlers (controllers)
│   │   ├── health_handler.go   # Health check endpoint
│   │   └── sensor_handler.go   # Sensor readings & stats endpoints
│   │
│   ├── middleware/             # HTTP middleware
│   │   ├── compression.go      # Gzip response compression
│   │   ├── metrics.go          # Prometheus metrics collection
│   │   └── prometheus.go       # /metrics endpoint handler
│   │
│   ├── model/                  # Data models (structs)
│   │   ├── health.go           # Health check response model
│   │   └── sensor.go           # Sensor reading & stats models
│   │
│   ├── repository/             # Database access layer
│   │   └── sensor_repo.go      # PostgreSQL queries & operations
│   │
│   └── service/                # Business logic layer
│       └── sensor_service.go   # Sensor data processing & caching logic
│
├── scripts/                    # Utility scripts
│   ├── schema/                 # Database schema & migrations
│   │   ├── migrations/         # SQL migration files
│   │   │   ├── 001_init_schema.sql         # Base table + primary key + 2 initial indexes
│   │   │   ├── 002_advanced_indexes.sql    # BRIN index + composite index
│   │   │   ├── 004_materialized_views.sql  # Hourly/daily/global stats MVs
│   │   │   ├── 005_incremental_mv_refresh.sql # Incremental MV refresh functions
│   │   │   └── 006_covering_index.sql      # Covering index with INCLUDE clause
│   │   └── (schema.sql)        # Initial schema (if needed)
│   │
│   ├── docker-compose.yml      # Docker services definition
│   ├── generate_data.go        # Go data generator (slower)
│   ├── generate_data_fast.py   # Python data generator (fast, uses COPY)
│   ├── generate_data_bulk.py   # Bulk generator with index optimization (drop/recreate indexes)
│   ├── verify_indexes.py       # Index verification tool (standalone or importable)
│   ├── refresh_materialized_views.sh  # Materialized view refresh automation
│   └── run_migrations.sh       # Automated migration runner
│
├── tests/                      # k6 Benchmark tests
│   ├── scenarios/              # Test scenario files
│   │   ├── 01-hot-device-pattern.js       # Zipf distribution test
│   │   ├── 02-time-range-queries.js       # Time-range query test
│   │   ├── 03-mixed-workload.js           # Real API usage test
│   │   └── 04-cache-performance.js        # Cache effectiveness test
│   ├── lib/                    # Test libraries
│   │   ├── config.js           # Test configuration
│   │   ├── endpoints.js        # API endpoint wrappers
│   │   └── helpers.js          # Utility functions
│   ├── k6.config.js            # k6 configuration
│   └── run-benchmarks.sh       # Main test runner script
│
├── docs/                       # Documentation
│   ├── implementation/         # Implementation guides
│   │   └── (detailed guides for specific optimizations)
│   ├── api-spec.md             # REST API specification
│   ├── architecture.md         # System architecture design
│   ├── stack.md                # Technology stack details
│   ├── testing.md              # Testing methodology
│   ├── ui-consideration.md     # UI/UX considerations
│   └── README.md               # Additional project documentation
│
├── data/                       # Persistent data storage (Docker volumes)
│   ├── postgres/               # PostgreSQL database files
│   └── redis/                  # Redis persistence file
│
├── test-results/               # Load test results
│   └── 20260317_110226/        # Test run outputs
│
├── bin/                        # Compiled binaries
│   ├── api                     # Compiled API server binary
│   └── generate_data           # Compiled data generator binary
│
├── docker-compose.yml          # Docker services (postgres, redis, api)
├── Dockerfile                  # API container build definition
├── .env                        # Environment variables (not in git)
├── .env.example                # Environment variables template
├── go.mod                      # Go module definition
├── go.sum                      # Go dependencies lock file
├── .gitignore                  # Git ignore patterns
└── README.md                   # This file
```

### Folder Purposes

| Folder | Purpose | Description |
|--------|---------|-------------|
| **`cmd/`** | Entry Points | Contains the main application entry points. `cmd/api/main.go` is the API server's entry point that initializes all components (config, database, cache, handlers) and starts the HTTP server. |
| **`internal/`** | Private Code | Application code that should not be imported by other projects. Follows Go's internal package convention. Contains all the business logic, handlers, and data access layers. |
| **`scripts/`** | Utilities | Standalone scripts for data generation, testing, database operations, and Docker orchestration. Can be run independently of the main application. |
| **`docs/`** | Documentation | Comprehensive documentation including API specs, architecture design, implementation guides, and technical decisions. |
| **`data/`** | Data Storage | Docker volume mount points. PostgreSQL database files and Redis persistence files are stored here. Data persists across container restarts. |
| **`test-results/`** | Test Outputs | Benchmark test results generated by k6. Each test run creates timestamped files with detailed metrics. |
| **`bin/`** | Compiled Binaries | Output directory for compiled Go executables. Contains `api` (server) and `generate_data` (generator). |

### Key Files Explained

| File | Purpose | Description |
|------|---------|-------------|
| **`docker-compose.yml`** | Container Orchestration | Defines 3 services: `api` (built from Dockerfile), `postgres` (PostgreSQL 16), and `redis` (Redis 7). Configures networking, volumes, and health checks. |
| **`Dockerfile`** | Container Build | Multi-stage build for the API. Stage 1: Build Go binary. Stage 2: Minimal Alpine image with only the binary. Results in ~20MB image. |
| **`.env`** | Environment Config | Contains sensitive configuration (database passwords, Redis settings). NOT in git. Use `.env.example` as template. |
| **`go.mod`** | Go Dependencies | Lists all Go module dependencies. Defines the module path and required Go version (1.21+). |
| **`generate_data_fast.py`** | Data Generator | Fast data generator using PostgreSQL COPY command. Generates 100,000+ rows/sec. Creates realistic IoT sensor data with Zipf distribution. |
| **`generate_data_bulk.py`** | Bulk Data Generator | Optimized generator that drops indexes before loading, recreates them after. Much faster for large datasets on existing tables. |
| **`verify_indexes.py`** | Index Verification | Verifies all `sensor_readings` indexes match migration definitions. Standalone tool or importable module for automated verification. |

### Application Architecture Flow

```
Request Flow:
─────────────
1. HTTP Request → [handler]
2. Handler → [service] (business logic)
3. Service → [cache] (check Redis)
4. Cache miss → [repository] (database query)
5. Repository → [database]
6. Response flows back through cache → service → handler → HTTP Response

File Responsibilities:
──────────────────────
├── handler/    → Parse HTTP requests, call services, format responses
├── service/    → Business logic, caching decisions, coordinate repository calls
├── repository/ → SQL queries, database connection management
├── model/      → Data structures (request/response DTOs, database models)
├── middleware/ → Cross-cutting concerns (metrics, compression, logging)
├── config/     → Configuration loading from environment variables
└── cache/      → Cache interface implementation (Redis)
```

---

## What is Higth?

**Higth** is a production-grade demonstration of high-performance time-series query optimization. It simulates thousands of IoT devices continuously sending sensor readings that need to be queried and analyzed in real-time.

This is a **portfolio-quality demonstration** that proves PostgreSQL can handle time-series workloads at scale through proper architecture—not by switching to specialized databases.

### Key Features

| Feature | Performance Impact | What It Enables |
|---------|-------------------|-----------------|
| **BRIN Indexes** | 99% smaller than B-tree; perfect for append-only time-series data | Efficient time-range queries on 50M+ rows without massive storage overhead |
| **Materialized Views** | 100-200× faster dashboard queries (50-200ms → 1-5ms) | Real-time aggregations across millions of rows for dashboards |
| **Redis LRU Cache** | 100× speedup on cache hits (500ms → 5ms) | Hot device queries (20% of devices get 80% of traffic) |
| **Covering Indexes** (Migration 006) | Index-only scans eliminate heap access (2-5× faster) | Fast "last N readings" queries: 50-200ms → 5-50ms |
| **Connection Pooling** | 10K+ concurrent requests | High concurrency without overwhelming the database |
| **Incremental MV Refresh** | Refresh only last N days (7 for hourly, 30 for daily) | Efficient materialized view updates without full rebuilds |

### Tech Stack Rationale

| Component | Technology | Why This Choice |
|-----------|-----------|-----------------|
| **API** | Go 1.21+ | Goroutines handle 10K+ concurrent connections; pgx driver uses binary protocol for faster queries; 20MB Docker image |
| **Database** | PostgreSQL 16 | BRIN indexes (perfect for time-series), materialized views (pre-computed aggregations), declarative partitioning-ready |
| **Cache** | Redis 7 | LRU eviction keeps hot keys cached; 30s TTL balances freshness vs performance; cache-aside pattern is resilient to failures |
| **Pool** | pgx built-in | Connection pooling directly in the Go application; 50 max connections with 10 min idle timeout; no separate PgBouncer needed |
| **Metrics** | Prometheus | Built-in `/metrics` endpoint; tracks request latency, cache hit rate, database pool stats; industry standard |
| **Testing** | k6 | Modern JavaScript-based load testing; built-in percentiles (p50, p95, p99); Docker integration for consistent testing |

---

## Quick Start (15 Minutes)

### Prerequisites

**Hardware:**
- CPU: 2+ cores recommended
- RAM: 4+ GB (8 GB recommended for large datasets)
- Disk: 10+ GB free (50+ GB for 50M row experiments)

**Software:**
- Docker & Docker Compose
- Go 1.21+ (for running API locally)
- Python 3.10+ (for data generation)
- curl (for testing API)

### Installation

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Start all services (PostgreSQL, Redis, API)
docker-compose up -d

# 3. Wait for services to be healthy (30 seconds)
docker-compose ps

# 4. Run database migrations (automated) ⭐
./scripts/run_migrations.sh

# 5. Generate test data (1,000 rows for quick start)
./scripts/generate_data_fast.py 1000 --devices 10 --days 1

# 6. Re-run migrations to create performance indexes and materialized views
./scripts/run_migrations.sh

# 7. Test the API
curl http://localhost:8080/health
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=10"
```

**⭐ IMPORTANT - Database Migrations:**

The `./scripts/run_migrations.sh` command is the **recommended way** to manage all database changes:

- **First run** (step 4): Creates base tables and initial schema
- **After data generation** (step 6): Creates performance indexes and materialized views
- **For any new deployment**: Run once to apply all pending migrations
- **After schema changes**: Run to apply new migrations automatically

The migration runner automatically:
- ✅ Tracks which migrations have been applied
- ✅ Only runs pending migrations (safe to re-run)
- ✅ Shows migration status and execution time
- ✅ Validates migrations before applying
- ✅ Backfills existing migrations if needed

See [Database Migrations](#database-migrations) for detailed information.

The API will start on `http://localhost:8080`

### Verify Setup

```bash
# Check health endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","timestamp":"2026-03-19T09:06:26Z","checks":{"cache":{"status":"healthy","latency_ms":1},"database":{"status":"healthy","latency_ms":2}}}

# Test sensor readings endpoint (requires device_id)
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=5"

# View all services
docker-compose ps

# Expected output: 3 services running (api, postgres, redis)
```

---


## Database Migrations

The Higth project uses an automated migration system to track and apply database schema changes. This ensures your database stays in sync with the application code.

### Running Migrations

**One command to run all pending migrations:**

```bash
./scripts/run_migrations.sh
```

**Available options:**

```bash
./scripts/run_migrations.sh           # Run all pending migrations
./scripts/run_migrations.sh --dry-run # Preview what would be applied
./scripts/run_migrations.sh --verbose # Show detailed output
./scripts/run_migrations.sh --force   # Force re-run of migrations (use with caution)
```

**What the migration runner does:**

1. ✅ Checks database connection
2. ✅ Creates `schema_migrations` tracking table (if needed)
3. ✅ Detects existing migrations (001, 002, 004, 005) and backfills them
4. ✅ Runs unapplied migrations in order
5. ✅ Tracks applied migrations with checksums
6. ✅ Provides clear, colored output

### Migration Reference

| Migration | Version | Description | Status |
|-----------|---------|-------------|--------|
| Base Schema | 001 | Creates `sensor_readings` table with basic indexes | ✅ Applied |
| Advanced Indexes | 002 | Creates BRIN and composite indexes for performance | ✅ Applied |
| Materialized Views | 004 | Creates `mv_device_hourly_stats`, `mv_device_daily_stats`, `mv_global_stats` | ✅ Applied |
| Incremental MV Refresh | 005 | Creates `refresh_*_incremental()` functions for fast MV refresh | ✅ Applied |

### Creating New Migrations

**Naming convention:**

```
NNN_description.sql
```

- `NNN` - Zero-padded 3-digit number (006, 007, etc.)
- `description` - Snake_case short description

**Examples:**
- `006_add_api_rate_limits.sql`
- `007_create_user_accounts.sql`
- `008_add_audit_log_table.sql`

**Steps to create a migration:**

1. **Create the migration file:**

```bash
# In scripts/schema/migrations/
touch 006_your_migration_name.sql
```

2. **Write idempotent SQL (can run multiple times safely):**

```sql
-- Migration 006: Add API rate limits
--
-- Adds rate limiting functionality to the API

BEGIN;

-- Create table with IF NOT EXISTS
CREATE TABLE IF NOT EXISTS rate_limits (
    api_key VARCHAR(255) PRIMARY KEY,
    requests_per_minute INT NOT NULL DEFAULT 60,
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes with IF NOT EXISTS
CREATE INDEX IF NOT EXISTS idx_rate_limits_api_key ON rate_limits(api_key);

COMMENT ON TABLE rate_limits IS 'API rate limiting configuration';

COMMIT;
```

3. **Run the migration:**

```bash
./scripts/run_migrations.sh
```

**Best practices for migrations:**

- ✅ Use `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`
- ✅ Wrap changes in transactions (`BEGIN`/`COMMIT`)
- ✅ Add comments explaining the migration purpose
- ✅ Test migrations on a copy of production data first
- ✅ Never modify or delete migration files that have been applied
- ✅ Keep migrations focused (one logical change per file)

### Migration Tracking

The `schema_migrations` table tracks applied migrations:

```sql
SELECT version, name, applied_at, execution_time_ms
FROM schema_migrations
ORDER BY version;
```

**Columns:**
- `version` - Migration number (001, 002, etc.)
- `name` - Migration name (from filename)
- `applied_at` - When the migration was applied
- `execution_time_ms` - How long the migration took to run
- `checksum` - SHA256 checksum of the migration file

### Troubleshooting

**Migration runner fails with "Database connection failed":**

```bash
# Check if PostgreSQL is running
docker ps | grep highth-postgres

# Restart if needed
docker-compose restart postgres
```

**Migration runner says "Already applied" but migration is missing:**

```bash
# Force re-run (use with caution!)
./scripts/run_migrations.sh --force
```

**Need to re-run a specific migration:**

```bash
# 1. Check which migrations are applied
docker exec highth-postgres psql -U sensor_user -d sensor_db \
  -c "SELECT version, name FROM schema_migrations ORDER BY version;"

# 2. Remove the migration record
docker exec highth-postgres psql -U sensor_user -d sensor_db \
  -c "DELETE FROM schema_migrations WHERE version = '006';"

# 3. Re-run migrations
./scripts/run_migrations.sh
```

**View all migrations in database:**

```bash
# View schema_migrations table
docker exec highth-postgres psql -U sensor_user -d sensor_db \
  -c "SELECT * FROM schema_migrations ORDER BY version;"

# View all tables
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dt"

# View all materialized views
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dmv"
```

## Detailed Setup Guide

### Option A: Docker Setup (Recommended)

**Step 1: Configure Environment**

```bash
# Copy environment template
cp .env.example .env

# Edit if needed (defaults work for most cases)
# .env includes:
# - POSTGRES_PASSWORD=sensor_password (change for production)
# - POSTGRES_PORT=5434 (avoids conflict with local PostgreSQL)
# - REDIS_PORT=6380 (avoids conflict with local Redis)
# - API_PORT=8080
```

**Step 2: Start Services**

```bash
# Start PostgreSQL, Redis, and API
docker-compose up -d

# Check all services are running
docker-compose ps

# View logs if needed
docker-compose logs -f api
docker-compose logs -f postgres
```

#### PostgreSQL Configuration

The `docker-compose.yml` includes 14 performance-tuned PostgreSQL parameters optimized for high-throughput time-series workloads on an 8GB RAM system.

**Parameters include:**
- **Memory tuning**: `shared_buffers=2GB`, `effective_cache_size=6GB`, `work_mem=16MB`, `maintenance_work_mem=1GB`
- **WAL optimization**: `wal_buffers=16MB`, `checkpoint_completion_target=0.9`
- **SSD optimization**: `random_page_cost=1.1`, `effective_io_concurrency=200`
- **Parallelism**: `max_worker_processes=8`, `max_parallel_workers_per_gather=2`, `max_parallel_workers=8`
- **Background writer**: `bgwriter_delay=200ms`, `bgwriter_lru_maxpages=100`
- **Connections**: `max_connections=200`

> **For detailed explanation of each parameter and why these values were chosen**, see [docs/high-throughput-guide/01-postgresql-setup.md](docs/high-throughput-guide/01-postgresql-setup.md#parameter-explanations).
>
> **Configuring for your hardware?** The default configuration is tuned for 8GB RAM + SSD + 8 cores. If your machine has different specs, see [Configuration Adjustment Guide](docs/high-throughput-guide/05-configuration-adjustment-guide.md) for hardware presets and parameter adjustment instructions.

**Step 3: Initialize Database**

```bash
# Run migrations to create all database schema and optimizations
./scripts/run_migrations.sh

# Verify tables created
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dt"

# Verify materialized views
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dmv"
```

### Option B: Local Setup (Advanced)

**Requirements:**
- PostgreSQL 15+ installed locally
- Redis 7+ installed locally
- Go 1.21+ installed

**Steps:**

```bash
# 1. Start PostgreSQL and Redis locally
# (Use your preferred method: brew, apt, systemctl, etc.)

# 2. Run migrations to create database schema and optimizations
#    (Database will be created automatically by PostgreSQL)
./scripts/run_migrations.sh

# 3. Configure environment variables
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export REDIS_HOST=localhost
export REDIS_PORT=6379

# 4. Run API
go run cmd/api/main.go
```

---

## Understanding the System
- [Database Migrations](#database-migrations)

### Architecture Overview

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP
       ▼
┌─────────────────────────────────────────────────┐
│              API Layer (Go)                      │
│  ┌─────────┐  ┌──────────┐  ┌──────────────┐  │
│  │ Handlers│  │Metrics MW│  │Compression MW│  │
│  └────┬────┘  └──────────┘  └──────────────┘  │
└───────┼──────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────┐
│              Service Layer                       │
│         Business Logic & Caching                 │
└───────┼──────────────────────────────────────────┘
        │
        ├─────────────┬─────────────┐
        ▼             ▼             ▼
┌──────────────┐ ┌──────────┐ ┌─────────────┐
│ Redis Cache  │ │PgBouncer │ │ PostgreSQL  │
│  (30s TTL)   │ │   Pool   │ │  Database   │
└──────────────┘ └──────────┘ └─────────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
              ┌─────────┐  ┌──────────┐  ┌──────────┐
              │  Base   │  │ Material │  │ Indexes  │
              │  Table  │  │  Views   │  │ (BRIN,  │
              │         │  │          │  │Covering) │
              └─────────┘  └──────────┘  └──────────┘
```

### API Endpoints

#### 1. Health Check
```bash
GET /health
```

**Response (Healthy):**
```json
{
  "status": "healthy",
  "timestamp": "2026-03-19T09:06:26Z",
  "checks": {
    "cache": {
      "status": "healthy",
      "latency_ms": 1
    },
    "database": {
      "status": "healthy",
      "latency_ms": 2
    }
  }
}
```

**Response (Degraded - cache unhealthy):**
```json
{
  "status": "degraded",
  "timestamp": "2026-03-19T09:06:26Z",
  "checks": {
    "cache": {
      "status": "unhealthy",
      "error": "connection refused",
      "latency_ms": 0
    },
    "database": {
      "status": "healthy",
      "latency_ms": 2
    }
  }
}
```

**HTTP Status Codes:**
- `200 OK` - All dependencies healthy
- `503 Service Unavailable` - One or more dependencies unhealthy

#### 2. Query Sensor Readings
```bash
GET /api/v1/sensor-readings?device_id={id}&limit={n}&reading_type={type}
```

**Parameters:**
- `device_id` (**required**): Device identifier (e.g., `sensor-000001`) - returns 400 if missing
- `limit` (optional): Number of results (1-500, default: 10)
- `reading_type` (optional): Filter by type (alphanumeric, 1-30 characters)

**Example:**
```bash
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=10&reading_type=temperature"
```

**Response:**
```json
{
  "data": [
    {
      "id": "12345678",
      "device_id": "sensor-000001",
      "timestamp": "2026-03-19T08:19:17Z",
      "reading_type": "temperature",
      "value": 34.63,
      "unit": "°C"
    }
  ],
  "meta": {
    "count": 1,
    "limit": 10,
    "device_id": "sensor-000001",
    "reading_type": "temperature"
  }
}
```

**Error Response (device not found):**
```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "device not found: no readings found for device_id: nonexistent",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "abc123"
  }
}
```

#### 3. Get Statistics
```bash
GET /api/v1/stats?device_id={id}&reading_type={type}&period={hour|day}
```

**Parameters:**
- `device_id` (optional): Specific device or global stats
- `reading_type` (optional): Filter by type
- `period` (optional): `hour` or `day` (default: `hour`)

**Example:**
```bash
curl "http://localhost:8080/api/v1/stats?reading_type=temperature&period=day"
```

**Response:**
```json
{
  "reading_type": "temperature",
  "statistics": {
    "count": 1500000,
    "avg": 22.45,
    "min": 15.2,
    "max": 35.8,
    "median": 22.3
  }
}
```

#### 4. Get Sensor Reading by ID (PK Lookup Mode)
```bash
GET /api/v1/sensor-readings?id={id}
```

**Parameters:**
- `id` (**required**, query parameter): Primary key ID of the sensor reading (positive integer)

**Mutual Exclusivity:** You must provide **exactly one** of `id` or `device_id`. Providing both returns 400 `INVALID_PARAMETER`.

**Example:**
```bash
curl "http://localhost:8080/api/v1/sensor-readings?id=12345678"
```

**Response:**
```json
{
  "data": {
    "id": "12345678",
    "device_id": "sensor-000001",
    "timestamp": "2026-03-19T08:19:17Z",
    "reading_type": "temperature",
    "value": 34.63,
    "unit": "°C"
  },
  "meta": {
    "id": "12345678"
  }
}
```

**Error Response (not found):**
```json
{
  "error": {
    "code": "READING_NOT_FOUND",
    "message": "reading not found: no sensor reading exists with id 99999999",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "abc123"
  }
}
```

**Error Response (mutual exclusivity violation):**
```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "id and device_id are mutually exclusive",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "abc123",
    "details": {
      "parameter": "id,device_id",
      "provided": {"id": "123", "device_id": "sensor-001"},
      "constraints": {"rule": "provide exactly one"}
    }
  }
}
```

**Cache Behavior:** Results are cached in Redis for 30 seconds. Check the `X-Cache-Status` header (`HIT` or `MISS`).

#### 5. Prometheus Metrics
```bash
GET /metrics
```
Returns Prometheus-format metrics for monitoring.

---

### Error Responses

All error responses follow this format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "timestamp": "2026-03-19T09:01:47Z",
    "request_id": "unique-request-id"
  }
}
```

**Common Error Codes:**

| Error Code | HTTP Status | Example |
|------------|-------------|---------|
| `INVALID_PARAMETER` | 400 | Missing required parameter or invalid value |
| `DEVICE_NOT_FOUND` | 404 | No readings found for the specified device |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

**Validation Error Example (with details):**
```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "limit must be between 1 and 500",
    "timestamp": "2026-03-19T09:01:47Z",
    "request_id": "692f0f0b8b63/7EGnsijONu-000003",
    "details": {
      "parameter": "limit",
      "provided": "600",
      "constraints": {
        "min": 1,
        "max": 500
      }
    }
  }
}
```

---

### Response Headers

All API responses include the following headers:

| Header | Description | Example |
|--------|-------------|---------|
| `Content-Type` | Response format | `application/json` |
| `X-Request-ID` | Unique request identifier for debugging | `692f0f0b8b63/7EGnsijONu-000003` |
| `X-Response-Time` | Server processing time in milliseconds | `3` |
| `Cache-Control` | Cache directives for client caching | `public, max-age=30` |

**Example:**
```bash
curl -i "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000000&limit=2"

# Response headers:
# HTTP/1.1 200 OK
# Cache-Control: public, max-age=30
# Content-Type: application/json
# X-Request-Id: 692f0f0b8b63/7EGnsijONu-000006
# X-Response-Time: 3
```

---

### Validation Rules

| Parameter | Type | Required | Default | Valid Values | Error on Invalid |
|-----------|------|----------|---------|--------------|------------------|
| `device_id` | string | **Yes** | - | Alphanumeric with hyphens/underscores, 1-50 chars | 400 INVALID_PARAMETER |
| `limit` | integer | No | 10 | 1-500 | 400 INVALID_PARAMETER |
| `reading_type` | string | No | - | Alphanumeric, 1-30 characters | 400 INVALID_PARAMETER |

---

### Database Schema

**Main Table: `sensor_readings`**

| Column | Type | Description |
|--------|------|-------------|
| id | BIGINT | Primary key |
| device_id | VARCHAR(50) | Device identifier (e.g., `sensor-000001`) |
| timestamp | TIMESTAMPTZ | Reading timestamp (UTC) |
| reading_type | VARCHAR(20) | Type: `temperature`, `humidity`, `pressure` |
| value | DECIMAL(10,2) | Sensor value |
| unit | VARCHAR(20) | Unit of measurement |

**Indexes:**
- Primary key B-tree index (on `id`)
- BRIN index on `timestamp` (for time-series queries, 99% smaller)
- Covering index on `(device_id, timestamp, ...)` for hot devices
- Composite index on `(device_id, timestamp)`

**Materialized Views:**
- **`mv_device_hourly_stats`**: Pre-computed hourly aggregations (100x faster)
- **`mv_device_daily_stats`**: Pre-computed daily aggregations with percentiles (200x faster)
- **`mv_global_stats`**: Global statistics across all devices (instant response)

---

## Running Experiments

### Experiment 1: Query Single Device

**Objective:** Fetch recent readings from a specific device

```bash
# Query for last 100 readings from sensor-000001
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100"

# Measure response time
time curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100" > /dev/null
```

**Expected Results:**
- First query (cold cache): 50-200 ms
- Subsequent queries (warm cache): 1-5 ms

### Experiment 2: Cold vs Warm Cache

**Objective:** Measure cache effectiveness

```bash
# Flush Redis cache
docker exec highth-redis redis-cli FLUSHALL

# Measure cold cache query
time curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100"

# Measure warm cache queries (run 5 times)
for i in {1..5}; do
  time curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100"
done
```

**Expected Results:**
- Cold cache: 50-200 ms
- Warm cache: 1-5 ms (40x faster)

### Experiment 3: Concurrent Load Testing

**Objective:** Measure performance under load

```bash
# Run the benchmark tests
./tests/run-benchmarks.sh --scenario mixed

# Results saved to test-results/ with timestamp
```

**Metrics to Watch:**
- p50 latency: Should be < 1 ms with optimizations
- p95 latency: Should be < 5 ms
- p99 latency: Should be < 10 ms
- Error rate: Should be 0%

### Experiment 4: Materialized Views vs Raw Queries

**Objective:** Compare MV performance to base table

```bash
# Connect to database
docker exec -it highth-postgres psql -U sensor_user -d sensor_db

# Enable timing
\timing on

# Base table query (slow)
SELECT device_id, date_trunc('hour', timestamp) as hour, reading_type, avg(value)
FROM sensor_readings
WHERE device_id = 'sensor-000001' AND timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY device_id, date_trunc('hour', timestamp), reading_type;

# Materialized view query (fast!)
SELECT * FROM mv_device_hourly_stats
WHERE device_id = 'sensor-000001' AND hour >= NOW() - INTERVAL '24 hours'
ORDER BY hour DESC;
```

**Expected Results:**
- Base table: 50-200 ms
- Materialized view: 1-5 ms (100x faster)

---

## Benchmark Testing

The Higth project includes a **comprehensive benchmark testing suite** powered by [k6](https://k6.io/), a modern load testing tool designed for developer-friendly performance testing.

### Why Benchmark Testing?

This benchmark system validates that the API + database can handle:

- **High data volume**: 83M+ rows in the database
- **High traffic**: Concurrent requests under sustained load
- **Low latency**: Average/median response time < 500ms

### Quick Start

**One command to run all benchmarks:**

```bash
./tests/run-benchmarks.sh
```

**Run specific benchmark scenarios:**

```bash
./tests/run-benchmarks.sh --scenario hot          # Hot device pattern
./tests/run-benchmarks.sh --scenario time-range   # Time-range queries
./tests/run-benchmarks.sh --scenario mixed        # Mixed workload
./tests/run-benchmarks.sh --scenario cache        # Cache performance
```

**Test remote API (different VM/server):**

```bash
./tests/run-benchmarks.sh --target-url https://api.example.com
```

**Customize load and duration:**

```bash
./tests/run-benchmarks.sh --rps 100 --duration 5m
```

**List all available scenarios:**

```bash
./tests/run-benchmarks.sh --list
```

### Command Reference

| Parameter | Short | Default | Description |
|-----------|-------|---------|-------------|
| `--scenario` | `-s` | all | Run specific scenario (hot, time-range, mixed, cache) |
| `--rps` | `-r` | 50 | Requests per second |
| `--duration` | `-d` | 2m | Test duration (e.g., 30s, 5m, 1h) |
| `--target-url` | `-u` | http://localhost:8080 | API endpoint to test |
| `--list` | `-l` | - | List available scenarios |
| `--skip-setup` | - | false | Skip service health checks |
| `--verbose` | `-v` | - | Enable verbose output |
| `--help` | `-h` | - | Show help message |

**Examples:**

```bash
# Test production API with higher load
./tests/run-benchmarks.sh --target-url https://prod-api.example.com --rps 100 --duration 10m

# Quick smoke test
./tests/run-benchmarks.sh --scenario hot --rps 10 --duration 30s

# Test specific scenario with custom settings
./tests/run-benchmarks.sh -s mixed -r 75 -d 5m -u http://192.168.1.100:8080
```

### Benchmark Scenarios

The test suite includes **6 comprehensive scenarios** that simulate real-world IoT traffic patterns:

| Scenario | Purpose | Duration | Target |
|----------|---------|----------|--------|
| **Hot Device Pattern** | Zipf distribution (20% devices get 80% traffic) | 3 min | p95 < 500ms |
| **Time-Range Queries** | Dashboard-style queries (1h/24h/7d) | 2 min | p95 < 500ms |
| **Mixed Workload** ⭐ | **PRIMARY TEST** - Real API usage mix | 2.5 min | p95 < 400ms |
| **Cache Performance** | Cold → Warm → Hot cache phases | 3 min | Hot: p95 < 100ms |
| **Stats and Aggregation** | Materialized view query performance | 2 min | p95 < 800ms |
| **PK Hot Lookup** | Single-row primary key index scan | 30s | p95 < 100ms |

### Scenario Details

#### 1. Hot Device Pattern

**Tests:** Zipf distribution (realistic IoT traffic)
- 80% of requests go to top 20% of devices
- Validates caching effectiveness under uneven load
- Tests connection pool behavior

**Success Criteria:**
- p50 < 300ms
- p95 < 500ms
- p99 < 800ms
- Error rate < 1%

#### 2. Time-Range Queries

**Tests:** Dashboard-style queries with varying time ranges
- Last hour (frequent, small dataset)
- Last 24 hours (medium dataset)
- Last 7 days (large dataset, tests materialized views)
- Varying limits (10, 50, 100, 500 records)

**Success Criteria:**
- p50 < 200ms
- p95 < 500ms
- p99 < 800ms

#### 3. Mixed Workload ⭐

**Tests:** Real-world API usage patterns
- 10% health checks (lightweight)
- 20% stats queries (moderate, uses materialized views)
- 70% sensor readings (heavy, main workload)
- Ramp-up: 25 → 50 → 100 RPS

**Success Criteria:**
- p50 < 150ms
- p95 < 400ms
- p99 < 600ms

#### 4. Cache Performance

**Tests:** Redis cache effectiveness across three phases:
- Phase 1: Cold cache (all database hits)
- Phase 2: Warm cache (populating, mixed hits/misses)
- Phase 3: Hot cache (high hit rate)

**Success Criteria:**
- Phase 1 p95 < 600ms (cold baseline)
- Phase 2 p95 < 300ms (warming up)
- Phase 3 p95 < 100ms (warm cache, very fast!)
- Cache hit rate > 80% in Phase 3

### Understanding Benchmark Results

**Console Output:**

```
╔════════════════════════════════════════════════════════════════╗
║              Higth IoT Benchmark Suite v2.0                      ║
╚════════════════════════════════════════════════════════════════╝

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 Scenario 1: Hot Device Pattern
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ Scenario 'hot_device_pattern' completed

Results:
  p50:   45ms  ✓ (target: <300ms)
  p95:   234ms  ✓ (target: <500ms)
  p99:   412ms  ✓ (target: <800ms)
  RPS:   52.3
  Errors: 0.0%

[SUCCESS] Scenario 'hot' completed
```

**HTML Reports:**

Benchmark results are saved to `test-results/` with timestamped files:
- JSON summary: `summary_TIMESTAMP.json`
- Individual scenario results: `scenario_name_TIMESTAMP.json`
- HTML reports: Available if [k6-to-html](https://github.com/k6io/html-reporter) is installed

**Install k6-to-html for beautiful reports:**

```bash
npm install -g k6-to-html
```

### Advanced Usage

**Custom requests per second:**

```bash
./tests/run-benchmarks.sh --rps 100
```

**Custom test duration:**

```bash
./tests/run-benchmarks.sh --duration 5m
```

**Test remote API:**

```bash
./tests/run-benchmarks.sh --target-url https://api.example.com
```

**Verbose output:**

```bash
./tests/run-benchmarks.sh --verbose
```

### Performance Targets

**Primary Goal:** Validate the system can handle 83M+ rows with <500ms latency

| Metric | Target | Why |
|--------|--------|-----|
| p50 (median) | < 300ms | 50% of requests should be fast |
| p95 | < 500ms | **Primary target** - 95% of requests |
| p99 | < 800ms | 99% of requests (allow some outliers |
| Error rate | < 1% | System reliability |

**Excellent Performance** (exceeds targets):
- p50 < 100ms ✅
- p95 < 300ms ✅
- Error rate = 0% ✅

**Needs Investigation**:
- p50 > 300ms → Check database indexes and caching
- p95 > 500ms → Check query plans and materialized view refresh
- p99 > 800ms → Check for slow queries or connection pool exhaustion
- Error rate > 1% → Check logs for failures

### Docker-Based Testing

The benchmark system uses Docker for isolated, reproducible testing:

```bash
# Start test environment
docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d k6

# Run tests manually inside container
docker exec -it highth-k6 sh
k6 run /tests/scenarios/01-hot-device-pattern.js
```

### Troubleshooting Benchmarks

**"API is not responding" error:**

```bash
# Check if services are running
docker-compose ps

# Start services if needed
docker-compose up -d

# Verify API health
curl http://localhost:8080/health
```

**"Database connection failed" error:**

```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Restart if needed
docker-compose restart postgres
```

**High latency (> 500ms) in results:**

1. Check database indexes are applied:
   ```bash
   docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\di"
   ```

2. Verify materialized views are refreshed:
   ```bash
   ./scripts/refresh_materialized_views.sh --status
   ```

3. Check Redis cache is working:
   ```bash
   docker exec highth-redis redis-cli INFO stats
   ```

---

## Monitoring Stack

The project includes a Prometheus + Grafana monitoring stack for visualizing live metrics during benchmark runs.

### Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   k6 Load Test  │────▶│  Go API (8080)  │
└─────────────────┘     └────────┬────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐
            │ Prometheus│  │PostgreSQL │  │   Redis   │
            │  (9090)   │  │  (5434)   │  │  (6380)   │
            └─────┬─────┘  └─────┬─────┘  └─────┬─────┘
                  │             │             │
                  └─────────────┼─────────────┘
                                ▼
                         ┌─────────────────┐
                         │    Grafana      │
                         │    (3000)       │
                         └─────────────────┘
```

### Startup

**Important**: The main application stack must be running first.

```bash
# Start main stack (if not already running)
docker compose up -d

# Start monitoring stack
docker compose -f compose.monitoring.yml up -d
```

### Access

| Service | URL | Credentials |
|---------|-----|-------------|
| Grafana | http://localhost:3000 | admin/admin |
| Prometheus | http://localhost:9090 | N/A (read-only) |

### Dashboards

Grafana comes pre-provisioned with 4 dashboards:

| Dashboard | Description |
|-----------|-------------|
| **Experiment Load** | Combined view - API latency, PostgreSQL cache hit ratio, Redis ops/sec |
| **API Overview** | Request rate, latency percentiles (p50/p95/p99), error rate, cache hit rate |
| **PostgreSQL Overview** | Active connections, transactions/sec, buffer cache hit ratio, query duration |
| **Redis Overview** | Connected clients, ops/sec, hit/miss rate, memory usage |

**Recommended**: Open the **"Experiment Load"** dashboard during a benchmark run to see live metrics across all services.

### Shutdown

```bash
# Stop monitoring stack only
docker compose -f compose.monitoring.yml down

# Stop everything
docker compose -f compose.monitoring.yml down
docker compose down
```

### Documentation

See [MONITORING.md](MONITORING.md) for detailed documentation on:
- Dashboard panel descriptions
- Prometheus query examples
- Metrics reference
- Troubleshooting

## Materialized Views

### What Are Materialized Views?

Materialized views are **pre-computed database tables** that store the results of complex queries. Unlike regular views (which are just saved queries), materialized views actually store the data, making queries dramatically faster.

**Why are they critical for Higth?**

- **100-200x faster queries**: Dashboard queries that take 50-200ms on raw tables complete in 1-5ms
- **Enables real-time dashboards**: Can query aggregations across 83M+ rows instantly
- **Reduces database load**: Expensive calculations are done once during refresh, not on every query
- **Scales time-series data**: Efficient aggregation for hourly/daily statistics

### The Three Materialized Views

| View | Purpose | Data Size | Refresh Strategy |
|------|---------|-----------|------------------|
| **mv_device_hourly_stats** | Hourly aggregations per device (last 7 days) | ~168K rows | Incremental - every 15 min |
| **mv_device_daily_stats** | Daily aggregations with percentiles (last 30 days) | ~3K rows | Incremental - daily |
| **mv_global_stats** | Global statistics across all devices | ~3 rows | Full - every 5 min |

**What each view provides:**

1. **mv_device_hourly_stats**: Average/min/max readings per device per hour
   - Use for: Device performance trends, hourly dashboards

2. **mv_device_daily_stats**: Daily statistics with percentiles (p50, p95, p99)
   - Use for: Long-term trends, capacity planning, anomaly detection

3. **mv_global_stats**: Total readings, active devices, data volume
   - Use for: System overview, billing, capacity monitoring

### Refreshing Materialized Views

Materialized views need to be refreshed to include new data. Higth provides an automated script:

#### Manual Refresh

```bash
# Refresh all views
./scripts/refresh_materialized_views.sh all

# Refresh specific view type
./scripts/refresh_materialized_views.sh hourly   # Hourly stats only
./scripts/refresh_materialized_views.sh daily    # Daily stats only
./scripts/refresh_materialized_views.sh global   # Global stats only
```

**What the script does:**
- Shows current view size and row count
- Uses incremental refresh for hourly/daily (only refreshes last N days)
- Uses CONCURRENTLY refresh for global stats (non-blocking)
- Reports refresh time and status

**Example output:**
```
╔════════════════════════════════════════════════════════════════╗
║         Materialized View Refresh Script v2.0                  ║
║         (Incremental Refresh Enabled)                           ║
╚════════════════════════════════════════════════════════════════╝

INFO: Refresh type: all
INFO: Database: sensor_db

INFO: Refreshing hourly statistics...
INFO: Current size: 25 MB
INFO: Current rows: 167,843
INFO: Using incremental refresh (last 7 days)
[SUCCESS] Refreshed device_hourly_stats (2s)

... (similar for other views)

─────────────────────────────────────────
Refresh Summary
─────────────────────────────────────────
Views refreshed: 3
Total time: 5s
Completed at: 2026-03-27 16:00:00

╔════════════════════════════════════════════════════════════════╗
║              Refresh Complete ✓                                 ║
╚════════════════════════════════════════════════════════════════╝
```

#### Automated Refresh (Production)

For production environments, automate refreshes using cron:

```bash
# Edit crontab
crontab -e

# Add these lines:
*/15 * * * * /path/to/highth/scripts/refresh_materialized_views.sh hourly >> /var/log/mv_refresh.log 2>&1
0 2 * * * /path/to/higth/scripts/refresh_materialized_views.sh daily >> /var/log/mv_refresh.log 2>&1
*/5 * * * * /path/to/higth/scripts/refresh_materialized_views.sh global >> /var/log/mv_refresh.log 2>&1
```

**Recommended schedule:**
- **Hourly stats**: Every 15 minutes (keeps last 7 days fresh)
- **Daily stats**: Once daily at 2 AM (off-peak hours)
- **Global stats**: Every 5 minutes (fast refresh, small dataset)

**Why these intervals?**
- Hourly stats need frequent updates for current-day dashboards
- Daily stats can refresh overnight (historical data doesn't change)
- Global stats are tiny, so frequent refreshes are cheap

### Best Practices

1. **Refresh before queries**
   - Run refresh before running dashboard queries for best performance
   - For testing: `./scripts/refresh_materialized_views.sh all && ./tests/run-benchmarks.sh`

2. **Monitor refresh time**
   - Normal refresh: 2-10 seconds
   - If refresh takes >30s, investigate database performance

3. **Check view size**
   - Views should stay relatively small (hourly: <100MB, daily: <50MB)
   - If growing too large, check incremental refresh is working

4. **Use INCREMENTAL refresh when possible**
   - The script uses incremental functions by default (faster, less resource-intensive)
   - Only refreshes the last N days (7 for hourly, 30 for daily)

### Troubleshooting

**Views are slow to query:**
```bash
# Check when views were last refreshed
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "
SELECT schemaname, matviewname, last_refresh
FROM pg_matviews WHERE matviewname LIKE 'mv_%';
"

# Force refresh all views
./scripts/refresh_materialized_views.sh all
```

**Refresh script fails:**
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Check database connection
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "SELECT 1"

# Check view exists
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\dmv"
```

**Views don't contain recent data:**
```bash
# Check refresh schedule (if using cron)
crontab -l | grep refresh_materialized_views

# Manually refresh to bring views up to date
./scripts/refresh_materialized_views.sh all
```

**Incremental refresh not working:**
```bash
# Check if incremental functions exist
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "\df refresh_*"

# Re-run all migrations if needed
./scripts/run_migrations.sh
```

---

## Understanding Your Results

### Interpreting Metrics

#### Latency Percentiles (p50, p95, p99)

**Definition:**
- **p50**: Median latency (50% of requests faster than this)
- **p95**: 95th percentile (95% of requests faster than this)
- **p99**: 99th percentile (99% of requests faster than this)

**Why It Matters:**
- p50 tells you the "typical" user experience
- p95 tells you the "good enough" experience for most users
- p99 tells you the "worst case" experience (outliers)

**Example:**
```
p50: 0.78 ms
p95: 1.47 ms
p99: 2.57 ms
```
This means:
- 50% of queries complete in <0.78ms
- 95% of queries complete in <1.47ms
- 99% of queries complete in <2.57ms

#### Performance Baselines

| Query Type | Without Opt | With Opt | Improvement |
|------------|-------------|----------|-------------|
| Single device | 100-200 ms | 1-5 ms | 40x |
| Time range | 500-1000 ms | 5-10 ms | 100x |
| Aggregation | 2000-5000 ms | <1 ms | 5000x |

---

## Generating More Data

### Data Generation Options

**`generate_data_fast.py`** — Standard generator (recommended for new/empty tables)

```bash
# Small dataset (1M rows) - ~2 minutes, 300 MB
python3 scripts/generate_data_fast.py 1000000

# Medium dataset (10M rows) - ~15 minutes, 3 GB
python3 scripts/generate_data_fast.py 10000000

# Large dataset (50M rows) - ~90 minutes, 15 GB
python3 scripts/generate_data_fast.py 50000000
```

**`generate_data_bulk.py`** — Optimized generator for large datasets on existing tables

For tables that already have millions of rows, the overhead of updating indexes during COPY becomes significant. The bulk generator drops secondary indexes before loading, then recreates them afterward.

```bash
# 100M+ rows on existing table (3-5x faster than fast generator)
python3 scripts/generate_data_bulk.py 100000000

# Skip index drop (if already dropped manually)
python3 scripts/generate_data_bulk.py 50000000 --skip-index-drop

# Skip index creation (debugging/testing only)
python3 scripts/generate_data_bulk.py 10000000 --skip-index-create
```

**When to use which generator:**

| Scenario | Use | Reason |
|----------|-----|--------|
| New/empty table | `generate_data_fast.py` | Simpler, indexes built from scratch anyway |
| Small datasets (<10M rows) | `generate_data_fast.py` | Index overhead is negligible |
| Large datasets (>10M rows) on existing table | `generate_data_bulk.py` | Eliminates index maintenance overhead |
| Rebuilding indexes manually | `generate_data_bulk.py --skip-index-drop` | If indexes already dropped |

### Index Verification

After generating data (especially with `generate_data_bulk.py`), verify that all indexes were recreated correctly:

```bash
# Standalone verification
python3 scripts/verify_indexes.py

# Verbose mode (shows detailed comparison)
python3 scripts/verify_indexes.py --verbose

# Custom database URL
python3 scripts/verify_indexes.py --db-url "postgres://..."

# Exit codes: 0 = all OK, 1 = some indexes missing/incorrect
```

The `generate_data_bulk.py` script automatically runs index verification at the end (Phase 5), unless you use `--skip-index-verify`.

### Custom Data Generation

Edit `scripts/generate_data_fast.py` or `scripts/generate_data_bulk.py`:

```python
# Change number of devices
NUM_DEVICES = 1000  # Fewer devices

# Change time range
DATA_DURATION_DAYS = 365  # Full year

# Add custom reading types
READING_TYPES = ['temperature', 'humidity', 'pressure', 'co2', 'light']
```

---

## Reference Guide

### Quick Commands

```bash
# === Service Management ===
docker-compose up -d              # Start all services
docker-compose down               # Stop all services
docker-compose restart api        # Restart API
docker-compose logs -f api        # View API logs

# === Database ===
docker exec -it highth-postgres psql -U sensor_user -d sensor_db
docker exec highth-postgres psql -U sensor_user -d sensor_db -c "SELECT count(*) FROM sensor_readings;"

# === Data Generation ===
python3 scripts/generate_data_fast.py 1000000
python3 scripts/generate_data_bulk.py 10000000           # With index optimization
python3 scripts/generate_data_bulk.py 50000000 --skip-index-verify  # Skip verification

# === Index Verification ===
python3 scripts/verify_indexes.py                           # Verify all indexes
python3 scripts/verify_indexes.py --verbose                  # Show detailed comparison

# === Migrations ===
./scripts/run_migrations.sh

# === Testing ===
./tests/run-benchmarks.sh

# === Materialized Views ===
./scripts/refresh_materialized_views.sh all

# === Cache ===
docker exec highth-redis redis-cli FLUSHALL
docker exec highth-redis redis-cli KEYS "*"
```

### API Endpoints

| Endpoint | Method | Description | Key Parameters |
|----------|--------|-------------|------------------|
| `/health` | GET | Health check with dependency status and latency | - |
| `/health/ready` | GET | Readiness probe | - |
| `/health/live` | GET | Liveness probe | - |
| `/api/v1/sensor-readings` | GET | Unified endpoint for sensor readings query | `id` OR `device_id` (required, mutually exclusive), `limit` (1-500), `reading_type`, `from`, `to` |
| `/api/v1/stats` | GET | Get database statistics | - |
| `/metrics` | GET | Prometheus metrics | - |

**Notes:**
- `/api/v1/sensor-readings` requires **exactly one** of `id` or `device_id` - returns 400 if both or neither provided
- When `id` is provided: Single-row primary key lookup (returns 404 `READING_NOT_FOUND` if not found)
- When `device_id` is provided: Device query mode (returns 404 `DEVICE_NOT_FOUND` if no readings)
- All responses include `X-Request-ID`, `X-Response-Time`, `X-Cache-Status`, and `Cache-Control` headers

---

## Troubleshooting

### Common Issues

#### Port Conflicts

```bash
# Check what's using the port
sudo lsof -i :5434  # PostgreSQL
sudo lsof -i :6380  # Redis
sudo lsof -i :8080  # API

# Change port in .env file
```

#### Slow Queries

```bash
# Check query plan
docker exec -it highth-postgres psql -U sensor_user -d sensor_db
EXPLAIN ANALYZE SELECT * FROM sensor_readings WHERE device_id = 'sensor-000001';

# Look for: Index Scan (good), Seq Scan (bad)
```

#### Cache Not Working

```bash
# Verify Redis is running
docker exec highth-redis redis-cli PING

# Check cache keys
docker exec highth-redis redis-cli KEYS "*"
```

#### Data Persistence Issues

```bash
# Check data folders exist
ls -la data/postgres
ls -la data/redis

# Verify docker-compose.yml has correct volume mounts
grep -A 2 "volumes:" docker-compose.yml

# Should show:
# ./data/postgres:/var/lib/postgresql/data
# ./data/redis:/data
```

---

## Additional Resources

### Documentation Files

- **`docs/high-throughput-guide/`** - **Production guide** for building high-throughput PostgreSQL + Golang systems (≤500ms median latency)
- **`docs/api-spec.md`** - Complete REST API specification with all endpoints
- **`docs/architecture.md`** - Detailed system architecture and design decisions
- **`docs/stack.md`** - Technology stack details and version requirements
- **`docs/testing.md`** - Comprehensive testing methodology
- **`docs/ui-consideration.md`** - UI/UX design considerations

### Implementation Guides

The `docs/implementation/` folder contains detailed guides for:
- Advanced indexing strategies
- Materialized view design
- Monitoring and metrics setup
- Performance optimization techniques

---

## Conclusion

**Higth** provides a complete platform for IoT time-series experiments at scale with a clean, well-organized codebase following Go best practices.

### Key Takeaways

1. **Clean Architecture**: Separation of concerns with handler/service/repository layers
2. **Production-Ready**: Docker-based deployment with health checks and auto-restart
3. **High Performance**: BRIN indexes, materialized views, Redis caching
4. **Observable**: Prometheus metrics for monitoring
5. **Well-Documented**: Comprehensive docs for setup and experimentation

### Next Steps

1. ✅ Complete Quick Start
2. ✅ Run Basic Experiments
3. ✅ Explore Performance Experiments
4. ✅ Generate More Data as needed
5. ✅ Customize for your experiments

---

**Happy Experimenting!**

*Last Updated: 2026-03-18* | *Version: 1.0.0*
