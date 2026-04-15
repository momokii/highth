# How to Resume — Higth Project

Protocol for agents starting or resuming work on this project.

---

## New Session (3 minutes)

1. Read `state/CURRENT_STATUS.md` — understand what is done and what remains
2. Read `state/TASK_QUEUE.md` — find open work items
3. Read `AGENT_RULES.md` — re-internalize behavioral rules before touching code
4. Read `CODING_STANDARDS.md` — re-internalize conventions before writing code
5. Verify environment:

```bash
# Check services are running
docker compose ps

# Health check
curl -s http://localhost:8080/health | jq .

# Build check
go build ./cmd/api
```

If services aren't running, see `ENVIRONMENT_GUIDE.md` for setup commands.

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
3. Update `state/CURRENT_STATUS.md` with what changed
4. Update `state/TASK_QUEUE.md` — mark task as done
5. Log decisions in `state/DECISIONS_LOG.md` if significant

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
