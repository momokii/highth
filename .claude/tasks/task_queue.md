# Master Task Queue

This document lists all tasks in dependency order. Each task links to its detailed definition in a phase-specific file.

---

## Task Legend

- **Status Codes:**
  - `pending` - Not started, dependencies not met
  - `ready` - Not started, dependencies met, ready to execute
  - `in_progress` - Currently being worked on
  - `completed` - Finished and verified
  - `blocked` - Cannot proceed due to blocker

- **Priority:**
  - `P0` - Must complete (critical path)
  - `P1` - Should complete (important)
  - `P2` - Nice to have (optional)

---

## Phase 0: Environment & Tooling (8 tasks, 2-3 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-001 | Verify Go 1.21+ installation | ready | None | P0 | 5 min |
| TASK-002 | Verify Docker installation | ready | TASK-001 | P0 | 10 min |
| TASK-003 | Verify Docker Compose | ready | TASK-002 | P0 | 5 min |
| TASK-004 | Verify PostgreSQL client (psql) | ready | None | P0 | 10 min |
| TASK-005 | Verify Redis client (redis-cli) | ready | None | P0 | 10 min |
| TASK-006 | Install Vegeta | ready | TASK-001 | P0 | 15 min |
| TASK-007 | Create project directory structure | ready | TASK-001 | P0 | 10 min |
| TASK-008 | Initialize go.mod and .env.example | ready | TASK-007 | P0 | 15 min |

**Phase 0 Complete When:** All 8 tasks completed, all tools verified

**See:** `.claude/tasks/phase_0_environment.md` for detailed task definitions

---

## Phase 1: Database Provisioning (10 tasks, 2-4 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-009 | Create Docker Compose configuration | pending | TASK-008 | P0 | 20 min |
| TASK-010 | Start PostgreSQL container | pending | TASK-009 | P0 | 10 min |
| TASK-011 | Create database schema SQL file | pending | TASK-010 | P0 | 20 min |
| TASK-012 | Create sensor_db database | pending | TASK-011 | P0 | 5 min |
| TASK-013 | Create sensor_readings table | pending | TASK-012 | P0 | 10 min |
| TASK-014 | Create BRIN index on timestamp | pending | TASK-013 | P0 | 5 min |
| TASK-015 | Create composite B-tree index | pending | TASK-014 | P0 | 5 min |
| TASK-016 | Create covering index with INCLUDE | pending | TASK-015 | P0 | 10 min |
| TASK-017 | Run ANALYZE on table | pending | TASK-016 | P0 | 5 min |
| TASK-018 | Verify database connection | pending | TASK-017 | P0 | 5 min |

**Phase 1 Complete When:** PostgreSQL 16+ running, database created, table created with 3 indexes, ANALYZE run

**See:** `.claude/tasks/phase_1_database.md` for detailed task definitions

---

## Phase 2: Data Generation (5 tasks, 1-3 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-019 | Create data generation script | pending | TASK-018 | P0 | 30 min |
| TASK-020 | Configure Zipf distribution parameters | pending | TASK-019 | P0 | 20 min |
| TASK-021 | Run data generation (50M rows) | pending | TASK-020 | P0 | 1-2 hours |
| TASK-022 | Verify row count and distribution | pending | TASK-021 | P0 | 15 min |
| TASK-023 | Run ANALYZE after data load | pending | TASK-022 | P0 | 5 min |

**Phase 2 Complete When:** 50M rows inserted, distribution verified (non-uniform), ANALYZE run

**See:** `.claude/tasks/phase_2_data_generation.md` for detailed task definitions

---

## Phase 3: API Development (10 tasks, 4-8 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-024 | Create model package (data structures) | pending | TASK-018 | P0 | 30 min |
| TASK-025 | Create config package (configuration) | pending | TASK-024 | P0 | 30 min |
| TASK-026 | Create repository package (database) | pending | TASK-025 | P0 | 60 min |
| TASK-027 | Create cache package (Redis wrapper) | pending | TASK-025 | P0 | 45 min |
| TASK-028 | Create service package (business logic) | pending | TASK-026, TASK-027 | P0 | 60 min |
| TASK-029 | Create handler package (HTTP) | pending | TASK-028 | P0 | 60 min |
| TASK-030 | Create cmd/api/main.go (entry point) | pending | TASK-029 | P0 | 30 min |
| TASK-031 | Implement request validation | pending | TASK-029 | P0 | 30 min |
| TASK-032 | Implement error handling | pending | TASK-029 | P0 | 30 min |
| TASK-033 | Test API endpoints manually | pending | TASK-030 | P0 | 30 min |

**Phase 3 Complete When:** API runs on port 8080, `/api/v1/sensor-readings` works, `/health` works, error handling implemented

**See:** `.claude/tasks/phase_3_api_development.md` for detailed task definitions

---

## Phase 4: Cache Integration (5 tasks, 1-2 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-034 | Start Redis container | pending | TASK-009 | P0 | 5 min |
| TASK-035 | Implement write-through cache logic | pending | TASK-034, TASK-027 | P0 | 30 min |
| TASK-036 | Configure 30s TTL | pending | TASK-035 | P0 | 15 min |
| TASK-037 | Implement graceful degradation | pending | TASK-036 | P0 | 30 min |
| TASK-038 | Test cache hit/miss behavior | pending | TASK-037 | P0 | 30 min |

**Phase 4 Complete When:** Redis running, cache integration working, 30s TTL functioning, graceful degradation working

**See:** `.claude/tasks/phase_4_cache_integration.md` for detailed task definitions

---

## Phase 5: Load Testing (9 tasks, 2-4 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-039 | Create test-runner.sh script | pending | TASK-038 | P0 | 30 min |
| TASK-040 | Create test scenarios config | pending | TASK-039 | P0 | 30 min |
| TASK-041 | Run health check test | pending | TASK-040 | P0 | 5 min |
| TASK-042 | Run cold start test | pending | TASK-041 | P0 | 10 min |
| TASK-043 | Run baseline test | pending | TASK-042 | P0 | 10 min |
| TASK-044 | Run concurrent test (primary) | pending | TASK-043 | P0 | 15 min |
| TASK-045 | Run hot device test | pending | TASK-044 | P0 | 10 min |
| TASK-046 | Run large N test | pending | TASK-045 | P0 | 10 min |
| TASK-047 | Analyze test results | pending | TASK-046 | P0 | 30 min |

**Phase 5 Complete When:** All 6 scenarios executed, results saved, pass/fail determined

**See:** `.claude/tasks/phase_5_load_testing.md` for detailed task definitions

---

## Phase 6: Results Analysis (3 tasks, 1-2 hours)

| ID | Task | Status | Dependencies | Priority | Est. Time |
|----|------|--------|--------------|----------|-----------|
| TASK-048 | Create performance report | pending | TASK-047 | P0 | 45 min |
| TASK-049 | Document conclusions | pending | TASK-048 | P0 | 30 min |
| TASK-050 | Document recommendations | pending | TASK-049 | P0 | 30 min |

**Phase 6 Complete When:** Performance report created, conclusions documented, recommendations provided

**See:** `.claude/tasks/phase_6_results_analysis.md` for detailed task definitions

---

## Summary

- **Total Tasks:** 50
- **Total Estimated Time:** 13-26 hours
- **Critical Path:** TASK-001 → TASK-008 → TASK-009 → TASK-018 → TASK-023 → TASK-030 → TASK-033 → TASK-038 → TASK-044 → TASK-050

---

## Next Task

**Current Next Task:** TASK-001 (Verify Go 1.21+ installation)

**To start, read:** `.claude/tasks/phase_0_environment.md`

---

## Task Status by Phase

```
Phase 0: Environment & Tooling    [░░░░░░░░] 0/8   (0%)
Phase 1: Database Provisioning    [░░░░░░░░] 0/10  (0%)
Phase 2: Data Generation          [░░░░░░░░] 0/5   (0%)
Phase 3: API Development          [░░░░░░░░] 0/10  (0%)
Phase 4: Cache Integration        [░░░░░░░░] 0/5   (0%)
Phase 5: Load Testing             [░░░░░░░░] 0/9   (0%)
Phase 6: Results Analysis         [░░░░░░░░] 0/3   (0%)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total Progress:                   [░░░░░░░░] 0/50  (0%)
```

---

**Document Version:** 1.0
**Last Updated:** 2026-03-11
