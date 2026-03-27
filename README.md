# Higth - IoT Sensor Query System

**A production-grade IoT sensor data query system optimized for time-series workloads with PostgreSQL materialized views, advanced indexing, and Redis caching.**

---

## Table of Contents

- [Repository Structure](#repository-structure)
- [What is Higth?](#what-is-higth)
- [Quick Start (15 Minutes)](#quick-start-15-minutes)
- [Detailed Setup Guide](#detailed-setup-guide)
- [Understanding the System](#understanding-the-system)
- [Running Experiments](#running-experiments)
- [Running Load Tests](#running-load-tests)
- [Benchmark Testing](#benchmark-testing)
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
│   │   │   ├── 002_advanced_indexes.sql      # BRIN & covering indexes
│   │   │   ├── 004_materialized_views.sql   # Hourly/daily/global stats MVs
│   │   │   └── 005_incremental_mv_refresh.sql # Incremental MV refresh functions
│   │   └── (schema.sql)        # Initial schema (if needed)
│   │
│   ├── docker-compose.yml      # Docker services definition
│   ├── generate_data.go        # Go data generator (slower)
│   ├── generate_data_fast.py   # Python data generator (fast, uses COPY)
│   ├── refresh_materialized_views.sh  # MV refresh automation
│   ├── run_migrations.sh       # Automated migration runner
│   └── test-runner.sh          # Load testing with Vegeta (legacy)
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
| **`test-results/`** | Test Outputs | Load test results generated by `test-runner.sh`. Each test run creates a timestamped folder with detailed metrics. |
| **`bin/`** | Compiled Binaries | Output directory for compiled Go executables. Contains `api` (server) and `generate_data` (generator). |

### Key Files Explained

| File | Purpose | Description |
|------|---------|-------------|
| **`docker-compose.yml`** | Container Orchestration | Defines 3 services: `api` (built from Dockerfile), `postgres` (PostgreSQL 16), and `redis` (Redis 7). Configures networking, volumes, and health checks. |
| **`Dockerfile`** | Container Build | Multi-stage build for the API. Stage 1: Build Go binary. Stage 2: Minimal Alpine image with only the binary. Results in ~20MB image. |
| **`.env`** | Environment Config | Contains sensitive configuration (database passwords, Redis settings). NOT in git. Use `.env.example` as template. |
| **`go.mod`** | Go Dependencies | Lists all Go module dependencies. Defines the module path and required Go version (1.21+). |
| **`generate_data_fast.py`** | Data Generator | Fast data generator using PostgreSQL COPY command. Generates 5,000 rows/sec. Creates realistic IoT sensor data with Zipf distribution. |

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

**Higth** is a production-grade IoT sensor query system designed for experimenting with time-series data at scale. It simulates a real-world scenario where thousands of IoT devices continuously send sensor readings that need to be queried and analyzed.

### Key Features

- **Time-Series Optimized**: BRIN indexes for efficient time-range queries (99% smaller than B-tree)
- **Materialized Views**: Pre-computed aggregations for 100-200x faster dashboard queries
- **Smart Caching**: Redis-based LRU cache with 30s TTL
- **Connection Pooling**: PgBouncer integration for high concurrency
- **Prometheus Metrics**: Built-in observability for monitoring
- **Load Testing**: Included test infrastructure for performance experiments

### Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **API** | Go 1.21+ | High-performance REST API |
| **Database** | PostgreSQL 15+ | Time-series data storage with advanced optimizations |
| **Cache** | Redis 7+ | Query result caching |
| **Pool** | PgBouncer | PostgreSQL connection pooling |
| **Metrics** | Prometheus | Performance monitoring |
| **Testing** | Vegeta | HTTP load testing |

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

# 4. Run database migrations (automated)
./scripts/run_migrations.sh

# 5. Generate test data (1,000 rows for quick start)
./scripts/generate_data_fast.py 1000 --devices 10 --days 1

# 6. Re-run migrations to create performance indexes and materialized views
./scripts/run_migrations.sh

# 7. Test the API
curl http://localhost:8080/health
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=10"
```

**IMPORTANT:** The migration runner automatically tracks and applies pending migrations. See [Database Migrations](#database-migrations) below for details.

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

**Step 3: Initialize Database**

```bash
# Apply advanced indexes migration
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/002_advanced_indexes.sql

# Apply materialized views migration
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/004_materialized_views.sql

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

# 2. Create database
createdb sensor_db

# 3. Run schema initialization
psql -U $USER -d sensor_db < scripts/schema/migrations/002_advanced_indexes.sql
psql -U $USER -d sensor_db < scripts/schema/migrations/004_materialized_views.sql

# 4. Configure environment variables
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export REDIS_HOST=localhost
export REDIS_PORT=6379

# 5. Run API
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

#### 4. Prometheus Metrics
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
# Run the included load test script
./scripts/test-runner.sh concurrent

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

## Running Load Tests

The Higth project includes a comprehensive load testing suite powered by [Vegeta](https://github.com/tsenart/vegeta), an HTTP load testing tool.

### Quick Start

```bash
# Run all tests
./scripts/test-runner.sh

# Run specific test
./scripts/test-runner.sh concurrent
./scripts/test-runner.sh hot_device
./scripts/test-runner.sh large_n
```

### Test Scenarios

The test suite includes **6 comprehensive scenarios**:

| Test | Purpose | Duration | Target |
|------|---------|----------|--------|
| **Health Check** | Verify API is responding | 10s | p50 ≤ 10ms |
| **Cold Start** | Measure performance with empty cache | 10s | p50 ≤ 600ms |
| **Baseline** | Measure performance with warm cache | 30s | p50 ≤ 50ms |
| **Concurrent** ⭐ | **PRIMARY TEST** - 50 RPS load | 60s | p50 ≤ 500ms, p95 ≤ 800ms |
| **Hot Device** | Simulate skewed access (90% to one device) | 30s | p50 ≤ 500ms, p99 ≤ 2×p95 |
| **Large N** | Test large result sets (limit=500) | 30s | p50 ≤ 500ms |

### Understanding Test Results

Test results are saved to `test-results/TIMESTAMP/`:

```
test-results/20260317_110226/
├── health.txt          # Health check results
├── cold_start.txt      # Cold cache performance
├── baseline.txt        # Warm cache baseline
├── concurrent.txt      # ⭐ PRIMARY TEST RESULTS
├── hot_device.txt      # Skewed access pattern
└── large_n.txt         # Large result set test
```

### Interpreting Results

Each result file contains Vegeta output format:

```
Requests: 3000 @ 50.02 RPS
Duration: 59.98s

Latencies:
  Mean:   890.98 μs
  p50:    779.37 μs  ← Median: 50% of requests faster than this
  p95:    1.47 ms    ← 95th percentile: 95% of requests faster than this
  p99:    2.57 ms    ← 99th percentile: 99% of requests faster than this
  Max:    34.35 ms

Bytes In: 138 KB (46 bytes/response)
Success Rate: 100% (all HTTP 200 OK)
```

**What These Metrics Mean:**

- **p50 (Median)**: Half of all requests completed faster than this time
- **p95**: 95% of requests completed faster than this time
- **p99**: 99% of requests completed faster than this time
- **RPS**: Requests Per Second - how much load was applied
- **Success Rate**: Percentage of requests that succeeded (HTTP 2xx/3xx)

**Performance Targets:**
- p50 ≤ 500 ms ✅ (Actual: 0.78 ms - 642× better!)
- p95 ≤ 800 ms ✅ (Actual: 1.47 ms - 544× better!)

### Test Results Summary

After running tests, a color-coded summary is displayed:

```
╔════════════════════════════════════════════════════════════════╗
║              Load Test Results Summary                           ║
╠════════════════════════════════════════════════════════════════╣
║  Test            Status    p50       p95       p99    Errors    ║
║ ────────────────────────────────────────────────────────────────── ║
║  Health          ✅ PASS   0.95 ms   24.5 ms   24.5 ms  0%      ║
║  Cold Start      ✅ PASS   0.78 ms   4.28 ms   4.28 ms   0%      ║
║  Baseline        ✅ PASS   0.96 ms   1.9 ms    2.25 ms   0%      ║
║  **Concurrent**  ✅ PASS   0.78 ms   1.47 ms   2.57 ms   0%      ║
║  Hot Device      ✅ PASS   0.69 ms   1.17 ms   1.67 ms   0%      ║
║  Large N         ✅ PASS   0.70 ms   0.99 ms   1.09 ms   0%      ║
╚════════════════════════════════════════════════════════════════╝
```

**All tests passed! Performance targets exceeded by 100-600x.**

### Running Individual Tests

**Test 1: Health Check**
```bash
./scripts/test-runner.sh health
```
Verifies the API is responding and healthy.

**Test 2: Cold Start**
```bash
./scripts/test-runner.sh cold_start
```
Measures performance when cache is empty (worst case).

**Test 3: Baseline**
```bash
./scripts/test-runner.sh baseline
```
Measures normal performance with warm cache.

**Test 4: Concurrent (PRIMARY TEST)** ⭐
```bash
./scripts/test-runner.sh concurrent
```
Applies 50 RPS load for 60 seconds to verify system can handle target load.

**Test 5: Hot Device**
```bash
./scripts/test-runner.sh hot_device
```
Simulates realistic skewed access where 90% of queries go to one device.

**Test 6: Large N**
```bash
./scripts/test-runner.sh large_n
```
Tests performance with large result sets (limit=500 rows).

### Custom Load Testing

You can also run custom tests with Vegeta directly:

```bash
# Define your test
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100" | \
  vegeta attack -duration=30s -rate=10 | vegeta report

# Generate report
echo "GET http://localhost:8080/api/v1/sensor-readings?device_id=sensor-000001&limit=100" | \
  vegeta attack -duration=30s -rate=10 > results.bin
vegeta report results.bin
```

### Analyzing Results Further

To analyze specific test results:

```bash
# View detailed results
cat test-results/20260317_110226/concurrent.txt

# Get summary statistics
vegeta report --type=text test-results/20260317_110226/concurrent.txt

# Generate JSON output for further analysis
vegeta report --type=json test-results/20260317_110226/concurrent.txt > metrics.json
```

### Performance Baselines

**Good Performance** (you're on track):
- p50 < 10 ms with warm cache
- p95 < 50 ms under load
- Error rate = 0%

**Excellent Performance** (exceeds targets):
- p50 < 1 ms ✅ (Current system achieves this!)
- p95 < 5 ms ✅ (Current system achieves this!)
- Error rate = 0% ✅

**Needs Investigation**:
- p50 > 100 ms → Check database indexes
- p95 > 500 ms → Check query plans, caching
- Error rate > 1% → Check logs for failures

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

**List all available scenarios:**

```bash
./tests/run-benchmarks.sh --list
```

### Benchmark Scenarios

The test suite includes **4 comprehensive scenarios** that simulate real-world IoT traffic patterns:

| Scenario | Purpose | Duration | Target |
|----------|---------|----------|--------|
| **Hot Device Pattern** | Zipf distribution (20% devices get 80% traffic) | 3 min | p95 < 500ms |
| **Time-Range Queries** | Dashboard-style queries (1h/24h/7d) | 2 min | p95 < 500ms |
| **Mixed Workload** ⭐ | **PRIMARY TEST** - Real API usage mix | 2.5 min | p95 < 400ms |
| **Cache Performance** | Cold → Warm → Hot cache phases | 3 min | Hot: p95 < 100ms |

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

```bash
# Small dataset (1M rows) - 2 minutes, 300 MB
python3 scripts/generate_data_fast.py 1000000

# Medium dataset (10M rows) - 15 minutes, 3 GB
python3 scripts/generate_data_fast.py 10000000

# Large dataset (50M rows) - 90 minutes, 15 GB
python3 scripts/generate_data_fast.py 50000000
```

### Custom Data Generation

Edit `scripts/generate_data_fast.py`:

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

# === Migrations ===
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/002_advanced_indexes.sql
docker exec -i highth-postgres psql -U sensor_user -d sensor_db < scripts/schema/migrations/004_materialized_views.sql

# === Testing ===
./scripts/test-runner.sh

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
| `/api/v1/sensor-readings` | GET | Query sensor readings for a device | `device_id` (required), `limit` (1-500), `reading_type` |
| `/api/v1/stats` | GET | Get database statistics | - |
| `/metrics` | GET | Prometheus metrics | - |

**Notes:**
- `device_id` is **required** for `/api/v1/sensor-readings` - returns 400 if missing
- Returns 404 `DEVICE_NOT_FOUND` if device has no readings (not empty array with 200)
- All responses include `X-Request-ID`, `X-Response-Time`, and `Cache-Control` headers

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
