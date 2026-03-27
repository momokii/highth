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

# Available scenarios
declare -A SCENARIOS=(
  ["hot"]="01-hot-device-pattern.js"
  ["time-range"]="02-time-range-queries.js"
  ["mixed"]="03-mixed-workload.js"
  ["cache"]="04-cache-performance.js"
)

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
                shift 2
                ;;
            --duration|-d)
                DURATION="$2"
                shift 2
                ;;
            --target-url|-u)
                TARGET_URL="$2"
                shift 2
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
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  -s, --scenario <name>     Run specific scenario (hot, time-range, mixed, cache)
  -r, --rps <number>        Requests per second (default: 50)
  -d, --duration <time>     Test duration (default: 2m)
  -u, --target-url <url>    API endpoint to test (default: http://localhost:8080)
  -l, --list                List available scenarios
  --skip-setup              Skip service health checks
  -v, --verbose             Enable verbose output
  -h, --help                Show this help message

Examples:
  $(basename "$0")                          # Run all scenarios
  $(basename "$0") --scenario hot           # Run hot device pattern test
  $(basename "$0") --rps 100 --duration 5m  # Custom load test
  $(basename "$0") --list                   # List scenarios

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
    echo "  Output:      $output_file"
    echo ""

    print_info "Running k6 test..."

    # Run k6 test directly (no eval, no string manipulation)
    docker run --rm --network host \
        -v "$SCRIPT_DIR:/tests" \
        -v "$RESULTS_DIR:/results" \
        -e TARGET_URL="$TARGET_URL" \
        -e CUSTOM_RPS="$RPS" \
        -e CUSTOM_DURATION="$DURATION" \
        grafana/k6:latest run \
        --summary-export="/results/summary_${timestamp}.json" \
        "/tests/scenarios/$scenario_file"

    local exit_code=$?

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
        local p50=$(jq -r '.metrics.http_req_duration.values["p(50)"] // "N/A"' "$summary_file")
        local p95=$(jq -r '.metrics.http_req_duration.values["p(95)"] // "N/A"' "$summary_file")
        local p99=$(jq -r '.metrics.http_req_duration.values["p(99)"] // "N/A"' "$summary_file")
        local rps=$(jq -r '.metrics.http_reqs.values["rate"] // "N/A"' "$summary_file")
        local errors=$(jq -r '.metrics.http_req_failed.values["rate"] // "N/A"' "$summary_file")

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
    print_info "Generating HTML reports..."

    # Use k6-to-html if available, otherwise use basic report
    if command -v k6-to-html &> /dev/null; then
        for json_file in "$RESULTS_DIR"/*.json; do
            if [ -f "$json_file" ]; then
                local html_file="${json_file%.json}.html"
                k6-to-html "$json_file" -o "$html_file"
                print_success "Generated: $html_file"
            fi
        done
    else
        print_warning "k6-to-html not installed. Install for HTML reports:"
        print_info "  npm install -g k6-to-html"
    fi
}

run_all_scenarios() {
    local total=0
    local passed=0
    local failed=0

    print_section "Running All Scenarios"

    for key in "${!SCENARIOS[@]}"; do
        print_info "Running scenario: $key"
        ((total+=1))
        if run_scenario "$key" "${SCENARIOS[$key]}"; then
            ((passed+=1))
        else
            ((failed+=1))
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

    if [ "$LIST_ONLY" = true ]; then
        list_scenarios
    fi

    print_header

    print_info "Configuration:"
    echo "  Target URL:  $TARGET_URL"
    echo "  Target Latency: p50<${P50_TARGET}ms, p95<${P95_TARGET}ms, p99<${P99_TARGET}ms"
    echo ""

    check_requirements
    check_services
    create_results_dir

    if [ -n "$SCENARIO" ]; then
        # Run single scenario
        if [[ ${SCENARIOS[$SCENARIO]+_} ]]; then
            run_scenario "$SCENARIO" "${SCENARIOS[$SCENARIO]}"
        else
            print_error "Unknown scenario: $SCENARIO"
            print_info "Available scenarios: ${!SCENARIOS[@]}"
            exit 1
        fi
    else
        # Run all scenarios
        run_all_scenarios
    fi

    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${BOLD}Benchmark Run Complete${NC}                                           ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

main "$@"
