// Scenario 5: Stats and Aggregation (Bundled)
//
// Tests the /api/v1/stats endpoint exclusively.
// This endpoint queries a materialized view (mv_global_stats) and BYPASSES Redis cache.
// Purpose: Validate performance of aggregation queries under sustained load.
//
// Key characteristics:
// - Cache status is always "BYPASS" (no Redis involvement)
// - Queries mv_global_stats for total_readings and total_devices
// - Uses TABLESAMPLE SYSTEM (0.5) for device count estimation
// - Performance depends on materialized view freshness and PostgreSQL load
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ===== CONFIGURATION =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

// ===== CUSTOM METRICS =====
// Track stats endpoint latency separately from http_req_duration
const statsLatency = new Trend('stats_latency');
const statsSuccessRate = new Rate('stats_success_rate');

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    stats_and_aggregation: {
      executor: 'constant-arrival-rate',
      rate: parseInt(__ENV.CUSTOM_RPS || '20'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '60s',
      preAllocatedVUs: 10,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 40,
      startTime: '0s',
    },
  },
  thresholds: {
    // Hard gate: project-level SLO targets
    // NOTE: The /api/v1/stats endpoint bypasses cache and queries the materialized view.
    // Under heavy load, this can be slower than cached sensor readings.
    // Threshold aligned with project p99 target (800ms) to account for MV query overhead.
    'http_req_duration': ['p(50)<300', 'p(95)<500', 'p(99)<800'],
    'http_req_failed': ['rate<0.01'],
    // Stats-specific: should meet project target but track separately
    'stats_latency': ['p(95)<800'],
    'stats_success_rate': ['rate>0.99'],
  },
};

// ===== API ENDPOINT FUNCTIONS =====
function getStats() {
    const url = `${BASE_URL}/api/v1/stats`;
    const params = {
        headers: { 'Accept': 'application/json' },
        tags: {
            name: 'GetStats',
        },
    };
    return http.get(url, params);
}

// ===== SETUP FUNCTION =====
export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 5: Stats and Aggregation');
  console.log('='.repeat(60));
  console.log('Testing /api/v1/stats endpoint exclusively.');
  console.log('This endpoint queries mv_global_stats and BYPASSES Redis cache.');
  console.log('Cache status will always be "BYPASS".');
  console.log('='.repeat(60));
}

// ===== MAIN TEST FUNCTION =====
export default function () {
  const startTime = Date.now();
  const response = getStats();
  const duration = Date.now() - startTime;

  statsLatency.add(duration);

  // Check response
  const is200 = response.status === 200;
  statsSuccessRate.add(is200 ? 1 : 0);

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response has data object': (r) => {
      try {
        const data = r.json('data');
        return data !== null && typeof data === 'object';
      } catch {
        return false;
      }
    },
    'response has total_readings': (r) => {
      try {
        const data = r.json('data');
        return typeof data.total_readings === 'number';
      } catch {
        return false;
      }
    },
    'response has total_devices': (r) => {
      try {
        const data = r.json('data');
        return typeof data.total_devices === 'number';
      } catch {
        return false;
      }
    },
    'cache status is BYPASS': (r) => {
      return r.headers['X-Cache-Status'] === 'BYPASS';
    },
    'response time < 800ms': (r) => r.timings.duration < 800,
  });
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Stats and Aggregation Test Complete');
  console.log('='.repeat(60));
}
