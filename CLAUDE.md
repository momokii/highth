# Higth — High-Performance Time-Series Query System

Go API querying 50M+ sensor readings through PostgreSQL + Redis with sub-500ms latency target.

## Commands

```bash
# Start services (external network must exist)
docker network create highth-network 2>/dev/null
docker compose up -d --build

# Stop services
docker compose down

# Run migrations
./scripts/run_migrations.sh              # apply pending
./scripts/run_migrations.sh --dry-run    # preview only
./scripts/run_migrations.sh --force      # re-run already-applied

# Run API locally (needs running postgres + redis)
go run ./cmd/api

# Build binary
go build -o bin/api ./cmd/api

# Seed data (Python fast generator, uses COPY protocol)
python3 scripts/generate_data_fast.py

# Verify indexes
python3 scripts/verify_indexes.py
```

## Benchmarks (k6 via Docker)

```bash
./tests/run-benchmarks.sh                          # all scenarios, default load
./tests/run-benchmarks.sh --tier smoke             # minimal verification
./tests/run-benchmarks.sh --tier medium -s hot     # specific scenario + tier
./tests/run-benchmarks.sh --tier high --with-html-report
./tests/run-benchmarks.sh --rps 200 --duration 2m  # custom load
```

Tiers: `smoke` (verify runs) → `low` (dev baseline) → `medium` (staging) → `high` (stress) → `expert` (find ceiling)

Scenarios: `hot` (cache), `time-range` (MV), `mixed` (multi-endpoint), `cache` (cold/warm/hot phases), `stats` (MV-only), `pk-lookup` (single-row PK scan)

## Architecture

```
cmd/api/main.go          → entry point, wires dependencies
internal/config/         → env loading via godotenv (.env)
internal/handler/        → HTTP handlers (chi router)
internal/service/        → business logic + cache-aside pattern
internal/repository/     → PostgreSQL queries (pgx/v5 pool)
internal/cache/          → Redis cache (go-redis/v9, LRU, 30s TTL)
internal/middleware/      → gzip, Prometheus metrics, request ID, security headers
internal/model/          → data structs
```

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sensor-readings?device_id=X` | Readings by device (limit, reading_type, from, to params) |
| GET | `/api/v1/sensor-readings?id=N` | Single reading by PK (mutually exclusive with device_id) |
| GET | `/api/v1/stats` | DB stats from materialized views (bypasses cache) |
| GET | `/health` | Full health check (DB + Redis with latency) |
| GET | `/health/ready` | Readiness (DB only) |
| GET | `/health/live` | Liveness (always 200) |
| GET | `/metrics` | Prometheus metrics |

## Environment

Copy `.env.example` to `.env`. All config via env vars:

- `DATABASE_URL` — Postgres connection string (required)
- `REDIS_URL` — Redis connection string (includes password)
- `REDIS_PASSWORD` — Redis AUTH password (required in docker-compose)
- `REDIS_ENABLED` / `CACHE_ENABLED` — toggle caching (default: true)
- `DB_MAX_CONNECTIONS` — pool size (default 50, docker-compose uses 200)
- `LOG_LEVEL` — structured logging level: debug, info, warn, error (default: info)
- `PORT` — API port (default 8080)

## Database

Migrations in `scripts/schema/migrations/` — numbered SQL files tracked in `schema_migrations` table. Gap at `003_*` is intentional.

Key schema optimizations:
- **BRIN index** on `timestamp` (99% smaller than B-tree for append-only data)
- **Covering index** with INCLUDE clause (index-only scans)
- **Materialized views** (`mv_global_stats`, hourly/daily aggregations) — refreshed via `scripts/refresh_materialized_views.sh`. Uses `REFRESH MATERIALIZED VIEW CONCURRENTLY` (full scan of all rows). Expect 5-20 min on 50M+ rows. Use `--status` to check MV sizes, `global` for fastest refresh.
- **JSONB metadata column** on `sensor_readings` (migration 007) — stores arbitrary sensor attributes
- Postgres config tuned for 8+ cores, 2GB shared_buffers (may need reduction on small machines)

## Gotchas

- **Docker network**: `highth-network` is external — must create before `docker compose up`
- **Module name**: `github.com/kelanach/higth` (note: "higth" not "highth")
- **Docker runs as non-root**: API container uses `appuser` (UID 1000) — `COPY --chown` in Dockerfile
- **Interface-based DI**: Service accepts `repository.Querier` and `cache.Cache` interfaces; handlers accept `service.SensorServicer` interface
- **Cache-aside pattern**: service checks Redis first, falls back to DB, populates cache. Stats endpoint bypasses cache (reads MV directly)
- **ID as string**: `SensorReading.ID` is stored as `string` in JSON responses (converted from `int64` in repository)
- **Device ID validation**: alphanumeric + hyphens + underscores, max 50 chars
- **Cache scenario**: benchmark runner auto-flushes Redis before cache tests for cold start
- **Stats device count**: uses TABLESAMPLE SYSTEM (0.5%) for estimation, falls back to MV sum
- **Structured logging**: uses `log/slog` with JSON handler (not `log.Printf`). Log level controlled by `LOG_LEVEL` env var
- **Security headers**: all responses include X-Content-Type-Options, X-Frame-Options, X-XSS-Protection
- **OpenAPI spec**: standalone `docs/openapi.yaml` (Swagger-compatible)

## Testing

```bash
# Run all Go unit tests
go test ./internal/... -v

# Run with race detection
go test ./internal/... -race

# Run linting
golangci-lint run ./...
```

Go unit tests use standard library `testing` package with table-driven patterns. Mocks implement interfaces from `repository/interface.go`, `cache/interface.go`, and `service/interface.go`.
