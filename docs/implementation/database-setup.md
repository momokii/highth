# Database Setup Guide

This guide covers provisioning PostgreSQL 16+ with optimized schema and indexes for 50M+ sensor readings.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Options](#installation-options)
- [Database Creation](#database-creation)
- [Schema Creation](#schema-creation)
- [Index Creation](#index-creation)
- [Configuration Tuning](#configuration-tuning)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting database setup, ensure:

- [ ] Phase 0 (Environment Setup) complete
- [ ] Docker installed OR PostgreSQL 16+ native installation
- [ ] At least 10GB free disk space (30GB recommended for data + indexes)
- [ ] PostgreSQL client tools (`psql`) installed

---

## Installation Options

### Option 1: Docker (Recommended)

**Pros:** Isolated environment, easy reset, consistent configuration

**Cons:** Additional resource overhead

```bash
# Pull PostgreSQL 16 image
docker pull postgres:16-alpine

# Start container
docker run -d \
  --name postgres-sensor \
  -e POSTGRES_DB=sensor_db \
  -e POSTGRES_USER=sensor_user \
  -e POSTGRES_PASSWORD=your_password_here \
  -p 5432:5432 \
  -v postgres_data:/var/lib/postgresql/data \
  postgres:16-alpine

# Wait for startup
sleep 5

# Verify connection
docker exec -it postgres-sensor psql -U sensor_user -d sensor_db -c "SELECT version();"
```

### Option 2: Native Installation

**Pros:** No container overhead, direct access to system resources

**Cons:** System-wide installation, harder to reset

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y postgresql-16 postgresql-contrib-16

# macOS
brew install postgresql@16
brew link postgresql@16

# Start PostgreSQL service
sudo systemctl start postgresql    # Linux
brew services start postgresql     # macOS
```

---

## Database Creation

### Using Docker

```bash
# Create database in container
docker exec -it postgres-sensor psql -U sensor_user -d postgres << 'EOF'
CREATE DATABASE sensor_db;
\q
EOF

# Or database can be created automatically by POSTGRES_DB environment variable
```

### Using Native PostgreSQL

```bash
# Create database
sudo -u postgres createdb sensor_db

# Create user and grant privileges
sudo -u postgres psql << 'EOF'
CREATE USER sensor_user WITH PASSWORD 'your_password_here';
GRANT ALL PRIVILEGES ON DATABASE sensor_db TO sensor_user;
\q
EOF
```

---

## Schema Creation

### Core Table DDL

```sql
-- Connect to sensor_db
\c sensor_db

-- Create sensor_readings table
CREATE TABLE sensor_readings (
    id              BIGSERIAL       PRIMARY KEY,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(30)     NOT NULL,
    value           NUMERIC(15,6)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL,
    metadata        JSONB
);

-- Add comment for documentation
COMMENT ON TABLE sensor_readings IS 'IoT sensor telemetry readings with flexible metadata';

-- Add column comments
COMMENT ON COLUMN sensor_readings.id IS 'Unique identifier for each reading';
COMMENT ON COLUMN sensor_readings.device_id IS 'Device/sensor identifier (repeating identifier)';
COMMENT ON COLUMN sensor_readings.timestamp IS 'When the reading was taken';
COMMENT ON COLUMN sensor_readings.reading_type IS 'Type of sensor reading (temperature, humidity, pressure, voltage)';
COMMENT ON COLUMN sensor_readings.value IS 'Sensor value with high precision';
COMMENT ON COLUMN sensor_readings.unit IS 'Unit of measurement';
COMMENT ON COLUMN sensor_readings.metadata IS 'Flexible JSONB metadata (firmware version, battery level, etc.)';
```

### Schema Explanation

| Column | Type | Purpose |
|--------|------|---------|
| `id` | BIGSERIAL | Primary key; 64-bit auto-incrementing ID |
| `device_id` | VARCHAR(50) | Repeating identifier; groups readings by device |
| `timestamp` | TIMESTAMPTZ | Time-zone aware timestamp for time-series queries |
| `reading_type` | VARCHAR(30) | Sensor type (temperature, humidity, etc.) |
| `value` | NUMERIC(15,6) | High-precision decimal value |
| `unit` | VARCHAR(20) | Unit (celsius, percent, pascal, volt) |
| `metadata` | JSONB | Flexible device-specific data |

**Why NUMERIC(15,6)?**
- 15 total digits supports values up to 999,999,999.999999
- 6 decimal places provides scientific precision
- Exact decimal arithmetic avoids floating-point errors

### Save Schema to File

```bash
# Create init.sql file
cat > init.sql << 'EOF'
-- Schema for sensor_readings table
CREATE TABLE sensor_readings (
    id              BIGSERIAL       PRIMARY KEY,
    device_id       VARCHAR(50)     NOT NULL,
    timestamp       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    reading_type    VARCHAR(30)     NOT NULL,
    value           NUMERIC(15,6)   NOT NULL,
    unit            VARCHAR(20)     NOT NULL,
    metadata        JSONB
);
EOF
```

---

## Index Creation

> **⚠️ IMPORTANT:** Create indexes in the order shown below. The BRIN index is very fast; the covering index takes the longest.

### Index 1: BRIN Index (Fastest)

```sql
-- BRIN index for time-series queries
-- Creates quickly (~5-10 seconds even with data)
CREATE INDEX idx_sensor_readings_ts_brin
    ON sensor_readings
    USING BRIN (timestamp);

-- Verify
\di idx_sensor_readings_ts_brin
```

**Purpose:** Time-range queries; extremely space-efficient (100x smaller than B-tree)

### Index 2: Composite B-tree (Medium Speed)

```sql
-- Composite index for device lookups with ORDER BY optimization
-- Creates in ~30-60 seconds with 50M rows
CREATE INDEX idx_sensor_readings_device_ts
    ON sensor_readings (device_id, timestamp DESC);

-- Verify
\di idx_sensor_readings_device_ts
```

**Purpose:** Primary query pattern; serves both WHERE and ORDER BY

### Index 3: Covering Index (Slowest)

```sql
-- Covering index to eliminate heap access
-- Creates in ~2-5 minutes with 50M rows
-- Set higher maintenance_work_mem for faster creation
SET maintenance_work_mem = '256MB';

CREATE INDEX idx_sensor_readings_device_covering
    ON sensor_readings (device_id, timestamp DESC)
    INCLUDE (reading_type, value, unit);

-- Reset maintenance_work_mem
SET maintenance_work_mem = '16MB';

-- Verify
\di idx_sensor_readings_device_covering
```

**Purpose:** Index-only scans; eliminates random I/O to table

### Index Verification

```sql
-- View all indexes on sensor_readings
SELECT
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'sensor_readings'
ORDER BY indexname;

-- View index sizes
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size,
    pg_relation_size(indexrelid) as size_bytes
FROM pg_index
JOIN pg_class ON pg_class.oid = indexrelid
WHERE indrelid = 'sensor_readings'::regclass
ORDER BY pg_relation_size(indexrelid) DESC;
```

**Expected sizes at 50M rows:**
- BRIN index: ~20-30 MB
- B-tree composite: ~1.5-2.5 GB
- Covering index: ~2-3 GB

---

## Configuration Tuning

### Docker Compose Configuration

```yaml
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
      - ./init.sql:/docker-entrypoint-initdb.d/01-init.sql:ro
    ports:
      - "5432:5432"
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
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sensor_user -d sensor_db"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

### Configuration Parameters Explained

| Parameter | Value | Explanation |
|-----------|-------|-------------|
| **Memory** |||
| `shared_buffers` | 2GB | PostgreSQL disk cache (25% of RAM on 8GB system) |
| `effective_cache_size` | 6GB | Planner's estimate of total cache (PG + OS file cache = 75% of RAM) |
| `work_mem` | 16MB | Memory per sort/hash operation (per query execution node) |
| `maintenance_work_mem` | 1GB | Memory for VACUUM, CREATE INDEX, and other maintenance operations |
| **Connections** |||
| `max_connections` | 200 | Max concurrent database connections (sufficient for connection pooling) |
| **WAL (Write-Ahead Log)** |||
| `wal_buffers` | 16MB | Write-Ahead Log memory buffer |
| `checkpoint_completion_target` | 0.9 | Spread checkpoint I/O over 90% of interval (prevents I/O spikes) |
| **Query Planner (SSD optimized)** |||
| `random_page_cost` | 1.1 | Cost of non-sequential disk access (default 4.0 is for HDD) |
| `effective_io_concurrency` | 200 | Parallel I/O operations SSD can handle |
| **Parallelism** |||
| `max_worker_processes` | 8 | Max background worker processes (matches CPU cores) |
| `max_parallel_workers_per_gather` | 2 | Max parallel workers per single query operation |
| `max_parallel_workers` | 8 | Max parallel workers across all operations |
| **Background Writer** |||
| `bgwriter_delay` | 200ms | Delay between background writer rounds |
| `bgwriter_lru_maxpages` | 100 | Max buffers flushed per round (limits I/O burst size) |

> **Note:** These parameters are tuned for an 8GB RAM system with SSD storage. For different hardware, adjust based on available RAM and storage characteristics. For detailed explanations of each parameter including trade-offs, see [high-throughput-guide/01-postgresql-setup.md](../high-throughput-guide/01-postgresql-setup.md#parameter-explanations).

---

## Verification

### Step-by-Step Verification

```sql
-- 1. Check database exists
\l

-- 2. Connect to sensor_db
\c sensor_db

-- 3. Check table exists
\dt sensor_readings

-- 4. Check table structure
\d sensor_readings

-- 5. Check indexes
\di sensor_readings_*

-- 6. Check index sizes
SELECT
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_index
JOIN pg_class ON pg_class.oid = indexrelid
WHERE indrelid = 'sensor_readings'::regclass;

-- 7. Verify BRIN index is actually a BRIN index
SELECT
    amname
FROM pg_index
JOIN pg_class ON pg_class.oid = indexrelid
JOIN pg_am ON pg_am.oid = indexrelid::regclass
WHERE indexrelid::regclass = 'sensor_readings'::regclass
  AND indexname = 'idx_sensor_readings_ts_brin';
-- Should return: brin

-- 8. Run ANALYZE for query planner
ANALYZE sensor_readings;

-- 9. Check table size
SELECT
    pg_size_pretty(pg_total_relation_size('sensor_readings')) as total_size,
    pg_size_pretty(pg_relation_size('sensor_readings')) as table_size,
    pg_size_pretty(pg_total_relation_size('sensor_readings') - pg_relation_size('sensor_readings')) as indexes_size;
```

### Test Connection from Host

```bash
# Using connection string
psql "postgres://sensor_user:your_password@localhost:5432/sensor_db" -c "SELECT 1;"

# Expected output: 1 row
```

---

## Troubleshooting

### Connection Issues

**Problem:** `connection refused` on port 5432

**Solutions:**
```bash
# Check if PostgreSQL is running
sudo systemctl status postgresql   # Linux
brew services list                     # macOS
docker ps | grep postgres              # Docker

# Check if port is in use
sudo lsof -i :5432

# Start PostgreSQL
sudo systemctl start postgresql    # Linux
brew services start postgresql      # macOS
docker start postgres-sensor        # Docker
```

**Problem:** `authentication failed`

**Solutions:**
```bash
# Check pg_hba.conf for authentication settings
# For local development, use 'trust' or 'md5'
# Docker: Ensure POSTGRES_PASSWORD matches what you're using

# Reset password
docker exec -it postgres-sensor psql -U postgres
ALTER USER sensor_user WITH PASSWORD 'new_password';
```

### Index Creation Issues

**Problem:** Index creation timeout

**Solutions:**
```sql
-- Increase maintenance_work_mem
SET maintenance_work_mem = '512MB';

-- Drop and recreate index
DROP INDEX IF EXISTS idx_sensor_readings_device_covering;
CREATE INDEX idx_sensor_readings_device_covering
    ON sensor_readings (device_id, timestamp DESC)
    INCLUDE (reading_type, value, unit);

-- Reset maintenance_work_mem
SET maintenance_work_mem = '128MB';
```

**Problem:** Out of disk space during index creation

**Solutions:**
```bash
# Check available disk space
df -h

# Clean up if needed (Docker)
docker system prune -a

-- Consider: Use native PostgreSQL for more control
-- Consider: Reduce dataset size to 10M for testing
```

### Performance Issues

**Problem:** Queries are slow even with indexes

**Investigation:**
```sql
-- Check if indexes are being used (EXPLAIN ANALYZE)
EXPLAIN ANALYZE
SELECT * FROM sensor_readings
WHERE device_id = 'sensor-001'
ORDER BY timestamp DESC
LIMIT 10;

-- Look for:
-- - Index Scan using idx_sensor_readings_device_ts (good)
-- - Index Only Scan using idx_sensor_readings_device_covering (best)
-- - Seq Scan (bad - index not used)

-- Check if table needs vacuum
SELECT schemaname, tablename, last_vacuum, last_autovacuum, autovacuum_count
FROM pg_stat_user_tables
WHERE tablename = 'sensor_readings';

-- Run manual VACUUM if needed
VACUUM ANALYZE sensor_readings;
```

---

## Next Steps

After database setup is complete:

1. **[data-generation.md](data-generation.md)** — Generate 50M test dataset
2. **[api-development.md](api-development.md)** — Build the Go API
3. Verify with `SELECT count(*) FROM sensor_readings;` after data generation

---

## Related Documentation

- **[../architecture.md](../architecture.md)** — Schema design rationale
- **[../stack.md](../stack.md)** — PostgreSQL features and configuration
