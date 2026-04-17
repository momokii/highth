# Agent Rules — Higth Project

Non-negotiable behavioral rules for every session. Read before writing any code.

---

## Session Start — Mandatory Before Any Action

1. Read `.claude/HOW_TO_RESUME.md` — understand the full resume protocol
2. Read `.claude/state/CURRENT_STATUS.md` — understand what is done and what remains
3. Read `.claude/state/TASK_QUEUE.md` — identify the next task
4. Read `.claude/CODING_STANDARDS.md` — internalize conventions before writing code
5. Read `.claude/SECURITY_STANDARDS.md` — internalize all security requirements
6. Identify the active environment — consult `ENVIRONMENT_GUIDE.md` if in doubt
7. Verify the environment: `docker compose ps`, `curl -s http://localhost:8080/health | jq .`
8. Build check: `go build ./cmd/api` — must succeed before touching any code
9. Read task-relevant docs — PRD section, architecture doc, API contract, or any doc directly relevant to the current task

---

## Architecture Invariants

- **Layered architecture is non-negotiable**: Handler → Service → Repository → PostgreSQL
- **No layer skipping**: Handler never touches DB. Service never returns HTTP status codes. Repository never touches cache.
- **No circular imports**: handler imports service; service imports repository and cache; repository imports model. Never reverse.
- **Module path**: Always `github.com/kelanach/higth` — note "higth" not "highth"

---

## Error Handling

- Use sentinel errors from `internal/service/sensor_service.go`: `ErrInvalidParameter`, `ErrDeviceNotFound`, `ErrReadingNotFound`
- Wrap with `fmt.Errorf("%w: context", sentinelErr)`
- Check with `errors.Is(err, service.ErrXxx)` in handler
- Map to HTTP codes in handler only: BadRequest, NotFound, InternalServerError

---

## Response Format

- Success: `{"data": ..., "meta": {"count": N, ...}}`
- Error: `{"error": {"code": "DEVICE_NOT_FOUND", "message": "...", "timestamp": "..."}}`
- Always set response headers: `X-Cache-Status`, `X-Response-Time`, `X-Request-ID`, `Content-Type: application/json`

---

## Database

- All SQL must use parameterized queries (`$1, $2, ...`) via pgx/v5
- Never concatenate user input into SQL
- Always use context with timeout (5s per query): `ctx, cancel := context.WithTimeout(ctx, 5*time.Second)`
- ID conversion: DB stores `int64`, JSON returns `string` via `fmt.Sprintf("%d", id)`

---

## Cache

- Cache-aside pattern in service layer (not repository)
- Stats endpoint bypasses cache — reads MV directly, returns `"BYPASS"` status
- TTL is 30s. Graceful degradation if Redis is down.
- Cache key format: `sensor:{device_id}:readings:{limit}[:{reading_type}][:{from_unix}][:{to_unix}]`
- PK cache key: `sensor:id:{id}`

---

## Configuration

- All config via environment variables loaded through `internal/config/config.go` (godotenv)
- Never hardcode connection strings, ports, or credentials
- Only `DATABASE_URL` is required — everything else has sensible defaults

---

## Testing

- Load/benchmark testing is via **k6** (not Vegeta — this project has never used Vegeta)
- Go unit tests do not exist yet — use table-driven pattern with `t.Run()` when adding them
- Run benchmarks: `./tests/run-benchmarks.sh --tier smoke`

---

## Implementation Rules

- Never make changes outside the stated scope of the current task
- Never delete, rename, or overwrite files without explicit user instruction
- Never introduce a new dependency, modify the database schema, or make an architectural decision without surfacing the proposal to the user and receiving explicit confirmation
- Follow patterns in `CODING_STANDARDS.md` — do not introduce new patterns without logging in `state/DECISIONS_LOG.md`
- **Zero-regression rule**: any change that causes `go build ./cmd/api` to fail or a previously passing test to fail must be flagged immediately — do not push forward past a regression

---

## Security Rules — Non-Negotiable

- Never write code that stores, logs, or exposes secrets, tokens, or credentials in any form — not in source code, not in test fixtures, not in log output
- Always validate and sanitize all external input at the **handler boundary layer** (`internal/handler/`) before it reaches any business logic
- Never implement an auth bypass "to be fixed later" — incomplete auth is a blocker, not a deferrable item
- Before adding any dependency, check for known vulnerabilities using `go list -json -m all | nancy sleuth` or equivalent, and document the check in `state/DECISIONS_LOG.md`
- Consult `SECURITY_STANDARDS.md` before implementing any feature involving input handling, authentication, external services, or data storage
- If a security vulnerability is discovered in existing code during any session, flag it to the user immediately before proceeding with the current task

---

## Environment Awareness Rules

- Always identify the active environment before running any command
- This project currently operates in **development only** — there is no staging or production environment configured
- If a staging or production environment is ever added: present a written plan and receive explicit confirmation before executing any change, migration, or destructive operation
- Never expose debug ports, seed scripts, or development tooling in production configuration
- Verify `.env` is properly gitignored before the first commit of any session
- Consult `ENVIRONMENT_GUIDE.md` when in doubt about environment-specific behavior

---

## Session End — Mandatory Before Closing

- Update `state/CURRENT_STATUS.md` with accurate current state and a session summary
- Update `state/TASK_QUEUE.md` — mark completed tasks, add newly discovered tasks
- Log any significant decision in `state/DECISIONS_LOG.md`
- Update `CODING_STANDARDS.md` if new patterns were established or existing ones were corrected
- Update `SECURITY_STANDARDS.md` if new security findings or patterns were identified
- Update `ENVIRONMENT_GUIDE.md` if environment configuration changed
- Update `README.md` if project-level context changed materially

---

## Self-Maintenance Directive

- The `.claude/` files must stay accurate at all times — they are not set-and-forget
- If a convention in `CODING_STANDARDS.md` is found to be wrong or outdated, correct it immediately and log the change in `DECISIONS_LOG.md`
- If the project state in `CURRENT_STATUS.md` is stale, update it before proceeding

---

## Escalation

When blocked, uncertain about scope, or facing a decision with significant architectural, security, or UX impact: document the blocker in `CURRENT_STATUS.md` and ask the user. Do not assume and proceed.
