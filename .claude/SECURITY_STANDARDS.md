# Security Standards — Higth Project

Security audit findings and standing requirements for all future code changes.

---

## Current Security Posture

This is a **portfolio project** demonstrating high-performance query patterns. It runs locally and is not exposed to the internet. Several security controls are intentionally omitted for scope reasons.

**Overall assessment: YELLOW** — minor issues acceptable for portfolio scope, but must be addressed before any production deployment.

---

## Audit Findings (2026-04-17)

### Accepted by Design (Portfolio Scope)

These issues exist and are accepted for the portfolio project scope:

| Finding | Severity | Notes |
|---------|----------|-------|
| No authentication/authorization | Critical | No auth middleware. All endpoints publicly accessible within network. |
| No rate limiting | High | API vulnerable to DoS within network. |
| No CORS configuration | Medium | No CORS headers set on responses. |
| No security headers | Medium | Missing X-Content-Type-Options, X-Frame-Options, CSP, HSTS. |
| sslmode=disable on DB connections | High | All database traffic unencrypted within Docker network. |
| Docker runs as root | ~~High~~ **RESOLVED** | Now runs as `appuser` (UID 1000) with `USER` directive in Dockerfile. |
| No .dockerignore file | ~~Medium~~ **RESOLVED** | `.dockerignore` created excluding `.env`, `.git`, docs, test-results. |
| Redis has no password | Medium | Redis accessible without authentication within Docker network. |
| Weak default passwords | ~~Medium~~ **RESOLVED** | Replaced with `CHANGE_ME_POSTGRES_PASSWORD` and `CHANGE_ME_GRAFANA_PASSWORD` placeholders. |
| Monitoring ports exposed to 0.0.0.0 | Medium | Prometheus (9090), Grafana (3000), exporters (9187, 9121) bound to all interfaces. Should be `127.0.0.1` only. |

### Security Strengths (Preserve These)

| Strength | Location |
|----------|----------|
| SQL injection protection | `internal/repository/sensor_repo.go` — all queries use `$N` parameterized placeholders |
| Input validation | `internal/service/sensor_service.go` — device ID regex, reading type alphanumeric, limit 1-500, positive integer for ID |
| Mutual exclusivity check | `internal/handler/sensor_handler.go` — `id` and `device_id` are mutually exclusive |
| Time range validation | `internal/handler/sensor_handler.go` — `from` must be <= `to` |
| Dependencies pinned | `go.mod` — all dependencies pinned to exact versions |
| .env gitignored | `.gitignore` — `.env` is excluded from version control |
| Error messages sanitized | Handler returns generic "An unexpected error occurred" for internal errors, never stack traces |
| Request ID tracking | `X-Request-ID` header on all responses for audit trail |

---

## Docker & Container Security

### Current Findings

| Finding | Severity | Detail |
|---------|----------|--------|
| API container runs as root | ~~High~~ **RESOLVED** | Dockerfile now uses `appuser` (UID 1000) with `USER` directive and `COPY --chown`. |
| No .dockerignore | ~~Medium~~ **RESOLVED** | `.dockerignore` created excluding `.env`, `.git`, docs, test-results, `.claude/`. |
| Monitoring ports on 0.0.0.0 | Medium | Prometheus `9090:9090`, Grafana `3000:3000`, postgres-exporter `9187:9187`, redis-exporter `9121:9121` all bound to all interfaces. |
| DB/Redis ports on 0.0.0.0 | Medium | PostgreSQL `5434:5432`, Redis `6379:6379` exposed to all interfaces. Should be `127.0.0.1` only for local dev. |
| sslmode=disable | High | All DATABASE_URL connections use `sslmode=disable`. Unencrypted within Docker network. |
| Weak fallback passwords | Medium | `docker-compose.yml` fallbacks: `sensor_password` for PostgreSQL, `admin` for Grafana. |

### Requirements for Future Docker Changes

- All future Dockerfile changes must maintain or improve on the existing posture
- When adding new services, do not expose ports to `0.0.0.0` — use `127.0.0.1` binding
- When creating new images, include a `USER` directive for non-root execution

---

## Environment Configuration

### Current State

This project operates in **development only**. There is no `APP_ENV` variable, no staging configuration, and no production configuration.

- Development: Docker Compose with debug-friendly defaults, verbose logging, seed scripts
- Staging: **Not configured**
- Production: **Not configured**

### Secrets & Environment Variable Management

- Never hardcode secrets, API keys, tokens, passwords, or any sensitive value in source code — not even in test files or fixtures
- All secrets must be managed via environment variables loaded from `.env` files excluded from version control via `.gitignore`
- A `.env.example` file exists at the root with all required variable names and placeholder values — this is committed to the repository
- The agent must never log, print, or expose environment variable values in output, error messages, or debug statements

---

## Input Validation & Sanitization

### Boundary Layer

The boundary layer for this project is the **handler layer** (`internal/handler/`). All external input must be validated and sanitized at this layer before reaching any business logic in the service or repository layers.

### Validation Rules (from `internal/service/sensor_service.go` and `internal/handler/sensor_handler.go`)

| Input | Validation | Location |
|-------|-----------|----------|
| `device_id` | Regex `^[a-zA-Z0-9_-]+$`, max 50 chars | Service layer |
| `reading_type` | Alphanumeric only, max 30 chars | Service layer |
| `limit` | Integer 1-500 | Handler layer |
| `id` | Positive integer (≥1) via `strconv.ParseInt` | Handler layer |
| `from` / `to` | RFC3339 format, `from <= to` | Handler layer |
| Mutual exclusivity | `id` and `device_id` cannot both be provided; exactly one required | Handler layer |

- Never trust client-supplied data for authorization decisions
- All new query parameters must be validated in the handler before reaching service layer

---

## Authentication & Authorization

### Current Approach

**None.** This is a portfolio project with no authentication or authorization. All API endpoints are publicly accessible within the Docker network.

### Requirements for Future Auth Work

- All protected routes must enforce auth checks — default deny posture
- Never implement auth bypasses deferrable to later
- See `SECURITY_STANDARDS.md` Production Readiness Checklist for auth requirements

---

## Dependency Security

### Current Approach

All dependencies are pinned to exact versions in `go.mod`:
- `go-chi/chi/v5` v5.2.5
- `jackc/pgx/v5` v5.8.0
- `joho/godotenv` v1.5.1
- `prometheus/client_golang` v1.23.2
- `redis/go-redis/v9` v9.18.0

No automated vulnerability scanning (no Dependabot, Renovate, or Snyk configured).

### Requirements for New Dependencies

- Pin all dependency versions in `go.mod`
- Check for known vulnerabilities before adding any new package: `go list -json -m all | nancy sleuth` or equivalent
- Log the check result in `state/DECISIONS_LOG.md`

---

## Rules for Future Code Changes

### SQL and Database
- Never introduce string concatenation into SQL queries. Always use `$N` placeholders via pgx/v5.
- Always add `context.WithTimeout` (5s) for database queries.
- Never log `DATABASE_URL` or connection strings.

### Input Handling
- Validate all new query parameters in the handler before reaching service layer.
- Device IDs: regex `^[a-zA-Z0-9_-]+$`, max 50 chars.
- Reading types: alphanumeric only, max 30 chars.
- Numeric IDs: must be positive integers (validate with `strconv.ParseInt`).
- Time ranges: RFC3339 format, `from <= to`.

### Dependencies
- Pin all dependency versions in `go.mod`.
- Check for known vulnerabilities before adding any new package (`go list -json -m all | nancy sleuth` or similar).
- Log the check result in `state/DECISIONS_LOG.md`.

### Secrets
- Never hardcode secrets, API keys, tokens, or passwords in source code — not even in tests.
- All secrets via environment variables loaded from `.env` (gitignored).
- Never log environment variable values.

---

## Production Readiness Checklist

Before exposing this system beyond localhost:

- [ ] Implement authentication (JWT or API key)
- [ ] Add rate limiting middleware
- [ ] Add CORS middleware with restrictive origins
- [ ] Add security headers (X-Content-Type-Options, X-Frame-Options, CSP, HSTS)
- [ ] Enable SSL on DB connections (`sslmode=require` or `verify-full`)
- [ ] Configure Redis AUTH with strong password
- [ ] Run Docker as non-root user (add `USER` directive to Dockerfile)
- [ ] Create `.dockerignore` excluding `.env`, `.git`, docs, test-results
- [ ] Replace default passwords with strong random values
- [ ] Add Nginx reverse proxy for SSL termination
- [ ] Bind monitoring ports to localhost only (`127.0.0.1:9090:9090`)
- [ ] Disable Grafana anonymous access (already done: `GF_AUTH_ANONYMOUS_ENABLED: "false"`)
