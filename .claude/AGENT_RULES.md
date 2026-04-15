# Agent Rules — Higth Project

Non-negotiable behavioral rules for every session. Read before writing any code.

---

## Session Start — Mandatory Before Any Action

1. Read `.claude/state/CURRENT_STATUS.md` — understand what is done and what remains
2. Read `.claude/state/TASK_QUEUE.md` — identify the next task
3. Read `.claude/CODING_STANDARDS.md` — internalize conventions before writing code
4. Read `.claude/SECURITY_STANDARDS.md` — internalize security requirements
5. Verify the environment: `docker compose ps`, `curl -s http://localhost:8080/health`
6. Build check: `go build ./cmd/api`

## Architecture Invariants

- **Layered architecture is non-negotiable**: Handler → Service → Repository → PostgreSQL
- **No layer skipping**: Handler never touches DB. Service never returns HTTP status codes. Repository never touches cache.
- **No circular imports**: handler imports service; service imports repository and cache; repository imports model. Never reverse.
- **Module path**: Always `github.com/kelanach/higth` — note "higth" not "highth"

## Error Handling

- Use sentinel errors from `internal/service/sensor_service.go`: `ErrInvalidParameter`, `ErrDeviceNotFound`, `ErrReadingNotFound`
- Wrap with `fmt.Errorf("%w: context", sentinelErr)`
- Check with `errors.Is(err, service.ErrXxx)` in handler
- Map to HTTP codes in handler only: BadRequest, NotFound, InternalServerError

## Response Format

- Success: `{"data": ..., "meta": {"count": N, ...}}`
- Error: `{"error": {"code": "DEVICE_NOT_FOUND", "message": "...", "timestamp": "..."}}`
- Always set response headers: `X-Cache-Status`, `X-Response-Time`, `X-Request-ID`, `Content-Type: application/json`

## Database

- All SQL must use parameterized queries (`$1, $2, ...`) via pgx/v5
- Never concatenate user input into SQL
- Always use context with timeout (5s per query): `ctx, cancel := context.WithTimeout(ctx, 5*time.Second)`
- ID conversion: DB stores `int64`, JSON returns `string` via `fmt.Sprintf("%d", id)`

## Cache

- Cache-aside pattern in service layer (not repository)
- Stats endpoint bypasses cache — reads MV directly, returns `"BYPASS"` status
- TTL is 30s. Graceful degradation if Redis is down.
- Cache key format: `sensor:{device_id}:readings:{limit}[:{reading_type}][:{from_unix}][:{to_unix}]`
- PK cache key: `sensor:id:{id}`

## Configuration

- All config via environment variables loaded through `internal/config/config.go` (godotenv)
- Never hardcode connection strings, ports, or credentials
- Only `DATABASE_URL` is required — everything else has sensible defaults

## Testing

- Load/benchmark testing is via **k6** (not Vegeta — this project has never used Vegeta)
- Go unit tests do not exist yet — use table-driven pattern with `t.Run()` when adding them
- Run benchmarks: `./tests/run-benchmarks.sh --tier smoke`

## Implementation Rules

- Never make changes outside the stated scope of the current task
- Never delete, rename, or overwrite files without explicit user instruction
- Never introduce a new dependency without user confirmation
- Never modify the database schema without user confirmation
- Follow patterns in `CODING_STANDARDS.md` — do not introduce new patterns without logging in `state/DECISIONS_LOG.md`

## Session End — Mandatory Before Closing

- Update `state/CURRENT_STATUS.md` with what changed
- Update `state/TASK_QUEUE.md` — mark completed tasks, add newly discovered ones
- Log significant decisions in `state/DECISIONS_LOG.md`
- Update `CODING_STANDARDS.md` if new patterns were established

## Escalation

When blocked, uncertain about scope, or facing a decision with significant architectural or security impact: document the blocker in `CURRENT_STATUS.md` and ask the user. Do not assume and proceed.
