#!/bin/bash

# test-runner.sh - Vegeta load testing script
# Run all 6 test scenarios and collect results

set -e

# Create results directory
RESULTS_DIR="test-results/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo "====================================="
echo "High-Performance IoT Sensor Query System"
echo "Load Testing with Vegeta"
echo "====================================="
echo ""
echo "Results directory: $RESULTS_DIR"
echo ""

# Configuration
API_BASE="http://localhost:8080"
DEVICE_1="sensor-001"
DEVICE_2="sensor-002"
DEVICE_3="sensor-003"

# Test 1: Health Check
echo "====================================="
echo "Test 1: Health Check"
echo "====================================="
echo "GET $API_BASE/health" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text > "$RESULTS_DIR/health.txt"
cat "$RESULTS_DIR/health.txt"
echo ""

# Test 2: Cold Start (flush cache first)
echo "====================================="
echo "Test 2: Cold Start (cache flushed)"
echo "====================================="
redis-cli FLUSHALL > /dev/null
echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=10" | \
  vegeta attack -duration=10s -rate=1 | \
  vegeta report -type=text > "$RESULTS_DIR/cold_start.txt"
cat "$RESULTS_DIR/cold_start.txt"
echo ""

# Test 3: Baseline (warm cache)
echo "====================================="
echo "Test 3: Baseline (warm cache)"
echo "====================================="
# Warm up cache
curl -s "$API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=10" > /dev/null
echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=10" | \
  vegeta attack -duration=30s -rate=1 | \
  vegeta report -type=text > "$RESULTS_DIR/baseline.txt"
cat "$RESULTS_DIR/baseline.txt"
echo ""

# Test 4: Concurrent Load Test (PRIMARY)
echo "====================================="
echo "Test 4: Concurrent Load Test (PRIMARY)"
echo "====================================="
# Create targets file with multiple devices
for i in {1..30}; do echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=10"; done
for i in {1..15}; do echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_2&limit=10"; done
for i in {1..5}; do echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_3&limit=10"; done | \
  shuf | \
  vegeta attack -duration=60s -rate=50 | \
  vegeta report -type=text > "$RESULTS_DIR/concurrent.txt"

cat "$RESULTS_DIR/concurrent.txt"
echo ""

# Test 5: Hot Device Test
echo "====================================="
echo "Test 5: Hot Device Test (90% same device)"
echo "====================================="
for i in {1..90}; do echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=10"; done
for i in {1..10}; do echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_2&limit=10"; done | \
  shuf | \
  vegeta attack -duration=30s -rate=50 | \
  vegeta report -type=text > "$RESULTS_DIR/hot_device.txt"

cat "$RESULTS_DIR/hot_device.txt"
echo ""

# Test 6: Large N Test (500 records)
echo "====================================="
echo "Test 6: Large N Test (500 records)"
echo "====================================="
echo "GET $API_BASE/api/v1/sensor-readings?device_id=$DEVICE_1&limit=500" | \
  vegeta attack -duration=30s -rate=10 | \
  vegeta report -type=text > "$RESULTS_DIR/large_n.txt"

cat "$RESULTS_DIR/large_n.txt"
echo ""

# Summary
echo "====================================="
echo "Test Summary"
echo "====================================="
echo ""
echo "Results saved to: $RESULTS_DIR"
echo ""
echo "Files:"
ls -la "$RESULTS_DIR"
echo ""
echo "Pass Criteria:"
echo "  Health Check:      p50 ≤ 10ms"
echo "  Cold Start:        p50 ≤ 600ms"
echo "  Baseline:          p50 ≤ 50ms"
echo "  Concurrent:        p50 ≤ 500ms, p95 ≤ 800ms (PRIMARY)"
echo "  Hot Device:        p50 ≤ 500ms"
echo "  Large N:           p50 ≤ 500ms"
echo ""
