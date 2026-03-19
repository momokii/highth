# Phase 1: Database Provisioning Tasks

**Goal:** Provision PostgreSQL 16+ with optimized schema and indexes for 50M+ rows.

**Estimated Time:** 2-4 hours
**Total Tasks:** 10
**Entry Criteria:** Phase 0 complete

---

## TASK-009: Create Docker Compose Configuration

**Status:** pending
**Dependencies:** TASK-008
**Estimated Time:** 20 minutes

**Description:**
Create docker-compose.yml to run PostgreSQL and Redis containers.

**Steps:**
1. Create `docker-compose.yml` in project root
2. Define PostgreSQL 16+ service with configuration
3. Define Redis service
4. Configure volumes for persistence
5. Expose ports 5432 (PostgreSQL) and 6379 (Redis)

**Output Definition:**
- docker-compose.yml file exists
- PostgreSQL service defined with proper configuration
- Redis service defined
- Volumes configured for data persistence

**File:** `docker-compose.yml`

**Expected Contents:**
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: highth-postgres
    environment:
      POSTGRES_USER: sensor_user
      POSTGRES_PASSWORD: sensor_password
      POSTGRES_DB: sensor_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    command: >
      postgres
      -c shared_buffers=256MB
      -c effective_cache_size=1GB
      -c work_mem=16MB
      -c maintenance_work_mem=128MB
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sensor_user"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: highth-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
  redis_data:
```

**Verification Commands:**
```bash
cat docker-compose.yml
docker compose config
```

**Next Task:** TASK-010

---

## TASK-010: Start PostgreSQL Container

**Status:** pending
**Dependencies:** TASK-009
**Estimated Time:** 10 minutes

**Description:**
Start PostgreSQL container using Docker Compose.

**Steps:**
1. Run `docker compose up -d postgres`
2. Wait for container to be healthy
3. Verify PostgreSQL is accepting connections

**Output Definition:**
- PostgreSQL container running
- Health check passing
- Port 5432 accessible

**Verification Commands:**
```bash
docker ps | grep highth-postgres
docker compose ps postgres
docker compose logs postgres | tail -20
```

**Expected Output:**
```
highth-postgres   Up   0.0.0.0:5432->5432/tcp   (healthy)
```

**Troubleshooting:**
- If port conflict: Check if PostgreSQL is already running on host
- If container not healthy: Check logs with `docker compose logs postgres`

**Next Task:** TASK-011

---

## TASK-011: Create Database Schema SQL File

**Status:** pending
**Dependencies:** TASK-010
**Estimated Time:** 20 minutes

**Description:**
Create init.sql file with complete database schema.

**Steps:**
1. Create `scripts/init.sql` file
2. Define sensor_readings table
3. Add column comments
4. Save file

**Output Definition:**
- init.sql file exists with complete schema

**File:** `scripts/init.sql`

**Expected Contents:**
```sql
-- Drop table if exists (for clean restarts)
DROP TABLE IF EXISTS sensor_readings;

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

-- Add table comment
COMMENT ON TABLE sensor_readings IS 'IoT sensor telemetry data with optimized indexing for high-volume queries';

-- Add column comments
COMMENT ON COLUMN sensor_readings.id IS 'Unique identifier (auto-generated)';
COMMENT ON COLUMN sensor_readings.device_id IS 'Device identifier (e.g., sensor-001)';
COMMENT ON COLUMN sensor_readings.timestamp IS 'Reading timestamp (UTC)';
COMMENT ON COLUMN sensor_readings.reading_type IS 'Type of reading (temperature, humidity, etc.)';
COMMENT ON COLUMN sensor_readings.value IS 'Sensor reading value';
COMMENT ON COLUMN sensor_readings.unit IS 'Unit of measurement (celsius, percent, etc.)';
COMMENT ON COLUMN sensor_readings.metadata IS 'Additional device metadata (JSON)';
```

**Verification Commands:**
```bash
cat scripts/init.sql
```

**Next Task:** TASK-012

---

## TASK-012: Create sensor_db Database

**Status:** pending
**Dependencies:** TASK-011
**Estimated Time:** 5 minutes

**Description:**
The database is already created by Docker Compose, but verify it exists.

**Steps:**
1. Connect to PostgreSQL
2. Verify sensor_db exists
3. Verify connection string works

**Output Definition:**
- sensor_db database exists
- Connection successful

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\l"
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "SELECT current_database();"
```

**Expected Output:**
```
 sensor_db
----------
 sensor_db
```

**Note:** Database is created automatically by Docker Compose POSTGRES_DB variable.

**Next Task:** TASK-013

---

## TASK-013: Create sensor_readings Table

**Status:** pending
**Dependencies:** TASK-012
**Estimated Time:** 10 minutes

**Description:**
Create the sensor_readings table using the init.sql script.

**Steps:**
1. Run init.sql script
2. Verify table created
3. Verify schema matches specification

**Output Definition:**
- sensor_readings table exists
- All columns present with correct types

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -f scripts/init.sql
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\dt sensor_readings"
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\d sensor_readings"
```

**Expected Output:**
```
                List of relations
 Schema |          Name           | Type  |  Owner
--------+-------------------------+-------+-------------
 public  | sensor_readings        | table | sensor_user
```

**Next Task:** TASK-014

---

## TASK-014: Create BRIN Index on Timestamp

**Status:** pending
**Dependencies:** TASK-013
**Estimated Time:** 5 minutes

**Description:**
Create BRIN index on timestamp column for time-range queries.

**Steps:**
1. Create BRIN index with pages_per_range = 128
2. Verify index created

**Output Definition:**
- BRIN index idx_sensor_readings_ts_brin exists

**SQL:**
```sql
CREATE INDEX idx_sensor_readings_ts_brin
ON sensor_readings USING BRIN (timestamp)
WITH (pages_per_range = 128);
```

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\di idx_sensor_readings_ts_brin"
```

**Expected Output:**
```
                          List of relations
 Schema |           Name            | Type  |  Owner   |      Table
--------+---------------------------+-------+----------+----------------
 public  | idx_sensor_readings_ts_brin | index | sensor_user | sensor_readings
```

**Next Task:** TASK-015

---

## TASK-015: Create Composite B-tree Index

**Status:** pending
**Dependencies:** TASK-014
**Estimated Time:** 5 minutes

**Description:**
Create composite B-tree index on (device_id, timestamp DESC) for device lookups.

**Steps:**
1. Create composite index with DESC on timestamp
2. Verify index created

**Output Definition:**
- Composite index idx_sensor_readings_device_ts exists

**SQL:**
```sql
CREATE INDEX idx_sensor_readings_device_ts
ON sensor_readings (device_id, timestamp DESC);
```

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\di idx_sensor_readings_device_ts"
```

**Expected Output:**
```
                              List of relations
 Schema |            Name             | Type  |  Owner   |      Table
--------+-----------------------------+-------+----------+----------------
 public  | idx_sensor_readings_device_ts | index | sensor_user | sensor_readings
```

**Next Task:** TASK-016

---

## TASK-016: Create Covering Index with INCLUDE

**Status:** pending
**Dependencies:** TASK-015
**Estimated Time:** 10 minutes

**Description:**
Create covering index to enable index-only scans.

**Steps:**
1. Create covering index with INCLUDE clause
2. Add frequently accessed columns to INCLUDE
3. Verify index created

**Output Definition:**
- Covering index idx_sensor_readings_device_covering exists

**SQL:**
```sql
CREATE INDEX idx_sensor_readings_device_covering
ON sensor_readings (device_id, timestamp DESC)
INCLUDE (reading_type, value, unit);
```

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\di idx_sensor_readings_device_covering"
```

**Expected Output:**
```
                                List of relations
 Schema |              Name               | Type  |  Owner   |      Table
--------+---------------------------------+-------+----------+----------------
 public  | idx_sensor_readings_device_covering | index | sensor_user | sensor_readings
```

**Note:** This index enables index-only scans for the primary query pattern.

**Next Task:** TASK-017

---

## TASK-017: Run ANALYZE on Table

**Status:** pending
**Dependencies:** TASK-016
**Estimated Time:** 5 minutes

**Description:**
Run ANALYZE to update statistics for the query optimizer.

**Steps:**
1. Run ANALYZE on sensor_readings table
2. Verify statistics updated

**Output Definition:**
- ANALYZE completed successfully

**Verification Commands:**
```bash
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "ANALYZE sensor_readings;"
```

**Expected Output:**
```
ANALYZE
```

**Note:** ANALYZE will be run again after data generation (TASK-023).

**Next Task:** TASK-018

---

## TASK-018: Verify Database Connection

**Status:** pending
**Dependencies:** TASK-017
**Estimated Time:** 5 minutes

**Description:**
Verify database connection is working from application context.

**Steps:**
1. Test connection with psql
2. Verify all indexes exist
3. Verify table is empty (0 rows)

**Output Definition:**
- Connection successful
- All 3 indexes verified
- Row count = 0

**Verification Commands:**
```bash
# Connection test
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "SELECT version();"

# Index verification
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "\di" | grep sensor_readings

# Row count
psql "postgres://sensor_user:sensor_password@localhost:5432/sensor_db" -c "SELECT count(*) FROM sensor_readings;"
```

**Expected Output:**
```
                                                  version
---------------------------------------------------------------------------------------------------------
 PostgreSQL 16.1 on x86_64-pc-linux-gnu, compiled by gcc (Debian 12.2.0-14) 16.1

 idx_sensor_readings_ts_brin | sensor_readings | index | sensor_user
 idx_sensor_readings_device_ts | sensor_readings | index | sensor_user
 idx_sensor_readings_device_covering | sensor_readings | index | sensor_user

 count
-------
     0
```

**Next Task:** TASK-019 (Phase 2)

---

## Phase 1 Completion Checklist

- [ ] TASK-009: docker-compose.yml created
- [ ] TASK-010: PostgreSQL container running
- [ ] TASK-011: init.sql schema file created
- [ ] TASK-012: sensor_db database verified
- [ ] TASK-013: sensor_readings table created
- [ ] TASK-014: BRIN index on timestamp created
- [ ] TASK-015: Composite B-tree index created
- [ ] TASK-016: Covering index created
- [ ] TASK-017: ANALYZE run
- [ ] TASK-018: Database connection verified

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 2.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
