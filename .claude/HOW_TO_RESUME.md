# How to Resume Work

**Purpose:** Complete guide for agents starting or resuming work on this project

---

## For a Brand-New Agent (Zero Prior Context)

### Step 1: Read Project Overview (5 minutes)

```bash
cat docs/README.md
cat docs/architecture.md
cat docs/implementation/plan.md
```

**What you should understand:**
- High-performance IoT sensor query system
- Target: p50 ≤ 500ms at 50M rows
- 6 implementation phases (Environment → Database → Data → API → Cache → Testing)
- Tech stack: Go, PostgreSQL 16+, Redis 7+, chi router, pgx, Vegeta

### Step 2: Read the Agent Manifest (2 minutes)

```bash
cat .claude/AGENT_MANIFEST.md
```

**What you should understand:**
- Current phase progress
- What's been completed
- What's next
- Any active blockers

### Step 3: Read Coding Standards (10 minutes) **CRITICAL**

```bash
cat .claude/CODING_STANDARDS.md
```

**What you should understand:**
- Architecture: Handler → Service → Repository
- Package organization rules
- Go naming conventions
- Error handling patterns
- Code quality requirements

**DO NOT write code until you've read this document.**

### Step 4: Check Current Progress (1 minute)

```bash
cat .claude/state/progress.json
```

This shows:
- Completed tasks (with timestamps)
- In-progress task (if any)
- Blocked tasks (with reasons)
- Next task to work on

### Step 5: Pick Your Task (1 minute)

```bash
cat .claude/tasks/task_queue.md
```

Find the first task with status `ready` (dependencies met).

### Step 6: Read Task Details (2 minutes)

Each task has its own file with:
- Description
- Dependencies
- Output definition (what "done" looks like)
- Verification commands
- Estimated time

Example:
```bash
cat .claude/tasks/phase_0_environment.md
```

### Step 7: Execute the Task (varies)

Follow the task's implementation steps. When done:

1. Verify outputs match the task's "Output Definition"
2. Run all verification commands
3. Update `.claude/state/progress.json`
4. Update `.claude/AGENT_MANIFEST.md`
5. Log session in `.claude/state/session_history.md`

---

## For a Resuming Agent (After Interruption)

### Quick Resume (2 minutes)

```bash
# Check what was in progress
cat .claude/state/progress.json | grep '"in_progress"'

# Check for blockers
cat .claude/state/blockers.md

# Read the manifest
cat .claude/AGENT_MANIFEST.md

# Continue where you left off
```

### Detailed Resume (5 minutes)

If it's been a while:

1. **Re-read the manifest** - Understand current state
2. **Read session history** - See what happened in previous sessions
3. **Review coding standards** - Refresh your knowledge
4. **Check the task queue** - Find the next task
5. **Verify environment** - Ensure tools and containers are running

---

## Understanding the File Structure

### Task Files (`tasks/`)

Each phase has a task file with ordered tasks:

```markdown
# Phase 0: Environment Setup

## TASK-001: Verify Go 1.21+ Installation

**Status:** pending
**Dependencies:** None
**Estimated Time:** 5 minutes

**Description:**
Verify that Go 1.21 or later is installed and accessible in PATH.

**Steps:**
1. Run `go version`
2. Verify output shows go1.21.0 or higher
3. Verify `go` is in PATH with `which go`

**Output Definition:**
- Task is complete when `go version` returns go1.21.0+
- `which go` returns a valid path

**Verification Commands:**
```bash
go version | grep -o 'go[0-9.]*'
which go
```

**Next Task:** TASK-002 (Verify Docker installation)
```

### Progress Tracking (`state/progress.json`)

```json
{
  "version": "1.0",
  "last_updated": "2026-03-11T15:30:00Z",
  "current_phase": "Phase 0: Environment & Tooling",
  "phase_progress": {
    "phase_0": {
      "name": "Environment & Tooling",
      "status": "in_progress",
      "completion_percentage": 20,
      "total_tasks": 8,
      "completed_tasks": 1,
      "in_progress_task": "TASK-002"
    }
  },
  "tasks": {
    "TASK-001": {
      "id": "TASK-001",
      "title": "Verify Go 1.21+ installation",
      "status": "completed",
      "started_at": "2026-03-11T15:00:00Z",
      "completed_at": "2026-03-11T15:05:00Z"
    }
  },
  "next_task": "TASK-003"
}
```

### Session History (`state/session_history.md`)

```markdown
# Session History

## Session 1: 2026-03-11 15:00-15:30 UTC

**Agent:** reactive-splashing-bengio-agent-aede964
**Phase:** Phase 0: Environment & Tooling

**Completed Tasks:**
- TASK-001: Verified Go 1.21.6 installed

**In-Progress Tasks:**
- TASK-002: Verifying Docker installation

**Issues Encountered:**
- None

**Notes:**
- Environment setup proceeding smoothly

**Time Spent:** 30 minutes
```

---

## Verification Commands by Phase

### Phase 0: Environment

```bash
# Verify Go
go version | grep -o 'go[0-9.]*'

# Verify Docker
docker --version
docker compose version

# Verify client tools
psql --version
redis-cli --version

# Verify Vegeta
vegeta --version

# Verify project structure
ls -la cmd/api internal/{handler,service,repository,cache,model,config}
```

### Phase 1: Database

```bash
# Verify PostgreSQL running
docker ps | grep postgres

# Verify database exists
psql "$DATABASE_URL" -c "\l" | grep sensor_db

# Verify table exists
psql "$DATABASE_URL" -c "\dt sensor_readings"

# Verify indexes
psql "$DATABASE_URL" -c "\di idx_sensor_readings_ts_brin"
```

### Phase 2: Data Generation

```bash
# Verify row count
psql "$DATABASE_URL" -c "SELECT count(*) FROM sensor_readings"

# Verify distribution (should be non-uniform)
psql "$DATABASE_URL" -c "
SELECT percentile_cont(0.50) WITHIN GROUP (ORDER BY reading_count) as p50
FROM (SELECT device_id, count(*) as reading_count
      FROM sensor_readings GROUP BY device_id) counts;"
```

### Phase 3: API Development

```bash
# Verify API runs
curl -s http://localhost:8080/health

# Verify sensor endpoint
curl -s "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"

# Verify error handling
curl -s "http://localhost:8080/api/v1/sensor-readings?limit=10" | jq '.error.code'
```

### Phase 4: Cache Integration

```bash
# Verify Redis running
redis-cli ping

# Verify cache populates
redis-cli FLUSHALL
curl "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10" > /dev/null
redis-cli KEYS "sensor:*"

# Verify TTL
redis-cli TTL "sensor:sensor-001:readings:10"
```

### Phase 5: Load Testing

```bash
# Verify test results exist
ls test-results/

# Verify concurrent test passed
grep "Latencies" test-results/*/concurrent.txt
```

### Phase 6: Results Analysis

```bash
# Verify performance report exists
ls docs/results/performance-report.md
```

---

## Common Resume Scenarios

### Scenario 1: "Continue where I left off"

```bash
cat .claude/state/progress.json | grep "in_progress"
cat .claude/state/blockers.md
# Continue working
```

### Scenario 2: "It's been a few days, I need context"

```bash
cat .claude/AGENT_MANIFEST.md
cat .claude/CODING_STANDARDS.md
cat .claude/state/session_history.md
# Resume work
```

### Scenario 3: "Different machine, full setup needed"

```bash
# Clone repository
git clone <repo-url>
cd highth

# Read all documentation
cat .claude/README.md
cat .claude/HOW_TO_RESUME.md
cat .claude/AGENT_MANIFEST.md
cat .claude/CODING_STANDARDS.md

# Check progress
cat .claude/state/progress.json

# Resume from next task
```

### Scenario 4: "Something is broken"

```bash
# Check what was last working
cat .claude/state/session_history.md

# Check for blockers
cat .claude/state/blockers.md

# Re-run verification commands for current phase
```

---

## Before You Start Any Task

1. **Read the task file completely** - Understand what "done" looks like
2. **Check dependencies** - Ensure all prerequisite tasks are complete
3. **Verify environment** - Run the phase's verification commands
4. **Check for blockers** - Read `state/blockers.md`

---

## After You Complete Any Task

1. **Run verification commands** - Ensure everything works
2. **Update progress.json** - Mark task as completed
3. **Update AGENT_MANIFEST.md** - Update phase progress
4. **Log the session** - Add entry to `state/session_history.md`
5. **Document any issues** - Add to `state/blockers.md` if needed

---

## Getting Help

If you're stuck:

1. **Re-read the documentation** - The answer is usually there
2. **Check the task details** - Look for troubleshooting sections
3. **Review session history** - See how similar issues were resolved
4. **Document the blocker** - Create an entry in `state/blockers.md`
5. **Ask the user** - If you can't proceed, explain the situation clearly

---

**Document Version:** 1.0
**Last Updated:** 2026-03-11
