# How to Resume — Higth Project

Step-by-step protocol for agents starting or resuming work on this project.

---

## Step 1: Read `.claude/README.md`
Orient yourself: understand the project, stack, architecture, and file organization.

## Step 2: Read `.claude/state/CURRENT_STATUS.md`
Know exactly what is done, in progress, and blocked.

## Step 3: Read `.claude/state/TASK_QUEUE.md`
Identify the next task and confirm its dependencies are met.

## Step 4: Read `.claude/AGENT_RULES.md`
Re-internalize all behavioral rules before touching anything.

## Step 5: Read `.claude/CODING_STANDARDS.md`
Re-internalize all conventions before writing any code.

## Step 6: Read `.claude/SECURITY_STANDARDS.md`
Re-internalize all security requirements before writing any code.

## Step 7: Identify the active environment
This project operates in **development only**. If staging or production is ever added, consult `ENVIRONMENT_GUIDE.md` for environment-specific behavior.

## Step 8: Read task-relevant docs
PRD section, architecture doc, API contract, or any doc directly relevant to the current task. Key docs:
- API spec: `docs/api-spec.md`
- Architecture: `docs/architecture.md`
- Testing: `docs/testing.md`
- Future enhancements: `docs/future-enhancements/`

## Step 9: Verify the environment is functional

```bash
# Check services are running
docker compose ps

# Health check (DB + Redis with latency)
curl -s http://localhost:8080/health | jq .

# If services aren't running, start them:
# docker network create highth-network 2>/dev/null
# docker compose up -d --build
```

See `ENVIRONMENT_GUIDE.md` for full setup commands.

## Step 10: Confirm no regressions

```bash
# Build must succeed before writing any code
go build ./cmd/api
```

If this fails, diagnose and fix before proceeding with any task.

## Step 11: Begin the task
Implement → test → security review → update `.claude/` state files → report to user.

---

## Working on an Existing Task

1. Find the task in `state/TASK_QUEUE.md` — read its full description and acceptance criteria
2. Check dependencies — ensure prerequisite tasks are complete
3. Follow the relevant template:
   - New feature → `templates/new_feature.md`
   - New endpoint → `templates/new_endpoint.md`
   - New test → `templates/new_test.md`
   - Bug fix → `templates/bug_fix.md`
4. Implement, verify, update state files

---

## After Completing Work

1. Verify: `go build ./cmd/api` succeeds
2. Run smoke benchmark if relevant: `./tests/run-benchmarks.sh --tier smoke`
3. Update `state/CURRENT_STATUS.md` with what changed and new date
4. Update `state/TASK_QUEUE.md` — mark task as done
5. Log decisions in `state/DECISIONS_LOG.md` if significant
6. Update `CODING_STANDARDS.md` if new patterns were established
7. Update `SECURITY_STANDARDS.md` if new security findings emerged
8. Update `ENVIRONMENT_GUIDE.md` if environment config changed

---

## Quick Reference Commands

```bash
# Start environment
docker network create highth-network 2>/dev/null
docker compose up -d --build

# Run migrations
./scripts/run_migrations.sh

# Build
go build -o bin/api ./cmd/api

# Health check
curl -s http://localhost:8080/health

# Smoke benchmark
./tests/run-benchmarks.sh --tier smoke

# Generate data
python3 scripts/generate_data_fast.py
```

Full command reference: `ENVIRONMENT_GUIDE.md`

---

## If Something Is Broken

1. Check `state/CURRENT_STATUS.md` for known issues
2. Re-read the relevant documentation in `docs/`
3. Check Docker services: `docker compose ps`, `docker compose logs api`
4. Check database connectivity: `docker exec -it highth-postgres psql -U sensor_user -d sensor_db -c "SELECT 1"`
5. Check Redis: `docker exec -it highth-redis redis-cli ping`
6. Document the blocker in `state/CURRENT_STATUS.md` and ask the user
