#!/bin/bash
# Higth IoT Benchmark Test Runner
#
# Comprehensive load testing for the Higth API using k6.
# Tests high-volume (83M+ rows) database performance with <500ms latency target.
#
# Usage:
#   ./run-benchmarks.sh                    # Run all scenarios
#   ./run-benchmarks.sh --scenario hot     # Run specific scenario
#   ./run-benchmarks.sh --rps 100          # Custom RPS
#   ./run-benchmarks.sh --duration 5m      # Custom duration
#   ./run-benchmarks.sh --list             # List available scenarios
#   ./run-benchmarks.sh --with-html-report # Generate HTML report
#
# Requirements:
#   - Docker and Docker Compose
#   - API, PostgreSQL, and Redis services running
#   - k6 Docker image (auto-pulled)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$PROJECT_ROOT/test-results"

# Default configuration
TARGET_URL="${TARGET_URL:-http://localhost:8080}"
RPS="${RPS:-50}"
DURATION="${DURATION:-2m}"
SCENARIO=""
LIST_ONLY=false
SKIP_SETUP=false
VERBOSE=false
WITH_HTML_REPORT=false
TIER=""
EXPLICIT_RPS=false
EXPLICIT_DURATION=false
COOLDOWN=15  # seconds to wait between scenarios for system recovery

# Array to track result files generated during this run
RESULT_FILES=()

# Available scenarios
declare -A SCENARIOS=(
  ["hot"]="01-hot-device-pattern.js"
  ["time-range"]="02-time-range-queries.js"
  ["mixed"]="03-mixed-workload.js"
  ["cache"]="04-cache-performance.js"
  ["stats"]="05-stats-and-aggregation.js"
  ["pk-lookup"]="06-pk-lookup.js"
  ["stress"]="07-stress-ceiling.js"
  ["complexity"]="08-query-complexity.js"
)

# ============================================================================
# TIER PROFILES
# ============================================================================
# Pre-defined load profiles per scenario and tier.
# Tiers: smoke, low, medium, high, expert
#
# Hardware guidance:
#   smoke   - Any machine that can run Docker (2+ cores, 4GB RAM)
#   low     - Development laptop (4 cores, 8GB RAM, SSD)
#   medium  - Staging server or beefy workstation (8 cores, 16GB RAM, SSD)
#   high    - Production-equivalent hardware (8+ cores, 16-32GB RAM, NVMe SSD)
#   expert  - Production hardware with monitoring active (8+ cores, 32GB RAM, NVMe)
#
# SLO policy: All tiers use the same k6 thresholds (hardcoded in .js files).
# Higher tiers prove the system meets the same SLO under increasing stress.
# If expert tier fails, it means "the system breaks before this RPS level"
# -- which is the whole point of the tier.
#
# Usage: ./run-benchmarks.sh --tier medium --scenario hot
# Override: --tier medium --rps 200  (explicit --rps overrides tier RPS)
# ============================================================================

# Tier profile lookup: TIER_RPS[<tier>:<scenario>] = RPS
declare -A TIER_RPS=(
    # hot scenario (constant-arrival-rate, default 50 RPS, cache-heavy)
    # Doc target: 450+ RPS with p95=7ms (cached)
    ["smoke:hot"]=5       ["low:hot"]=50      ["medium:hot"]=150     ["high:hot"]=350     ["expert:hot"]=600
    # time-range scenario (constant-arrival-rate, default 30 RPS, MV queries)
    # Doc target: 300+ RPS with p95=20ms
    ["smoke:time-range"]=5  ["low:time-range"]=30   ["medium:time-range"]=90  ["high:time-range"]=250  ["expert:time-range"]=400
    # mixed scenario (ramping-arrival-rate, default peak 100 RPS, multi-endpoint)
    # Doc target: 470+ RPS with p95=8ms. RPS value = peak of ramp.
    ["smoke:mixed"]=20     ["low:mixed"]=100     ["medium:mixed"]=250    ["high:mixed"]=500    ["expert:mixed"]=800
    # cache scenario (constant-arrival-rate, default 50 RPS, 3-phase cold/warm/hot)
    # Duration split into 3 equal phases. Each phase needs >=10s for meaningful measurement.
    ["smoke:cache"]=5      ["low:cache"]=50      ["medium:cache"]=100    ["high:cache"]=200    ["expert:cache"]=400
    # stats scenario (constant-arrival-rate, default 20 RPS, MV-only, no Redis)
    # Most DB-intensive scenario. Each request = 1 MV query, no cache help.
    ["smoke:stats"]=3      ["low:stats"]=20      ["medium:stats"]=60     ["high:stats"]=120    ["expert:stats"]=200
    # pk-lookup scenario (constant-arrival-rate, default 50 RPS, single-row PK index scan)
    # Simplest possible query — tightest latency targets in the suite.
    ["smoke:pk-lookup"]=5  ["low:pk-lookup"]=50  ["medium:pk-lookup"]=200 ["high:pk-lookup"]=500 ["expert:pk-lookup"]=800
    # stress scenario (ramping-arrival-rate, finds hardware ceiling)
    # RPS value = starting rate. Scenario ramps up by 50 RPS per step.
    # Duration = time per step. Total runtime depends on how high it ramps.
    ["smoke:stress"]=20      ["low:stress"]=50      ["medium:stress"]=50     ["high:stress"]=50     ["expert:stress"]=50
    # complexity scenario (4 sequential sub-scenarios testing different hardware paths)
    # Lower RPS since each sub-scenario runs sequentially with cooldown between them.
    ["smoke:complexity"]=10   ["low:complexity"]=30   ["medium:complexity"]=50  ["high:complexity"]=100 ["expert:complexity"]=150
)

# Tier profile lookup: TIER_DURATION[<tier>:<scenario>] = duration
declare -A TIER_DURATION=(
    # hot scenario durations
    ["smoke:hot"]=10s      ["low:hot"]=30s      ["medium:hot"]=45s      ["high:hot"]=60s      ["expert:hot"]=2m
    # time-range scenario durations
    ["smoke:time-range"]=10s ["low:time-range"]=30s  ["medium:time-range"]=45s ["high:time-range"]=60s ["expert:time-range"]=2m
    # mixed scenario (per-stage duration for ramping-arrival-rate)
    ["smoke:mixed"]=10s    ["low:mixed"]=10s     ["medium:mixed"]=10s     ["high:mixed"]=15s     ["expert:mixed"]=15s
    # cache scenario durations (total duration, split into 3 equal phases)
    ["smoke:cache"]=30s    ["low:cache"]=45s     ["medium:cache"]=45s     ["high:cache"]=60s     ["expert:cache"]=90s
    # stats scenario durations
    ["smoke:stats"]=15s    ["low:stats"]=60s     ["medium:stats"]=60s     ["high:stats"]=90s     ["expert:stats"]=2m
    # pk-lookup scenario durations
    ["smoke:pk-lookup"]=10s ["low:pk-lookup"]=30s ["medium:pk-lookup"]=45s ["high:pk-lookup"]=60s ["expert:pk-lookup"]=2m
    # stress scenario durations (per ramp step — total runtime = steps × duration)
    ["smoke:stress"]=10s    ["low:stress"]=20s     ["medium:stress"]=30s    ["high:stress"]=30s    ["expert:stress"]=30s
    # complexity scenario durations (per sub-scenario — 4 sub-scenarios + cooldowns)
    ["smoke:complexity"]=10s ["low:complexity"]=30s ["medium:complexity"]=30s ["high:complexity"]=45s ["expert:complexity"]=45s
)

# ============================================================================
# HIGH-LOAD / STRESS CONFIGURATION (uncomment for stress testing)
# ============================================================================
# Target: 2000 RPS sustained load
#
# Rationale:
#   - DB pool: 50 connections → saturates at ~1000-2000 RPS depending on query time
#   - Redis: ~100K ops/sec (single-threaded), well above 2000 RPS
#   - Auto-calculated maxVUs: 2000 * 2 = 4000 VUs
#   - No API rate limiting
#
# PRE-CONDITIONS:
#   - Data seeded (100K devices, 83M+ rows)
#   - Redis warm (run hot scenario first)
#   - Monitoring active (compose.monitoring.yml)
#
# MONITORING:
#   - docker stats highth-api (CPU/memory)
#   - Grafana postgres-exporter (connection pool)
#   - docker logs highth-api 2>&1 | tail -f (connection errors)
#
# CAVEATS:
#   - k6 needs ~200-500MB RAM at 4000 VUs
#   - Host needs sufficient ephemeral ports
#   - Expect degraded latency; focus on error rate <5%
#   - Dropped iterations = system cannot sustain target RPS
#
# USAGE:
#   ./tests/run-benchmarks.sh --scenario hot --rps 2000 --duration 5m --verbose
#   ./tests/run-benchmarks.sh --scenario mixed --rps 2000 --duration 5m
#   ./tests/run-benchmarks.sh --rps 2000 --duration 5m  # all scenarios
# ============================================================================
# RPS="${RPS:-2000}"
# DURATION="${DURATION:-5m}"
# ============================================================================

# Target latency thresholds (milliseconds)
P50_TARGET=300
P95_TARGET=500
P99_TARGET=800

# Helper functions
print_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${BOLD}Higth IoT Benchmark Suite v2.0${NC}                              ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

# Parse command-line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --scenario|-s)
                SCENARIO="$2"
                shift 2
                ;;
            --rps|-r)
                RPS="$2"
                EXPLICIT_RPS=true
                shift 2
                ;;
            --duration|-d)
                DURATION="$2"
                EXPLICIT_DURATION=true
                shift 2
                ;;
            --target-url|-u)
                TARGET_URL="$2"
                shift 2
                ;;
            --vus)
                VUS="$2"
                shift 2
                ;;
            --cooldown)
                COOLDOWN="$2"
                shift 2
                ;;
            --no-cooldown)
                COOLDOWN=0
                shift
                ;;
            --list|-l)
                LIST_ONLY=true
                shift
                ;;
            --skip-setup)
                SKIP_SETUP=true
                shift
                ;;
            --verbose|-v)
                VERBOSE=true
                shift
                ;;
            --with-html-report)
                WITH_HTML_REPORT=true
                shift
                ;;
            --tier)
                TIER="$2"
                shift 2
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << 'EOF'
Usage: run-benchmarks.sh [OPTIONS]

TEST SCENARIOS:
  hot          Hot Device Pattern
                 Simulates Zipf-distributed IoT traffic (20% of devices receive 80% of traffic).
                 Tests caching effectiveness under uneven load patterns.
                 Executor: constant-arrival-rate | Default RPS: 50 | VUs: 10-50

  time-range   Time Range Queries
                 Tests dashboard-style queries with varying time windows (1h, 24h, 7d).
                 Validates materialized view performance for different data volumes.
                 Executor: constant-arrival-rate | Default RPS: 30 | VUs: 10-40

  mixed        Mixed Workload
                 Realistic API usage mix: 10% health checks, 20% stats queries, 70% readings.
                 Uses ramping-arrival-rate to simulate increasing/decreasing load.
                 Executor: ramping-arrival-rate | RPS: 10-100 ramp | VUs: 10-100

  cache        Cache Performance
                 Three-phase Redis cache effectiveness test (cold/warm/hot).
                 Measures cache hit rate and latency improvements as cache warms.
                 Executor: constant-arrival-rate | Default RPS: 50 | VUs: 10-50

  stats        Stats and Aggregation
                 Tests /api/v1/stats (materialized view, cache bypass).
                 Validates MV query performance under sustained load without Redis.
                 Executor: constant-arrival-rate | Default RPS: 20 | VUs: 10-40

  pk-lookup    Primary-Key Hot Lookup
                 Single-row lookup by primary key ID (B-tree index scan).
                 Benchmarks raw PostgreSQL hot-path performance.
                 Dynamic ID range via /api/v1/stats (total_readings == MAX(id)).
                 Executor: constant-arrival-rate | Default RPS: 50 | VUs: 10-50

  stress       Stress Ceiling Finder
                 Ramps RPS from 50 upward by 50 RPS per step until system breaks.
                 Reports maximum sustainable RPS -- finds the hardware ceiling.
                 Uses mixed query types (PK, device, stats).
                 Executor: ramping-arrival-rate | Start RPS: 50 | VUs: 20-500

  complexity   Query Complexity Tiers
                 4 sequential sub-scenarios testing different hardware paths:
                 1) PK lookup (B-tree -> CPU+memory)
                 2) Device filter (composite index -> CPU+memory)
                 3) Time-range (BRIN scan -> disk I/O)
                 4) Stats aggregation (materialized view -> MV freshness)
                 Use to identify WHICH hardware component is the bottleneck.
                 Executor: constant-arrival-rate (per tier) | VUs: 3-30

PARAMETERS:
  -s, --scenario <name>     Run specific scenario (hot, time-range, mixed, cache, stats, pk-lookup, stress, complexity)
  --tier <name>             Load tier profile (smoke, low, medium, high, expert)
  -r, --rps <number>        Requests per second for constant-arrival-rate scenarios
  -d, --duration <time>     Test duration per scenario (e.g., 30s, 5m, 1h)
  -u, --target-url <url>    API endpoint to test (default: http://localhost:8080)
  --vus <number>            Maximum virtual users (maxVUs). If omitted, auto-calculated as RPS * 2
  --cooldown <seconds>      Seconds to wait between scenarios for system recovery (default: 15)
  --no-cooldown             Disable cooldown between scenarios
  --with-html-report        Generate HTML report alongside JSON output
  --skip-setup              Skip service health checks before running tests
  -l, --list                List available scenarios
  -v, --verbose             Enable verbose output (show detailed metrics after each test)
  -h, --help                Show this help message

LOAD TIERS:
  --tier <name>            Apply a pre-defined load profile per scenario

  Available tiers:
    smoke     Minimal load (verify scenario runs without errors)
              Hardware: Any machine with Docker (2+ cores, 4GB RAM)

    low       Light realistic load (everyday baseline)
              Hardware: Development laptop (4 cores, 8GB RAM, SSD)
              Values match scenario defaults

    medium    Moderate production-like traffic (should meet SLO)
              Hardware: Staging server (8 cores, 16GB RAM, SSD)

    high      Stress test (peak traffic, SLO should hold)
              Hardware: Production-equivalent (8+ cores, 16-32GB RAM, NVMe)

    expert    Near-limit load (find the ceiling before breakage)
              Hardware: Production with monitoring (8+ cores, 32GB RAM, NVMe)
              Failure is acceptable and informative

  Tier profiles set per-scenario RPS and duration. VUs are auto-calculated.
  Explicit --rps, --duration, or --vus flags override tier values.

  SLO policy: All tiers use the same k6 thresholds. Higher tiers prove the
  system meets the same SLO under increasing stress.

VUs VS RPS:
  VUs (Virtual Users) are the maximum concurrent connections k6 will open.
  RPS (Requests Per Second) is the target arrival rate.

  k6 automatically manages actual concurrency between preAllocatedVUs and maxVUs.
  If maxVUs is too low for the requested RPS, k6 drops iterations (check
  'dropped_iterations' in the report). If VUs are excessively high relative to
  RPS, connections idle and waste resources.

  AUTO-CALCULATION:
  When --vus is omitted, maxVUs is automatically calculated as RPS * 2.
  This provides sufficient headroom for most workloads while avoiding
  excessive idle connections. For precise control, explicitly set --vus.

  Recommended:
  - Let the script auto-calculate (omit --vus) for most cases
  - Manually set --vus only when you need precise control for specific scenarios
  - For 1000 RPS: auto-calculated maxVUs = 2000

  Manual override examples:
  - For 100 RPS with default auto-calculation: --rps 100 (maxVUs=200)
  - For 1000 RPS with fewer VUs: --rps 1000 --vus 1500 (if you know 2000 is too many)

EXAMPLES:
  # Run all scenarios with defaults
  run-benchmarks.sh

  # Run all scenarios at medium tier
  run-benchmarks.sh --tier medium

  # Run a specific scenario
  run-benchmarks.sh --scenario hot

  # Run hot scenario at high tier with HTML report
  run-benchmarks.sh --tier high --scenario hot --with-html-report

  # Expert tier for stats (heaviest DB workload) with verbose output
  run-benchmarks.sh --tier expert --scenario stats --verbose

  # Tier with explicit RPS override
  run-benchmarks.sh --tier medium --scenario hot --rps 200

  # Custom load with HTML report
  run-benchmarks.sh --scenario hot --rps 100 --duration 5m --with-html-report

  # High concurrency test with custom VUs
  run-benchmarks.sh --scenario mixed --rps 200 --vus 300

  # Test different endpoint with verbose output
  run-benchmarks.sh --target-url http://staging.example.com --verbose

  # Quick smoke test (short duration, low RPS)
  run-benchmarks.sh --scenario cache --duration 30s --rps 20

EOF
}

list_scenarios() {
    print_header
    echo "Available Test Scenarios:"
    echo ""
    for key in "${!SCENARIOS[@]}"; do
        echo -e "  ${CYAN}${key}${NC}    ${SCENARIOS[$key]}"
    done
    echo ""
    echo "Use --scenario <name> to run a specific test"
    exit 0
}

check_requirements() {
    print_info "Checking requirements..."

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    print_success "Docker is installed"

    # Check Docker Compose
    if ! command -v docker compose &> /dev/null; then
        print_error "Docker Compose is not installed"
        exit 1
    fi
    print_success "Docker Compose is installed"

    # Pull k6 image if not exists
    if ! docker images grafana/k6 --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -q 'grafana/k6:latest'; then
        print_info "Pulling k6 Docker image..."
        if docker pull grafana/k6:latest; then
            print_success "k6 image pulled successfully"
        else
            print_error "Failed to pull k6 image"
            exit 1
        fi
    else
        print_success "k6 image is available"
    fi
}

check_services() {
    if [ "$SKIP_SETUP" = true ]; then
        print_info "Skipping service health checks"
        return
    fi

    print_info "Checking service health..."

    # Check API
    if ! curl -sf "$TARGET_URL/health" > /dev/null 2>&1; then
        print_error "API is not responding at $TARGET_URL"
        print_info "Start services with: docker-compose up -d"
        exit 1
    fi
    print_success "API is healthy"

    # Check database (via API stats endpoint)
    if ! curl -sf "$TARGET_URL/api/v1/stats" > /dev/null 2>&1; then
        print_error "API stats endpoint is not responding"
        exit 1
    fi
    print_success "Database connection is working"
}

create_results_dir() {
    mkdir -p "$RESULTS_DIR"
}

run_scenario() {
    local scenario_name="$1"
    local scenario_file="$2"

    print_section "Scenario: $scenario_name"

    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_file="$RESULTS_DIR/${scenario_name}_${timestamp}.json"

    print_info "Configuration:"
    echo "  Scenario:    $scenario_file"
    echo "  Target URL:  $TARGET_URL"
    if [ -n "$RPS" ]; then
        echo "  Custom RPS:  $RPS"
    fi
    if [ -n "$DURATION" ]; then
        echo "  Duration:    $DURATION"
    fi
    if [ -n "$VUS" ]; then
        echo "  Custom VUs:  $VUS (maxVUs ceiling — actual concurrency depends on RPS and latency)"
    fi
    echo "  Output:      $output_file"
    echo ""

    # Apply tier profile if --tier is set (tier overrides global RPS/DURATION for this scenario)
    local effective_rps="$RPS"
    local effective_duration="$DURATION"
    local effective_vus="$VUS"
    if [ -n "$TIER" ]; then
        local tier_key="${TIER}:${scenario_name}"
        if [[ -n "${TIER_RPS[$tier_key]+_}" ]]; then
            # Tier values are used only if no explicit override was provided
            if [ "$EXPLICIT_RPS" != true ]; then
                effective_rps="${TIER_RPS[$tier_key]}"
            fi
            if [ "$EXPLICIT_DURATION" != true ]; then
                effective_duration="${TIER_DURATION[$tier_key]}"
            fi
            # VUs: recalculate from effective RPS unless explicitly set
            if [ -z "$VUS" ]; then
                effective_vus=$(( effective_rps * 2 ))
                if [ "$effective_vus" -lt 50 ]; then
                    effective_vus=50
                fi
            fi
            print_info "Tier '$TIER' profile: RPS=$effective_rps, Duration=$effective_duration"
        else
            print_error "No tier profile for tier '$TIER', scenario '$scenario_name'"
            return 1
        fi
    fi

    # Flush Redis before cache scenario to ensure cold cache start
    if [ "$scenario_name" = "cache" ]; then
        print_info "Flushing Redis cache for cold cache start..."
        if docker exec highth-redis redis-cli FLUSHALL > /dev/null 2>&1; then
            print_success "Redis cache flushed"
        else
            print_warning "Failed to flush Redis cache (container may not be running)"
        fi
    fi

    print_info "Running k6 test..."

    # Build k6 verbose flag if --verbose was passed
    local k6_verbose_flag=""
    if [ "$VERBOSE" = true ]; then
        k6_verbose_flag="--verbose"
    fi

    # Run k6 test directly (no eval, no string manipulation)
    docker run --rm --network host \
        --user "$(id -u):$(id -g)" \
        -v "$SCRIPT_DIR:/tests" \
        -v "$RESULTS_DIR:/results" \
        -e TARGET_URL="$TARGET_URL" \
        -e CUSTOM_RPS="$effective_rps" \
        -e CUSTOM_DURATION="$effective_duration" \
        -e CUSTOM_VUS="$effective_vus" \
        grafana/k6:latest run \
        $k6_verbose_flag \
        --summary-trend-stats="avg,min,med,max,p(90),p(95),p(99)" \
        --summary-export="/results/${scenario_name}_${timestamp}.json" \
        "/tests/scenarios/$scenario_file"

    local exit_code=$?

    # Inject scenario metadata into the JSON summary
    local summary_file="$RESULTS_DIR/${scenario_name}_${timestamp}.json"
    if [ -f "$summary_file" ] && [ -s "$summary_file" ] && command -v jq &> /dev/null; then
        jq --arg scenario "$scenario_name" \
           --arg scenario_file "$scenario_file" \
           --arg timestamp "$timestamp" \
           '. + {scenario: $scenario, scenario_file: $scenario_file, timestamp: $timestamp}' \
           "$summary_file" > "${summary_file}.tmp" 2>/dev/null
        if [ $? -eq 0 ] && [ -s "${summary_file}.tmp" ]; then
            command rm -f "$summary_file" && command mv "${summary_file}.tmp" "$summary_file"
        else
            rm -f "${summary_file}.tmp"
        fi
    fi

    # Track result file for HTML report generation
    RESULT_FILES+=("$summary_file")

    # Show parsed metrics summary in verbose mode
    if [ "$VERBOSE" = true ] && command -v jq &> /dev/null; then
        parse_k6_summary "$summary_file"
    fi

    if [ $exit_code -eq 0 ]; then
        print_success "Scenario '$scenario_name' completed"
        return 0
    else
        print_error "Scenario '$scenario_name' failed (exit code: $exit_code)"
        return 1
    fi
}

parse_k6_summary() {
    local summary_file="$1"

    if [ ! -f "$summary_file" ]; then
        print_warning "Summary file not found: $summary_file"
        return
    fi

    # Parse JSON summary using basic tools (no jq dependency)
    if command -v jq &> /dev/null; then
        local p50=$(jq -r '.metrics.http_req_duration["p(50)"] // .metrics.http_req_duration.med // "N/A"' "$summary_file")
        local p95=$(jq -r '.metrics.http_req_duration["p(95)"] // "N/A"' "$summary_file")
        local p99=$(jq -r '.metrics.http_req_duration["p(99)"] // "N/A"' "$summary_file")
        local rps=$(jq -r '.metrics.http_reqs.rate // "N/A"' "$summary_file")
        local errors=$(jq -r '.metrics.http_req_failed.rate // "N/A"' "$summary_file")

        echo ""
        echo "Results:"
        echo "  p50:   ${p50}ms  $(check_latency $p50 $P50_TARGET)"
        echo "  p95:   ${p95}ms  $(check_latency $p95 $P95_TARGET)"
        echo "  p99:   ${p99}ms  $(check_latency $p99 $P99_TARGET)"
        echo "  RPS:   ${rps}"
        echo "  Errors: ${errors}"
    else
        print_warning "Install jq for detailed metrics parsing"
    fi
}

check_latency() {
    local actual=$1
    local target=$2

    if [ "$actual" = "N/A" ]; then
        echo "(N/A)"
    elif (( $(echo "$actual < $target" | bc -l) )); then
        echo -e "${GREEN}✓ (target: <${target}ms)${NC}"
    else
        echo -e "${RED}✗ (target: <${target}ms)${NC}"
    fi
}

generate_html_report() {
    if [ ${#RESULT_FILES[@]} -eq 0 ]; then
        print_warning "No result files to convert to HTML"
        return
    fi

    print_info "Generating HTML reports (${#RESULT_FILES[@]} file(s))..."

    for json_file in "${RESULT_FILES[@]}"; do
        if [ ! -f "$json_file" ]; then
            print_warning "Result file not found: $json_file"
            continue
        fi

        local html_file="${json_file%.json}.html"
        local basename=$(basename "$json_file" .json)

        # Extract metrics using jq if available, otherwise use N/A
        if command -v jq &> /dev/null; then
            local p50=$(jq -r '.metrics.http_req_duration["p(50)"] // .metrics.http_req_duration.med // "N/A"' "$json_file")
            local p95=$(jq -r '.metrics.http_req_duration["p(95)"] // "N/A"' "$json_file")
            local p99=$(jq -r '.metrics.http_req_duration["p(99)"] // "N/A"' "$json_file")
            local avg=$(jq -r '.metrics.http_req_duration.avg // "N/A"' "$json_file")
            local med=$(jq -r '.metrics.http_req_duration.med // "N/A"' "$json_file")
            local min=$(jq -r '.metrics.http_req_duration.min // "N/A"' "$json_file")
            local max=$(jq -r '.metrics.http_req_duration.max // "N/A"' "$json_file")
            local rps=$(jq -r '.metrics.http_reqs.rate // "N/A"' "$json_file")
            local total_reqs=$(jq -r '.metrics.http_reqs.count // "N/A"' "$json_file")
            local check_passes=$(jq -r '.metrics.checks.passes // "N/A"' "$json_file")
            local check_fails=$(jq -r '.metrics.checks.fails // "N/A"' "$json_file")
            local vus=$(jq -r '.metrics.vus.value // "N/A"' "$json_file")
            local dropped=$(jq -r '.metrics.dropped_iterations.count // "0"' "$json_file")
        else
            local p50="N/A" p95="N/A" p99="N/A"
            local avg="N/A" med="N/A" min="N/A" max="N/A"
            local rps="N/A" total_reqs="N/A"
            local check_passes="N/A" check_fails="N/A"
            local vus="N/A" dropped="N/A"
            print_warning "Install jq for detailed metrics. Using placeholders."
        fi

        # Calculate threshold status (pass/fail) for p50/p95/p99
        local p50_status="pass" p95_status="pass" p99_status="pass"
        if [ "$p50" != "N/A" ] && [ "$p50" != "null" ]; then
            if (( $(echo "$p50 >= $P50_TARGET" | bc -l 2>/dev/null || echo "1") )); then
                p50_status="fail"
            fi
        fi
        if [ "$p95" != "N/A" ] && [ "$p95" != "null" ]; then
            if (( $(echo "$p95 >= $P95_TARGET" | bc -l 2>/dev/null || echo "1") )); then
                p95_status="fail"
            fi
        fi
        if [ "$p99" != "N/A" ] && [ "$p99" != "null" ]; then
            if (( $(echo "$p99 >= $P99_TARGET" | bc -l 2>/dev/null || echo "1") )); then
                p99_status="fail"
            fi
        fi

        # Get raw JSON for embedding (escape for HTML)
        local raw_json=$(sed 's/&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g; s/"/\&quot;/g' "$json_file")

        # Generate HTML report
        cat > "$html_file" << HTMLEOF
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>k6 Benchmark Report - ${basename}</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif; background: #f5f7fa; color: #1f2937; line-height: 1.6; padding: 20px; }
.container { max-width: 1000px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); overflow: hidden; }
.header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 24px; }
.header h1 { font-size: 24px; font-weight: 600; margin-bottom: 8px; }
.meta { opacity: 0.9; font-size: 14px; }
.section { padding: 24px; border-bottom: 1px solid #e5e7eb; }
.section:last-child { border-bottom: none; }
.section h2 { font-size: 18px; font-weight: 600; margin-bottom: 16px; color: #374151; }
table { width: 100%; border-collapse: collapse; margin-bottom: 16px; }
th, td { padding: 10px 12px; text-align: left; border-bottom: 1px solid #f3f4f6; font-size: 14px; }
th { background: #f9fafb; font-weight: 600; color: #4b5563; }
tr:last-child th, tr:last-child td { border-bottom: none; }
.pass { color: #10b981; font-weight: 600; }
.fail { color: #ef4444; font-weight: 600; }
.metric-card { display: inline-block; background: #f9fafb; border-radius: 6px; padding: 16px; margin: 0 16px 16px 0; min-width: 140px; }
.metric-card .label { font-size: 12px; color: #6b7280; margin-bottom: 4px; }
.metric-card .value { font-size: 20px; font-weight: 600; color: #111827; }
.metric-card .unit { font-size: 14px; color: #6b7280; font-weight: 400; }
details { margin-top: 16px; }
details summary { cursor: pointer; padding: 8px; background: #f9fafb; border-radius: 4px; font-size: 14px; font-weight: 500; }
details summary:hover { background: #f3f4f6; }
details pre { background: #1f2937; color: #e5e7eb; padding: 16px; border-radius: 4px; overflow: auto; font-size: 12px; margin-top: 8px; }
.flag { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
.flag.pass { background: #d1fae5; color: #065f46; }
.flag.fail { background: #fee2e2; color: #991b1b; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>k6 Benchmark Report</h1>
    <div class="meta">Generated: $(date -Iseconds 2>/dev/null || date) | Source: ${basename}</div>
  </div>

  <div class="section">
    <h2>HTTP Request Duration</h2>
    <table>
      <thead>
        <tr><th>Metric</th><th>Value</th><th>Target</th><th>Status</th></tr>
      </thead>
      <tbody>
        <tr>
          <td>p50</td><td>${p50} ms</td><td>&lt;${P50_TARGET}ms</td><td><span class="flag ${p50_status}">${p50_status}</span></td>
        </tr>
        <tr>
          <td>p95</td><td>${p95} ms</td><td>&lt;${P95_TARGET}ms</td><td><span class="flag ${p95_status}">${p95_status}</span></td>
        </tr>
        <tr>
          <td>p99</td><td>${p99} ms</td><td>&lt;${P99_TARGET}ms</td><td><span class="flag ${p99_status}">${p99_status}</span></td>
        </tr>
        <tr><td>Average</td><td>${avg} ms</td><td>-</td><td>-</td></tr>
        <tr><td>Median</td><td>${med} ms</td><td>-</td><td>-</td></tr>
        <tr><td>Min</td><td>${min} ms</td><td>-</td><td>-</td></tr>
        <tr><td>Max</td><td>${max} ms</td><td>-</td><td>-</td></tr>
      </tbody>
    </table>
  </div>

  <div class="section">
    <h2>Throughput</h2>
    <table>
      <tbody>
        <tr><td><strong>Requests/sec</strong></td><td>${rps}</td></tr>
        <tr><td><strong>Total Requests</strong></td><td>${total_reqs}</td></tr>
        <tr><td><strong>Virtual Users</strong></td><td>${vus}</td></tr>
        <tr><td><strong>Dropped Iterations</strong></td><td>${dropped}</td></tr>
      </tbody>
    </table>
  </div>

  <div class="section">
    <h2>Checks</h2>
    <table>
      <tbody>
        <tr><td><strong>Passed</strong></td><td class="pass">${check_passes}</td></tr>
        <tr><td><strong>Failed</strong></td><td class="fail">${check_fails}</td></tr>
      </tbody>
    </table>
  </div>

  <div class="section">
    <h2>Raw JSON Data</h2>
    <details>
      <summary>Click to expand raw k6 summary JSON</summary>
      <pre>${raw_json}</pre>
    </details>
  </div>
</div>
</body>
</html>
HTMLEOF

        print_success "Generated HTML: $html_file"
    done
}

run_all_scenarios() {
    local total=0
    local passed=0
    local failed=0

    print_section "Running All Scenarios"

    local scenario_count=${#SCENARIOS[@]}
    local idx=0

    for key in "${!SCENARIOS[@]}"; do
        print_info "Running scenario: $key"
        ((total+=1))
        if run_scenario "$key" "${SCENARIOS[$key]}"; then
            ((passed+=1))
        else
            ((failed+=1))
        fi
        ((idx+=1))

        # Cooldown between scenarios (skip after the last one)
        if [ "$COOLDOWN" -gt 0 ] && [ "$idx" -lt "$scenario_count" ]; then
            print_info "Cooldown: waiting ${COOLDOWN}s for system recovery..."
            sleep "$COOLDOWN"
            # Verify services are still healthy before next test
            if ! curl -sf "$TARGET_URL/health" > /dev/null 2>&1; then
                print_warning "API not responding after cooldown, waiting additional 10s..."
                sleep 10
            fi
        fi
    done

    print_section "Final Summary"
    echo "  Total Scenarios:     $total"
    echo -e "  Passed:              ${GREEN}${passed}${NC}"
    if [ $failed -gt 0 ]; then
        echo -e "  Failed:              ${RED}${failed}${NC}"
    else
        echo "  Failed:              $failed"
    fi
    echo ""
    echo -e "Results directory: ${CYAN}$RESULTS_DIR${NC}"

    if [ $failed -eq 0 ]; then
        echo ""
        print_success "All benchmarks PASSED ✓"
        return 0
    else
        echo ""
        print_error "Some benchmarks failed"
        return 1
    fi
}

main() {
    parse_args "$@"

    # Validate tier if specified
    if [ -n "$TIER" ]; then
        local valid_tiers="smoke low medium high expert"
        if ! echo " $valid_tiers " | grep -q " $TIER "; then
            print_error "Invalid tier: $TIER"
            print_info "Valid tiers: $valid_tiers"
            exit 1
        fi
        print_info "Load tier: $TIER"
    fi

    # Auto-calculate maxVUs from RPS if --vus not provided
    # This fixes the ~500 RPS cap caused by default maxVUs=50 being insufficient for higher RPS targets
    # When --tier is active, per-scenario auto-calculation happens in run_scenario()
    if [ -z "$VUS" ] && [ -z "$TIER" ]; then
        VUS=$(( RPS * 2 ))
        if [ "$VUS" -lt 50 ]; then
            VUS=50
        fi
        print_info "Auto-calculated maxVUs: $VUS (RPS * 2)"
        print_info "Override with --vus for precise control"
    elif [ -z "$VUS" ] && [ -n "$TIER" ]; then
        print_info "VUs will be auto-calculated per scenario based on tier profile"
    fi

    if [ "$LIST_ONLY" = true ]; then
        list_scenarios
    fi

    print_header

    print_info "Configuration:"
    echo "  Target URL:  $TARGET_URL"
    if [ -n "$TIER" ]; then
        echo "  Load Tier:   $TIER"
        if [ "$EXPLICIT_RPS" = true ]; then
            echo "  RPS:         $RPS (explicit override, ignoring tier)"
        else
            echo "  RPS:         per-scenario (from tier)"
        fi
        if [ "$EXPLICIT_DURATION" = true ]; then
            echo "  Duration:    $DURATION (explicit override, ignoring tier)"
        else
            echo "  Duration:    per-scenario (from tier)"
        fi
    else
        echo "  Default RPS:  $RPS"
    fi
    echo "  Max VUs:  $VUS"
    echo "  Target Latency: p50<${P50_TARGET}ms, p95<${P95_TARGET}ms, p99<${P99_TARGET}ms"
    echo ""

    check_requirements
    check_services
    create_results_dir

    if [ -n "$SCENARIO" ]; then
        # Run single scenario
        if [[ ${SCENARIOS[$SCENARIO]+_} ]]; then
            if ! run_scenario "$SCENARIO" "${SCENARIOS[$SCENARIO]}"; then
                print_error "Scenario '$SCENARIO' failed"
            fi
        else
            print_error "Unknown scenario: $SCENARIO"
            print_info "Available scenarios: ${!SCENARIOS[@]}"
            exit 1
        fi
    else
        # Run all scenarios
        run_all_scenarios
    fi

    # Generate HTML report if requested
    if [ "$WITH_HTML_REPORT" = true ]; then
        generate_html_report
    fi

    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${BOLD}Benchmark Run Complete${NC}                                           ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

main "$@"
