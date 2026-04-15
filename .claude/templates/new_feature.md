# New Feature Checklist — Higth

## Before Starting
- [ ] Task exists in `state/TASK_QUEUE.md` with clear acceptance criteria
- [ ] Read `CODING_STANDARDS.md` for patterns and conventions
- [ ] Read `SECURITY_STANDARDS.md` for security requirements
- [ ] Verify environment: `go build ./cmd/api` succeeds
- [ ] Confirm active environment is development

## Design
- [ ] Scope defined — list every file to be created or modified
- [ ] Edge cases identified before implementation begins
- [ ] Security implications assessed
- [ ] If new dependency required: vulnerability check + user confirmation
- [ ] If schema change required: proposal submitted to user and confirmed

## Implementation (follow layer order)

1. **Model** (`internal/model/`) — Define data structures first
   - [ ] Structs follow existing naming (PascalCase, JSON tags)
   - [ ] Constants for enum-like values (see `ReadingType` pattern in `sensor.go`)

2. **Repository** (`internal/repository/`) — Database queries
   - [ ] Parameterized queries only (`$1`, `$2`, etc.)
   - [ ] Context with 5s timeout on all queries
   - [ ] ID conversion: int64 from DB → string for JSON

3. **Service** (`internal/service/`) — Business logic + cache
   - [ ] Input validation before any logic
   - [ ] Cache-aside pattern if applicable (check cache → query DB → populate cache)
   - [ ] Return `(cacheStatus, data, error)` tuple if caching
   - [ ] Graceful degradation if Redis is down

4. **Handler** (`internal/handler/`) — HTTP concerns
   - [ ] Request parsing and validation
   - [ ] Call service layer
   - [ ] Map service errors to HTTP status codes via `errors.Is()`
   - [ ] Set response headers: `X-Cache-Status`, `X-Response-Time`, `X-Request-ID`
   - [ ] Success response: `{"data": ..., "meta": ...}`
   - [ ] Error response: `{"error": {"code": "...", "message": "...", "timestamp": "..."}}`

5. **Route** (`cmd/api/main.go`) — Register route in chi router
   - [ ] Follow existing route pattern under `/api/v1/`

6. **Migration** (if needed) — `scripts/schema/migrations/`
   - [ ] Next number after 006 (i.e., `007_*.sql`)
   - [ ] Never renumber existing migrations

## Security Review
- [ ] No secrets hardcoded in new code
- [ ] All external input validated at handler layer
- [ ] No SQL concatenation — parameterized queries only
- [ ] No sensitive data in log output
- [ ] `.env.example` updated if new env vars introduced

## Testing
- [ ] Happy path verified manually with `curl`
- [ ] Error cases verified (invalid input, not found, service error)
- [ ] `go build ./cmd/api` succeeds
- [ ] Existing endpoints still work (regression check)

## Completion
- [ ] `state/CURRENT_STATUS.md` updated
- [ ] `state/TASK_QUEUE.md` — task marked done
- [ ] `state/DECISIONS_LOG.md` updated if significant decision made
