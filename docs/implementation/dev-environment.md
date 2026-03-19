# Development Environment Setup

This guide covers installing and configuring all required tools for the High-Performance IoT Sensor Query System.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Go Installation](#go-installation)
- [Docker Installation](#docker-installation)
- [Client Tools Installation](#client-tools-installation)
- [Vegeta Installation](#vegeta-installation)
- [Project Structure Setup](#project-structure-setup)
- [Environment Variables](#environment-variables)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting, ensure you have:

- **Operating System:** Linux (Ubuntu 22.04+ recommended) or macOS (Ventura+)
  - Windows: Use WSL2 with Ubuntu (not covered in this guide)
- **Hardware:**
  - Minimum: 8GB RAM, 20GB free disk space, SSD (not HDD)
  - Recommended: 16GB RAM, 50GB free disk space, NVMe SSD
- **Internet:** Connection for downloading packages and Docker images
- **Permissions:** sudo/admin access for package installation

---

## Go Installation

### Required Version: Go 1.21 or later

### Linux (Ubuntu/Debian)

```bash
# Remove old Go versions if present
sudo rm -rf /usr/local/go

# Download Go 1.21.6 (or latest)
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz

# Verify download
sha256sum go1.21.6.linux-amd64.tar.gz

# Extract to /usr/local
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
# Expected output: go version go1.21.6 linux/amd64
```

### macOS

```bash
# Using Homebrew (recommended)
brew install go

# Or download installer from
# https://go.dev/dl/

# Verify installation
go version
```

### Configuration

```bash
# Set Go workspace (optional, but recommended)
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
source ~/.bashrc

# Create workspace directory
mkdir -p $GOPATH/src
```

---

## Docker Installation

### Linux (Ubuntu 22.04+)

```bash
# Update package index
sudo apt update

# Install prerequisites
sudo apt install -y ca-certificates curl gnupg

# Add Docker's official GPG key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

# Set up repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add user to docker group (requires logout/login)
sudo usermod -aG docker $USER

# Enable and start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Verify installation
docker --version
docker compose version
```

### macOS

```bash
# Install Docker Desktop (includes Docker Compose)
# Download from: https://www.docker.com/products/docker-desktop/

# Or using Homebrew Cask
brew install --cask docker

# Start Docker Desktop application after installation
# Verify installation
docker --version
docker compose version
```

### Docker Configuration

```bash
# Allocate adequate resources to Docker Desktop
# macOS: Docker Dashboard > Settings > Resources
# - Memory: 4GB minimum (8GB recommended)
# - CPUs: 2 minimum (4 recommended)
# - Disk: 50GB minimum

# Linux: No configuration needed, uses system resources
```

---

## Client Tools Installation

### PostgreSQL Client Tools

#### Linux

```bash
sudo apt update
sudo apt install -y postgresql-client

# Verify installation
psql --version
```

#### macOS

```bash
brew install postgresql

# Or use Postgres.app (includes client tools)
# Download from: https://postgresapp.com/

# Verify installation
psql --version
```

### Redis Client Tools

#### Linux

```bash
sudo apt install -y redis-tools

# Verify installation
redis-cli --version
```

#### macOS

```bash
brew install redis

# Start Redis server (optional, we'll use Docker)
brew services start redis

# Verify installation
redis-cli --version
```

---

## Vegeta Installation

### Via Go Install (Recommended)

```bash
# Install Vegeta
go install github.com/tsenart/vegeta@latest

# Add Go bin to PATH if not already
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
vegeta --version
# Expected output: vegeta version 12.11.0
```

### Via Binary Download

```bash
# Download latest release
wget https://github.com/tsenart/vegeta/releases/download/v12.11.0/vegeta_12.11.0_linux_amd64.tar.gz

# Extract
tar -xvf vegeta_12.11.0_linux_amd64.tar.gz

# Install
sudo mv vegeta /usr/local/bin/

# Verify installation
vegeta --version
```

### macOS Alternative

```bash
brew install vegeta
```

---

## Project Structure Setup

### Create Directory Structure

```bash
# Navigate to project directory
cd /path/to/your/projects/highth

# Create Go project structure
mkdir -p cmd/api
mkdir -p internal/{handler,service,repository,cache,model,config}
mkdir -p pkg
mkdir -p docs/implementation
mkdir -p scripts
mkdir -p test-results

# Verify structure
tree -L 2
```

Expected structure:
```
highth/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── cache/
│   ├── model/
│   └── config/
├── pkg/
├── docs/
│   └── implementation/
├── scripts/
├── test-results/
├── .env.example
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml
```

### Initialize Go Module

```bash
# Initialize go.mod
go mod init github.com/yourusername/highth

# Create basic go.mod with dependencies
cat > go.mod << 'EOF'
module github.com/yourusername/highth

go 1.21

require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/jackc/pgx/v5 v5.5.1
    github.com/redis/go-redis/v9 v9.4.0
    github.com/google/uuid v1.5.0
)
EOF
```

---

## Environment Variables

### Create .env.example

```bash
# Create .env.example with all required variables
cat > .env.example << 'EOF'
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
EOF

# Create actual .env from example (DO NOT commit .env to git)
cp .env.example .env

# Update .env with your actual values
nano .env
```

### .gitignore

```bash
# Create .gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/
/tmp/

# Test binary, built with `go test -c`
*.test

# Output of the go coverage tool
*.out

# Dependency directories
/vendor/

# Go workspace file
go.work

# Environment variables
.env

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Test results
test-results/

# Docker volumes
postgres_data/
redis_data/
EOF
```

---

## Verification

### Run All Verification Checks

```bash
#!/bin/bash

echo "=== Environment Verification ==="

echo -n "Go version: "
go version | grep -o 'go[0-9.]*'

echo -n "Docker version: "
docker --version | grep -o 'Docker version [0-9.]*'

echo -n "Docker Compose version: "
docker compose version | grep -o 'v[0-9.]*'

echo -n "PostgreSQL client: "
psql --version | grep -o 'psql (PostgreSQL) [0-9.]*'

echo -n "Redis client: "
redis-cli --version | grep -o 'redis-cli [0-9.]*'

echo -n "Vegeta: "
vegeta --version | grep -o 'vegeta version [0-9.]*'

echo ""
echo "=== Project Structure ==="
ls -la

echo ""
echo "=== Environment Files ==="
ls -la .env.example

echo ""
echo "=== Go Module ==="
cat go.mod

echo ""
echo "✓ Environment setup complete!"
```

### Test Docker Functionality

```bash
# Test Docker with hello-world
docker run hello-world

# Expected output: Hello from Docker!
```

---

## Troubleshooting

### Go Installation Issues

**Problem:** `go: command not found`

**Solution:**
```bash
# Check if Go is in PATH
which go

# If not found, add to PATH
export PATH=$PATH:/usr/local/go/bin

# Add permanently to shell config
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**Problem:** `permission denied` when installing Go

**Solution:**
```bash
# Use sudo for system-wide installation
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Or install to user directory (no sudo needed)
mkdir -p $HOME/go
tar -C $HOME/go -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:$HOME/go/bin
```

### Docker Issues

**Problem:** Docker daemon not running

**Solution (Linux):**
```bash
sudo systemctl start docker
sudo systemctl enable docker
```

**Solution (macOS):**
```bash
# Start Docker Desktop application
open -a Docker
```

**Problem:** Permission denied when running Docker without sudo

**Solution (Linux):**
```bash
# Add user to docker group (if not already added)
sudo usermod -aG docker $USER

# Log out and log back in for group change to take effect
# Or use newgrp docker (may not work in all shells)
```

**Problem:** Docker resources insufficient

**Solution:**
- macOS: Docker Dashboard > Settings > Resources > Increase Memory
- Linux: Docker uses system resources; ensure adequate RAM available

### PostgreSQL Client Issues

**Problem:** `psql: command not found`

**Solution:**
```bash
# Ubuntu/Debian
sudo apt install postgresql-client

# macOS
brew install postgresql
```

### Redis Client Issues

**Problem:** `redis-cli: command not found`

**Solution:**
```bash
# Ubuntu/Debian
sudo apt install redis-tools

# macOS
brew install redis
```

### Vegeta Issues

**Problem:** `vegeta: command not found`

**Solution:**
```bash
# Check Go bin directory
echo $(go env GOPATH)/bin

# Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Reinstall Vegeta
go install github.com/tsenart/vegeta@latest
```

**Problem:** Vegeta installation fails

**Solution:**
```bash
# Ensure Go is properly installed
go version

# Check GOPATH is set
go env GOPATH

# Set GOPATH if empty
export GOPATH=$HOME/go
mkdir -p $GOPATH/src
```

### Port Conflicts

**Problem:** Port 5432 or 8080 already in use

**Solution:**
```bash
# Check what's using the port
sudo lsof -i :5432  # PostgreSQL
sudo lsof -i :8080  # API server

# Stop conflicting service
sudo systemctl stop postgresql  # If conflicting PostgreSQL

# Or use different ports in .env
# Update .env:
# DATABASE_URL=postgres://...@localhost:5433/sensor_db
# PORT=8081
```

---

## Next Steps

Once environment setup is complete and verified:

1. **[database-setup.md](database-setup.md)** — Provision PostgreSQL with schema
2. **[data-generation.md](data-generation.md)** — Generate 50M test dataset
3. **[api-development.md](api-development.md)** — Build the Go API
4. **[cache-setup.md](cache-setup.md)** — Integrate Redis caching
5. **[load-testing-setup.md](load-testing-setup.md)** — Execute performance tests
6. **[validation-checklist.md](validation-checklist.md)** — Verify everything works

---

## Related Documentation

- **[../README.md](../README.md)** — Project overview
- **[../architecture.md](../architecture.md)** — System architecture
- **[../stack.md](../stack.md)** — Technology stack details
