# Agent Infrastructure — Higth Project

Go API querying 50M+ IoT sensor readings through PostgreSQL + Redis with sub-500ms latency.
Module: `github.com/kelanach/higth`. Status: ~95% complete, maintenance/enhancement phase.

---

## Orientation Sequence

Read these files **in order** before starting any work:

1. **This file** — project overview and file map
2. **`AGENT_RULES.md`** — non-negotiable behavioral rules
3. **`CODING_STANDARDS.md`** — patterns and conventions derived from actual codebase
4. **`state/CURRENT_STATUS.md`** — what is done, what remains
5. **`state/TASK_QUEUE.md`** — open work items

---

## File Organization

```
.claude/
  README.md              # This file — entry point for agents
  AGENT_RULES.md          # Non-negotiable rules for every session
  CODING_STANDARDS.md     # Patterns and conventions (derived from real code)
  SECURITY_STANDARDS.md   # Security audit findings + requirements
  ENVIRONMENT_GUIDE.md    # Verified commands for all operations
  HOW_TO_RESUME.md        # Resume protocol for new sessions
  settings.json           # Tool permissions (git-tracked)
  settings.local.json     # Local plugin settings
  state/
    CURRENT_STATUS.md     # What is done, in progress, blocked
    TASK_QUEUE.md         # Prioritized backlog of work items
    DECISIONS_LOG.md      # Key architectural decisions
    .gitkeep              # Ensures directory exists in git
  templates/
    new_feature.md        # Feature implementation checklist
    new_endpoint.md       # API endpoint checklist
    new_test.md           # Test creation checklist
    bug_fix.md            # Bug investigation and fix checklist
```

**State files** (`state/*.md`) are git-ignored — they are session-specific and updated after every session.
**Config files** (everything else) are git-tracked — they change rarely and reflect stable project conventions.

---

## Architecture

```
Request → Handler → Service → Repository → PostgreSQL
                    ↓
                   Cache (Redis, cache-aside)
```

- **Handler** (`internal/handler/`): HTTP concerns only — request parsing, response formatting, status codes
- **Service** (`internal/service/`): Business logic, cache orchestration, input validation
- **Repository** (`internal/repository/`): PostgreSQL queries only (pgx/v5, parameterized)
- **Cache** (`internal/cache/`): Redis operations (go-redis/v9, 30s TTL, LRU eviction)
- **Config** (`internal/config/`): Environment variable loading via godotenv

Full details: `docs/architecture.md`

---

## Key Facts

| Fact | Detail |
|------|--------|
| Module path | `github.com/kelanach/higth` ("higth" not "highth") |
| Go version | 1.25.7 |
| Testing tool | k6 (via Docker) — **not Vegeta** |
| Database | PostgreSQL 16 with BRIN + covering indexes, materialized views |
| Cache | Redis 7, cache-aside in service layer, 30s TTL |
| Migration gap | No 003 migration — gap is intentional |
| Unit tests | None — all testing is k6 benchmarks |
| Auth | None — portfolio project (by design) |

---

## Key Documentation

| Doc | Path | Content |
|-----|------|---------|
| API Spec | `docs/api-spec.md` | All endpoints with request/response examples |
| Architecture | `docs/architecture.md` | Schema design, indexing strategy, caching |
| Testing | `docs/testing.md` | Test plan and scenario descriptions |
| Future Enhancements | `docs/future-enhancements/` | Planned features with specs |
| Quick Start | `README.md` (project root) | 15-minute setup guide |
| Agent Context | `CLAUDE.md` (project root) | Auto-discovered lightweight project reference |
