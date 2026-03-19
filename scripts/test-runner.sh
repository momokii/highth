#!/bin/bash
# Load testing script for IoT Sensor Query API
# Uses Vegeta HTTP load testing tool

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_BASE_URL="${API_URL:-http://localhost:8080}"
RESULTS_DIR="test-results/$(date +%Y%m%d_%H%M%S)"
VEGETA_BIN="${VEGETA_BIN:-vegeta}"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Test results tracking
declare -A TEST_RESULTS

# Check if vegeta is installed
if ! command -v "$VEGETA_BIN" &> /dev/null; then
    echo -e "${RED}Error: vegeta not found${NC}"
    echo "Please install vegeta: go install github.com/tsenart/vegeta@latest"
    exit 1
fi

echo -e "${GREEN}IoT Sensor Query API - Load Testing${NC}"
echo "API Base URL: $API_BASE_URL"
echo "Results Directory: $RESULTS_DIR"
echo ""

# Test scenarios
run_health_test() {
    echo -e "${YELLOW}Running Health Check Test...${NC}"

    echo "GET ${API_BASE_URL}/health" | \
        "$VEGETA_BIN" attack -duration=10s -rate=1 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/health.txt"

    cat "$RESULTS_DIR/health.txt"

    # Check pass criteria: p50 <= 10ms
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/health.txt" || echo "0")
    if (( $(echo "$p50 <= 10" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 10ms${NC}"
        TEST_RESULTS[health_check]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms > 10ms${NC}"
        TEST_RESULTS[health_check]="FAIL"
    fi
    echo ""
}

run_cold_start_test() {
    echo -e "${YELLOW}Running Cold Start Test (Cache Flush)...${NC}"

    # First flush the Redis cache
    echo "Flushing Redis cache..."
    docker exec highth-redis redis-cli -p 6380 FLUSHALL > /dev/null 2>&1 || true

    # Then immediately hit the API
    echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000001&limit=10" | \
        "$VEGETA_BIN" attack -duration=10s -rate=1 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/cold_start.txt"

    cat "$RESULTS_DIR/cold_start.txt"

    # Check pass criteria: p50 <= 600ms
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/cold_start.txt" || echo "0")
    if (( $(echo "$p50 <= 600" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 600ms${NC}"
        TEST_RESULTS[cold_start]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms > 600ms${NC}"
        TEST_RESULTS[cold_start]="FAIL"
    fi
    echo ""
}

run_baseline_test() {
    echo -e "${YELLOW}Running Baseline Test (warm cache, 1 RPS)...${NC}"

    # Warm up cache first
    curl -s "${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000001&limit=10" > /dev/null

    echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000001&limit=10" | \
        "$VEGETA_BIN" attack -duration=30s -rate=1 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/baseline.txt"

    cat "$RESULTS_DIR/baseline.txt"

    # Check pass criteria: p50 <= 50ms
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/baseline.txt" || echo "0")
    if (( $(echo "$p50 <= 50" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 50ms${NC}"
        TEST_RESULTS[baseline]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms > 50ms${NC}"
        TEST_RESULTS[baseline]="FAIL"
    fi
    echo ""
}

run_concurrent_test() {
    echo -e "${YELLOW}Running Concurrent Test (50 RPS, 60s) - PRIMARY TEST...${NC}"

    # Create targets file with multiple devices for realistic load
    local targets_file="$RESULTS_DIR/concurrent_targets.txt"
    > "$targets_file"

    # Use multiple devices for realistic load distribution
    for i in {1..20}; do
        device_id=$(printf "sensor-%06d" $((RANDOM % 1000)))
        echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=${device_id}&limit=10" >> "$targets_file"
    done

    # Shuffle targets for variety
    shuf "$targets_file" -o "$targets_file"

    cat "$targets_file" | \
        "$VEGETA_BIN" attack -duration=60s -rate=50 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/concurrent.txt"

    cat "$RESULTS_DIR/concurrent.txt"

    # Check pass criteria: p50 <= 500ms, p95 <= 800ms (PRIMARY TARGET)
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/concurrent.txt" || echo "0")
    local p95=$(grep -oP 'Latencies.*mean, [0-9.]+, \K[0-9.]+' "$RESULTS_DIR/concurrent.txt" || echo "0")

    echo -e "${GREEN}p50=${p50}ms, p95=${p95}ms${NC}"

    if (( $(echo "$p50 <= 500" | bc -l 2>/dev/null || echo "0") )) && (( $(echo "$p95 <= 800" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 500ms, p95=${p95}ms <= 800ms${NC}"
        TEST_RESULTS[concurrent]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms (target: 500ms), p95=${p95}ms (target: 800ms)${NC}"
        TEST_RESULTS[concurrent]="FAIL"
    fi

    # Check error rate
    local success=$(grep -oP 'Success\s+\[ratio\]\s+\K[0-9.]+' "$RESULTS_DIR/concurrent.txt" || echo "0")
    local error_rate=$(echo "100 - $success" | bc 2>/dev/null || echo "0")
    if (( $(echo "$error_rate <= 1" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ Error rate ${error_rate}% <= 1%${NC}"
    else
        echo -e "${YELLOW}✗ Error rate ${error_rate}% > 1%${NC}"
    fi
    echo ""
}

run_hot_device_test() {
    echo -e "${YELLOW}Running Hot Device Test (90% to same device, 50 RPS)...${NC}"

    local targets_file="$RESULTS_DIR/hot_targets.txt"

    # Create 90% requests to hot device, 10% to others
    > "$targets_file"
    for i in {1..90}; do
        echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000001&limit=10" >> "$targets_file"
    done
    for i in {1..10}; do
        echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000002&limit=10" >> "$targets_file"
    done

    shuf "$targets_file" -o "$targets_file"

    cat "$targets_file" | \
        "$VEGETA_BIN" attack -duration=30s -rate=50 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/hot_device.txt"

    cat "$RESULTS_DIR/hot_device.txt"

    # Check pass criteria: p50 <= 500ms, p99 <= 2x p95
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/hot_device.txt" || echo "0")
    local p95=$(grep -oP 'Latencies.*mean, [0-9.]+, \K[0-9.]+' "$RESULTS_DIR/hot_device.txt" || echo "0")
    local p99=$(grep -oP 'Latencies.*mean, [0-9.]+, [0-9.]+, \K[0-9.]+' "$RESULTS_DIR/hot_device.txt" || echo "0")

    local max_p99=$(echo "$p95 * 2" | bc 2>/dev/null || echo "999999")

    if (( $(echo "$p50 <= 500" | bc -l 2>/dev/null || echo "0") )) && (( $(echo "$p99 <= $max_p99" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 500ms, p99=${p99}ms <= 2x p95 (${p95}ms)${NC}"
        TEST_RESULTS[hot_device]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms, p99=${p99}ms, 2x p95=${max_p99}ms${NC}"
        TEST_RESULTS[hot_device]="FAIL"
    fi
    echo ""
}

run_large_n_test() {
    echo -e "${YELLOW}Running Large N Test (limit=500, 10 RPS)...${NC}"

    echo "GET ${API_BASE_URL}/api/v1/sensor-readings?device_id=sensor-000001&limit=500" | \
        "$VEGETA_BIN" attack -duration=30s -rate=10 | \
        "$VEGETA_BIN" report -type=text > "$RESULTS_DIR/large_n.txt"

    cat "$RESULTS_DIR/large_n.txt"

    # Check pass criteria: p50 <= 500ms
    local p50=$(grep -oP 'Latencies.*mean, \K[0-9.]+' "$RESULTS_DIR/large_n.txt" || echo "0")
    if (( $(echo "$p50 <= 500" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "${GREEN}✓ PASSED: p50=${p50}ms <= 500ms${NC}"
        TEST_RESULTS[large_n]="PASS"
    else
        echo -e "${YELLOW}✗ FAILED: p50=${p50}ms > 500ms${NC}"
        TEST_RESULTS[large_n]="FAIL"
    fi
    echo ""
}

analyze_results() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Test Results Summary${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Results Directory: $RESULTS_DIR"
    echo ""

    # Display summary table
    echo -e "${YELLOW}Test Results Summary${NC}"
    echo ""
    printf "%-20s %-10s %s\n" "Test" "Status" "Notes"
    printf "%-20s %-10s %s\n" "----" "------" "-----"

    for test_name in "Health Check" "Cold Start" "Baseline" "Concurrent" "Hot Device" "Large N"; do
        local key=$(echo "$test_name" | tr '[:upper:]' '[:lower:]' | sed 's/ /_/g')
        local status="${TEST_RESULTS[$key]:-NOT_RUN}"
        local icon="✓"
        local color="${GREEN}"

        if [ "$status" = "FAIL" ]; then
            icon="✗"
            color="${YELLOW}"
        elif [ "$status" = "NOT_RUN" ]; then
            icon="○"
            color="${NC}"
        fi

        printf "%-20s ${color}%-10s${NC} %s\n" "$test_name" "${icon} ${status}"
    done

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}PRIMARY TEST: Concurrent Load${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo "Target: p50 ≤ 500ms, p95 ≤ 800ms"
    echo ""

    if [ -f "$RESULTS_DIR/concurrent.txt" ]; then
        echo "Concurrent Test Results:"
        grep -A 2 "Latencies" "$RESULTS_DIR/concurrent.txt" || true
        grep "Success" "$RESULTS_DIR/concurrent.txt" || true
    fi

    echo ""
    echo -e "${GREEN}Full results available in:${NC} $RESULTS_DIR"
    echo ""
}

# Main execution
main() {
    case "${1:-all}" in
        health)
            run_health_test
            ;;
        cold)
            run_cold_start_test
            ;;
        baseline)
            run_baseline_test
            ;;
        concurrent)
            run_concurrent_test
            ;;
        hot)
            run_hot_device_test
            ;;
        large)
            run_large_n_test
            ;;
        analyze)
            analyze_results
            ;;
        all)
            run_health_test
            run_cold_start_test
            run_baseline_test
            run_concurrent_test
            run_hot_device_test
            run_large_n_test
            analyze_results
            ;;
        *)
            echo "Usage: $0 {health|cold|baseline|concurrent|hot|large|analyze|all}"
            echo ""
            echo "Test scenarios:"
            echo "  health      - Quick health check test"
            echo "  cold        - Cold start test (flushes cache first)"
            echo "  baseline    - Low load test (10 RPS)"
            echo "  concurrent  - Main test (100 RPS, 5 min)"
            echo "  hot         - Hot device test (single device, 50 RPS)"
            echo "  large       - Large result set test (limit=500)"
            echo "  analyze     - Analyze all results"
            echo "  all         - Run all tests"
            exit 1
            ;;
    esac
}

main "$@"
