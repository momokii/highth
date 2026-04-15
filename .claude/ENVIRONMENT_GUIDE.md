# Environment Guide — Higth Project

Verified commands and environment configuration derived from the actual running codebase.

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25.7 | API development |
| Docker + Docker Compose | v2+ | PostgreSQL, Redis, API containers |
| Python 3 | Any | Data generation scripts |
| jq | Any | Parsing benchmark JSON results |
| k6 | Via Docker | Benchmark testing (auto-pulled) |

---

## Initial Setup

```bash
# 1. Create environment file
cp .env.example .env

# 2. Create external Docker network (required before first compose up)
docker network create highth-network 2>/dev/null

# 3. Start all services
docker compose up -d --build

# 4. Run database migrations
./scripts/run_migrations.sh

# 5. Generate seed data (Python, uses COPY protocol — fast)
python3 scripts/generate_data_fast.py
```

---

## Running the API

```bash
# Via Docker (included in docker compose up)
# API available at http://localhost:8080

# Locally (requires running postgres + redis containers)
go run ./cmd/api

# Build binary
go build -o bin/api ./cmd/api
```

---

## Database Operations

```bash
# Run pending migrations
./scripts/run_migrations.sh

# Preview migrations without applying
./scripts/run_migrations.sh --dry-run

# Force re-run already-applied migrations
./scripts/run_migrations.sh --force

# Refresh materialized views
./scripts/refresh_materialized_views.sh all     # All MVs
./scripts/refresh_materialized_views.sh hourly   # Hourly only
./scripts/refresh_materialized_views.sh daily    # Daily only

# Verify all indexes exist and are correct
python3 scripts/verify_indexes.py

# Connect to PostgreSQL
docker exec -it highth-postgres psql -U sensor_user -d sensor_db
```

---

## Data Generation

```bash
# Fast generator (100K+ rows/sec via COPY protocol)
python3 scripts/generate_data_fast.py                  # Default: 1M rows
python3 scripts/generate_data_fast.py 10000000         # 10M rows
python3 scripts/generate_data_fast.py 50000000 --days 90  # 50M rows over 90 days

# Bulk generator (drops/recreates indexes for faster loading)
python3 scripts/generate_data_bulk.py 10000000         # 10M rows with index optimization
```

---

## Benchmarking (k6)

```bash
# Run all scenarios
./tests/run-benchmarks.sh

# Smoke test (minimal load, verify runs)
./tests/run-benchmarks.sh --tier smoke

# Specific scenario with tier
./tests/run-benchmarks.sh --tier medium -s hot
./tests/run-benchmarks.sh --tier high -s cache

# With HTML report
./tests/run-benchmarks.sh --tier high --with-html-report

# Custom load
./tests/run-benchmarks.sh --rps 200 --duration 2m

# List available scenarios
./tests/run-benchmarks.sh --list

# Verbose output with detailed metrics
./tests/run-benchmarks.sh --tier low --verbose
```

**Tiers**: `smoke` → `low` → `medium` → `high` → `expert`
**Scenarios**: `hot`, `time-range`, `mixed`, `cache`, `stats`, `pk-lookup`

---

## Health Verification

```bash
# Full health check (DB + Redis with latency)
curl -s http://localhost:8080/health | jq .

# Readiness (DB only)
curl -s http://localhost:8080/health/ready

# Liveness (always 200)
curl -s http://localhost:8080/health/live

# Prometheus metrics
curl -s http://localhost:8080/metrics

# Quick smoke test
curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=5" | jq .
```

---

## Monitoring Stack

```bash
# Start with monitoring overlay
docker compose -f docker-compose.yml -f compose.monitoring.yml up -d

# Access points
# Grafana:    http://localhost:3000  (admin/admin)
# Prometheus:  http://localhost:9090
# API metrics: http://localhost:8080/metrics
```

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| `docker compose up` fails with network error | `highth-network` doesn't exist | `docker network create highth-network` |
| API won't start, DB connection error | Postgres not ready yet | Wait for health check: `docker compose ps` |
| Benchmark tests fail to connect | API not running | `docker compose up -d`, check `curl localhost:8080/health` |
| Postgres performance poor on small machine | Config tuned for 8+ cores | Reduce `shared_buffers`, `effective_cache_size` in docker-compose.yml |
| Port 5434 already in use | Host postgres conflict | Port 5434 is mapped (not 5432). Change in docker-compose.yml if needed. |
| Redis connection refused | Redis not running | `docker compose up -d redis` |
| `python3 scripts/generate_data_fast.py` fails | DATABASE_URL not set | Set in .env or pass `--db-url` flag |
| Module not found errors | Wrong module path | Module is `github.com/kelanach/higth` ("higth" not "highth") |

---

## Environment Variables

Key variables from `.env.example`:

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `REDIS_URL` | `redis://redis:6379` | Redis connection string |
| `REDIS_ENABLED` | `true` | Toggle Redis cache |
| `CACHE_ENABLED` | `true` | Toggle cache in service layer |
| `DB_MAX_CONNECTIONS` | `50` | Connection pool max (docker-compose uses 200) |
| `PORT` | `8080` | API listen port |
| `LOG_LEVEL` | `info` | Logging verbosity |
| `REQUEST_TIMEOUT` | `30s` | HTTP request timeout |
