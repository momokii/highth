# Phase 0: Environment & Tooling Tasks

**Goal:** Establish a working development environment with all required tools installed and configured.

**Estimated Time:** 2-3 hours
**Total Tasks:** 8

---

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

**Expected Output:**
```
go version go1.21.6 linux/amd64
/usr/local/go/bin/go
```

**Troubleshooting:**
- If `go: command not found`: Install Go from https://go.dev/dl/
- If version < 1.21: Update Go to 1.21+

**Next Task:** TASK-002

---

## TASK-002: Verify Docker Installation

**Status:** pending
**Dependencies:** TASK-001
**Estimated Time:** 10 minutes

**Description:**
Verify Docker is installed and running.

**Steps:**
1. Run `docker --version`
2. Run `docker info` to verify daemon is running
3. Test Docker with hello-world: `docker run hello-world`

**Output Definition:**
- Docker version displayed
- Docker daemon running
- hello-world container runs successfully

**Verification Commands:**
```bash
docker --version
docker info | grep "Server Version"
docker run hello-world
```

**Expected Output:**
```
Docker version 24.0.7
Server Version: 24.0.7
Hello from Docker!
```

**Troubleshooting:**
- If daemon not running: `sudo systemctl start docker` (Linux) or start Docker Desktop (macOS)
- If permission denied: Add user to docker group: `sudo usermod -aG docker $USER`

**Next Task:** TASK-003

---

## TASK-003: Verify Docker Compose

**Status:** pending
**Dependencies:** TASK-002
**Estimated Time:** 5 minutes

**Description:**
Verify Docker Compose v2+ is installed.

**Steps:**
1. Run `docker compose version`
2. Verify v2+ is installed (not v1)

**Output Definition:**
- Docker Compose v2+ displayed

**Verification Commands:**
```bash
docker compose version
```

**Expected Output:**
```
Docker Compose version v2.23.0
```

**Troubleshooting:**
- If command not found: Install Docker Compose v2 plugin
- If showing v1: Update to Docker Compose v2

**Next Task:** TASK-004

---

## TASK-004: Verify PostgreSQL Client (psql)

**Status:** pending
**Dependencies:** None
**Estimated Time:** 10 minutes

**Description:**
Verify PostgreSQL client tools (psql) are installed.

**Steps:**
1. Run `psql --version`
2. Verify psql is accessible

**Output Definition:**
- psql version displayed

**Verification Commands:**
```bash
psql --version
```

**Expected Output:**
```
psql (PostgreSQL) 14.10 or higher
```

**Troubleshooting:**
- Linux: `sudo apt install postgresql-client`
- macOS: `brew install postgresql`

**Next Task:** TASK-005

---

## TASK-005: Verify Redis Client (redis-cli)

**Status:** pending
**Dependencies:** None
**Estimated Time:** 10 minutes

**Description:**
Verify Redis client tools (redis-cli) are installed.

**Steps:**
1. Run `redis-cli --version`
2. Verify redis-cli is accessible

**Output Definition:**
- redis-cli version displayed

**Verification Commands:**
```bash
redis-cli --version
```

**Expected Output:**
```
redis-cli 7.0.0 or higher
```

**Troubleshooting:**
- Linux: `sudo apt install redis-tools`
- macOS: `brew install redis`

**Next Task:** TASK-006

---

## TASK-006: Install Vegeta

**Status:** pending
**Dependencies:** TASK-001
**Estimated Time:** 15 minutes

**Description:**
Install Vegeta load testing tool.

**Steps:**
1. Install via Go: `go install github.com/tsenart/vegeta@latest`
2. Verify $(go env GOPATH)/bin is in PATH
3. Run `vegeta --version`

**Output Definition:**
- vegeta version displayed
- vegeta accessible in PATH

**Verification Commands:**
```bash
vegeta --version
```

**Expected Output:**
```
vegeta version 12.11.0
```

**Troubleshooting:**
- If not in PATH: Add `$(go env GOPATH)/bin` to PATH
- If install fails: Ensure Go 1.21+ is installed

**Next Task:** TASK-007

---

## TASK-007: Create Project Directory Structure

**Status:** pending
**Dependencies:** TASK-001
**Estimated Time:** 10 minutes

**Description:**
Create the Go project directory structure.

**Steps:**
1. Create directories: `cmd/api`, `internal/{handler,service,repository,cache,model,config}`, `pkg`, `scripts`, `test-results`

**Output Definition:**
- All directories created
- Structure matches Go standard layout

**Verification Commands:**
```bash
ls -la cmd/api
ls -la internal/handler
ls -la internal/service
ls -la internal/repository
ls -la internal/cache
ls -la internal/model
ls -la internal/config
ls -la pkg
ls -la scripts
ls -la test-results
```

**Expected Output:**
```
cmd/api/
internal/handler/
internal/service/
internal/repository/
internal/cache/
internal/model/
internal/config/
pkg/
scripts/
test-results/
```

**Troubleshooting:**
- If directories exist: Skip (no action needed)
- Use `mkdir -p` to create nested directories

**Next Task:** TASK-008

---

## TASK-008: Initialize go.mod and .env.example

**Status:** pending
**Dependencies:** TASK-007
**Estimated Time:** 15 minutes

**Description:**
Initialize Go module and create environment variable template.

**Steps:**
1. Run `go mod init github.com/yourusername/highth`
2. Create `.env.example` with all required variables
3. Verify go.mod is created

**Output Definition:**
- go.mod file exists with module name
- .env.example file exists with all variables

**Verification Commands:**
```bash
cat go.mod
cat .env.example
```

**Expected go.mod Contents:**
```go
module github.com/yourusername/highth

go 1.21
```

**Expected .env.example Contents:**
```bash
# Server Configuration
PORT=8080
HOST=0.0.0.0

# Database Configuration
DATABASE_URL=postgres://sensor_user:your_password_here@localhost:5432/sensor_db
DB_MAX_CONNECTIONS=25
DB_MIN_CONNECTIONS=5

# Redis Configuration
REDIS_URL=redis://localhost:6379
REDIS_ENABLED=true
REDIS_TTL=30s

# Cache Configuration
CACHE_ENABLED=true

# Application Configuration
LOG_LEVEL=info
REQUEST_TIMEOUT=30s
```

**Troubleshooting:**
- If go.mod init fails: Ensure Go 1.21+ is installed and you're in the project root

**Next Task:** TASK-009 (Phase 1)

---

## Phase 0 Completion Checklist

- [ ] TASK-001: Go 1.21+ installed and verified
- [ ] TASK-002: Docker installed and running
- [ ] TASK-003: Docker Compose v2+ verified
- [ ] TASK-004: PostgreSQL client (psql) installed
- [ ] TASK-005: Redis client (redis-cli) installed
- [ ] TASK-006: Vegeta installed
- [ ] TASK-007: Project directory structure created
- [ ] TASK-008: go.mod and .env.example created

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 1.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
