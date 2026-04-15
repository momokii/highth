# Security Standards — Higth Project

Security audit findings and standing requirements for all future code changes.

---

## Current Security Posture

This is a **portfolio project** demonstrating high-performance query patterns. It runs locally and is not exposed to the internet. Several security controls are intentionally omitted for scope reasons.

**Overall assessment: YELLOW** — minor issues acceptable for portfolio scope, but must be addressed before any production deployment.

---

## Audit Findings (2026-04-15)

### Accepted by Design (Portfolio Scope)

These issues exist and are accepted for the portfolio project scope:

| Finding | Severity | Notes |
|---------|----------|-------|
| No authentication/authorization | Critical | No auth middleware. All endpoints publicly accessible within network. |
| No rate limiting | High | API vulnerable to DoS within network. |
| No CORS configuration | Medium | No CORS headers set on responses. |
| No security headers | Medium | Missing X-Content-Type-Options, X-Frame-Options, CSP, HSTS. |
| sslmode=disable on DB connections | High | All database traffic unencrypted within Docker network. |
| Docker runs as root | High | WORKDIR /root/ in Dockerfile. No non-root user configured. |
| No .dockerignore file | Medium | Risk of .env or sensitive files being included in image. |
| Redis has no password | Medium | Redis accessible without authentication within Docker network. |
| Weak default passwords | Medium | .env.example uses `sensor_password`, Grafana uses `admin/admin`. |

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
