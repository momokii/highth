// Scenario 6: Primary-Key Hot Lookup (Bundled)
//
// Benchmarks raw PostgreSQL hot-path performance via single-row primary key lookups.
// - Uses B-tree index scan on sensor_readings.id (BIGSERIAL PRIMARY KEY)
// - Dynamic ID range detection via /api/v1/stats to guarantee all IDs exist
// - Tightest latency thresholds in the suite (simplest possible query)
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ===== CONFIGURATION (inline from lib/config.js) =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

// ===== HELPER FUNCTIONS =====

// ===== API ENDPOINT FUNCTIONS =====
function getSensorReadingById(id) {
    const url = `${BASE_URL}/api/v1/sensor-readings?id=${id}`;

    const params = {
        headers: { 'Accept': 'application/json' },
        tags: {
            name: 'SensorReadingByID',
            endpoint: 'pk-lookup',
        },
    };

    return http.get(url, params);
}

function getStats() {
    const url = `${BASE_URL}/api/v1/stats`;
    const params = {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'Stats' },
    };
    return http.get(url, params);
}

// ===== CUSTOM METRICS =====
const pkLookupLatency = new Trend('pk_lookup_latency');
const pkSuccessRate = new Rate('pk_success_rate');

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    pk_lookup: {
      executor: 'constant-arrival-rate',
      rate: parseInt(__ENV.CUSTOM_RPS || '50'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 10,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 50,
      startTime: '0s',
    },
  },
  thresholds: {
    // Tightest thresholds in the suite — PK lookup is the simplest possible query
    // (single-row B-tree index scan, no joins, no sorting)
    'http_req_duration': [
      'p(50)<50',    // sub-50ms median expected for index scan
      'p(95)<100',   // sub-100ms p95
      'p(99)<200',   // sub-200ms p99
    ],
    'http_req_failed': ['rate<0.01'],              // error rate < 1%
    'pk_lookup_latency': ['p(95)<100'],            // custom PK latency tracking
    'pk_success_rate': ['rate>0.99'],              // >99% success rate
  },
};

// ===== SETUP FUNCTION =====
// Queries the stats endpoint to get total_readings (which equals MAX(id) for
// BIGSERIAL with bulk insert — no gaps). This guarantees all generated IDs exist.
export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 6: Primary-Key Hot Lookup');
  console.log('='.repeat(60));

  // Query stats endpoint to determine the live ID range
  const statsResponse = getStats();
  let maxID = 100000000; // fallback default

  if (statsResponse.status === 200) {
    try {
      const body = statsResponse.json();
      // Stats returns { data: { total_readings: N, ... } }
      const totalReadings = body.data && body.data.total_readings;
      if (totalReadings && totalReadings > 0) {
        maxID = totalReadings;
        console.log(`Detected live dataset: ${maxID} rows (maxID = ${maxID})`);
      } else {
        console.log(`Warning: Could not parse total_readings from stats response, using fallback maxID=${maxID}`);
      }
    } catch (e) {
      console.log(`Warning: Failed to parse stats response: ${e.message}, using fallback maxID=${maxID}`);
    }
  } else {
    console.log(`Warning: Stats endpoint returned status ${statsResponse.status}, using fallback maxID=${maxID}`);
  }

  console.log(`ID range: [1, ${maxID}]`);
  console.log('='.repeat(60));

  return { maxID: maxID };
}

// ===== MAIN TEST FUNCTION =====
export default function (data) {
  // Generate random ID uniformly in [1, maxID]
  const randomId = Math.floor(Math.random() * data.maxID) + 1;

  const startTime = Date.now();
  const response = getSensorReadingById(randomId);
  const duration = Date.now() - startTime;

  pkLookupLatency.add(duration);

  const isSuccess = response.status === 200;
  pkSuccessRate.add(isSuccess ? 1 : 0);

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 100ms': (r) => r.timings.duration < 100,
    'has data object': (r) => {
      try {
        const body = r.json();
        return body.data !== null && typeof body.data === 'object';
      } catch {
        return false;
      }
    },
    'data has id field': (r) => {
      try {
        const body = r.json();
        return body.data && body.data.id === String(randomId);
      } catch {
        return false;
      }
    },
    'has cache status header': (r) => {
      const cacheStatus = r.headers['X-Cache-Status'];
      return cacheStatus === 'HIT' || cacheStatus === 'MISS';
    },
  });
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Primary-Key Hot Lookup Test Complete');
  console.log('='.repeat(60));
}
