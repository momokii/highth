# Agent Continuity Infrastructure

**Purpose:** This directory contains all information needed for an AI agent to continue work on this project across multiple sessions and machines.

---

## Quick Start for New Agents

1. **Read `HOW_TO_RESUME.md`** - Learn how to start working
2. **Read `AGENT_MANIFEST.md`** - Understand current project state
3. **Read `CODING_STANDARDS.md`** - Learn architecture patterns (CRITICAL)
4. **Check `state/progress.json`** - See what's done/in-progress/blocked
5. **Pick next task from `tasks/task_queue.md`**

---

## File Organization

```
.claude/
├── README.md                    # This file - entry point
├── AGENT_MANIFEST.md            # Single source of truth for project state
├── CODING_STANDARDS.md          # Architecture patterns and conventions
├── HOW_TO_RESUME.md             # Complete resume guide
│
├── tasks/                       # Task definitions and queue
│   ├── task_queue.md            # Master ordered list (50 tasks)
│   ├── phase_0_environment.md   # Phase 0 tasks
│   ├── phase_1_database.md      # Phase 1 tasks
│   ├── phase_2_data_generation.md
│   ├── phase_3_api_development.md
│   ├── phase_4_cache_integration.md
│   ├── phase_5_load_testing.md
│   └── phase_6_results_analysis.md
│
├── state/                       # Progress tracking (git-ignored)
│   ├── progress.json            # Current progress state (machine-readable)
│   ├── session_history.md       # Session-by-session log
│   └── blockers.md              # Active blockers with resolutions
│
└── templates/                   # File templates for consistency
    ├── go_handler_template.go
    ├── go_service_template.go
    ├── go_repository_template.go
    ├── go_model_template.go
    ├── go_config_template.go
    ├── docker_compose_template.yml
    └── test_runner_template.sh
```

---

## Critical Rules

1. **ALWAYS update `state/progress.json`** after completing work
2. **ALWAYS read `CODING_STANDARDS.md`** before writing code
3. **ALWAYS check dependencies** before starting a task
4. **NEVER skip verification steps**
5. **ALWAYS document blockers** in `state/blockers.md`

---

## Project Overview

**Name:** High-Performance IoT Sensor Query System
**Type:** Portfolio-grade backend performance demonstration
**Target:** p50 <= 500ms, p95 <= 800ms at 50M rows
**Tech Stack:** Go 1.21+, PostgreSQL 16+, Redis 7+, chi router, pgx, Vegeta
**Implementation:** 6 phases, 50 tasks, 13-26 hours estimated

---

## Current Status

Check `AGENT_MANIFEST.md` for:
- Phase progress (0-100% for each of 6 phases)
- Current task (what to work on next)
- Environment state (tools installed, containers running)
- Active blockers
- Session history

---

## Task Queue

The `tasks/task_queue.md` file contains all 50 tasks ordered by dependency:
- TASK-001 through TASK-008: Phase 0 (Environment & Tooling)
- TASK-009 through TASK-018: Phase 1 (Database Provisioning)
- TASK-019 through TASK-023: Phase 2 (Data Generation)
- TASK-024 through TASK-033: Phase 3 (API Development)
- TASK-034 through TASK-038: Phase 4 (Cache Integration)
- TASK-039 through TASK-047: Phase 5 (Load Testing)
- TASK-048 through TASK-050: Phase 6 (Results Analysis)

Each task has:
- ID and title
- Status (pending/ready/in_progress/completed/blocked)
- Dependencies (what must exist first)
- Output definition (what "done" looks like)
- Verification commands
- Estimated time

---

## Progress Tracking

### `state/progress.json` (Machine-Readable)

```json
{
  "version": "1.0",
  "last_updated": "ISO-8601 timestamp",
  "current_phase": "Phase X: Name",
  "phase_progress": { /* 6 phases */ },
  "tasks": { /* TASK-001 through TASK-050 */ },
  "blockers": [],
  "next_task": "TASK-XXX"
}
```

### `state/session_history.md` (Human-Readable)

Logs each session with:
- Date/time and agent ID
- Tasks completed
- Issues encountered
- Time spent

### `state/blockers.md`

Tracks:
- Active blockers with proposed resolutions
- Resolved blockers with outcomes

---

## Documentation Reference

**Existing Project Documentation:**
- `docs/README.md` - Project overview
- `docs/architecture.md` - Database schema, indexing, caching
- `docs/api-spec.md` - API contract
- `docs/implementation/plan.md` - 6-phase implementation plan

**Agent Infrastructure (this folder):**
- `CODING_STANDARDS.md` - Architecture patterns, naming conventions
- `HOW_TO_RESUME.md` - Resume guide for new/resuming agents

---

## Before Starting Any Task

1. Read the task file completely (understand what "done" looks like)
2. Check dependencies (ensure prerequisite tasks are complete)
3. Verify environment (run the phase's verification commands)
4. Check for blockers (read `state/blockers.md`)

---

## After Completing Any Task

1. Run verification commands (ensure everything works)
2. Update `state/progress.json` (mark task as completed)
3. Update `AGENT_MANIFEST.md` (update phase progress)
4. Log the session in `state/session_history.md`
5. Document any issues in `state/blockers.md` (if needed)

---

## Getting Unstuck

If you encounter a blocker:

1. Re-read the documentation (answer is usually there)
2. Check the task details (look for troubleshooting sections)
3. Review session history (see how similar issues were resolved)
4. Document the blocker (create entry in `state/blockers.md`)
5. Ask the user (if you can't proceed, explain clearly)

---

**Infrastructure Version:** 1.0
**Last Updated:** 2026-03-11
**Compatible With:** Claude Code agents on any platform
