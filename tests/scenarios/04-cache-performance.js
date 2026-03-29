// Scenario 4: Cache Performance (Bundled)
//
// Tests Redis cache effectiveness across three phases:
// - Phase 1: Cold cache (all database hits)
// - Phase 2: Warm cache (populating, mixed hits/misses)
// - Phase 3: Hot cache (high hit rate)
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ===== CONFIGURATION =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const HOT_DEVICE_COUNT = 20;

function generateHotDevices() {
    const devices = [];
    for (let i = 1; i <= HOT_DEVICE_COUNT; i++) {
        const deviceId = String(i).padStart(6, '0');
        devices.push(`sensor-${deviceId}`);
    }
    return devices;
}

const HOT_DEVICES = generateHotDevices();
const READING_TYPES = ['temperature', 'humidity', 'pressure'];

// ===== HELPER FUNCTIONS =====
function randomHotDevice() {
    const index = Math.floor(Math.random() * HOT_DEVICES.length);
    return HOT_DEVICES[index];
}

function randomReadingType() {
    const index = Math.floor(Math.random() * READING_TYPES.length);
    return READING_TYPES[index];
}

// ===== API ENDPOINT FUNCTIONS =====
function getSensorReadings(deviceId, options = {}) {
    const { type = null, limit = 100 } = options;

    const queryParams = [];
    queryParams.push(`device_id=${deviceId}`);
    if (type) queryParams.push(`type=${type}`);
    queryParams.push(`limit=${limit}`);

    const queryString = queryParams.join('&');
    const url = `${BASE_URL}/api/v1/sensor-readings?${queryString}`;

    const params = {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'SensorReadings', device_id: deviceId, reading_type: type || 'all' },
    };

    return http.get(url, params);
}

// ===== CUSTOM METRICS =====
const coldCacheLatency = new Trend('cold_cache_latency');
const warmCacheLatency = new Trend('warm_cache_latency');
const hotCacheLatency = new Trend('hot_cache_latency');
const cacheHitRate = new Rate('cache_hit_rate');

// Track current phase
let currentPhase = 'cold';
let testStartTime = null;

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    cache_performance: {
      executor: 'constant-arrival-rate',
      rate: parseInt(__ENV.CUSTOM_RPS || '50'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '45s', // Total 45 seconds (15s per phase for testing)
      preAllocatedVUs: 10,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 50,
      startTime: '0s',
    },
  },
  thresholds: {
    'http_req_duration': ['p(50)<10', 'p(95)<50', 'p(99)<100'],
    'http_req_failed': ['rate<0.01'],
    'cold_cache_latency': ['p(95)<100'],
    'warm_cache_latency': ['p(95)<50'],
    'hot_cache_latency': ['p(95)<20'],
    'cache_hit_rate': ['rate>0.6'],
  },
};

// ===== SETUP FUNCTION =====
export function setup() {
  testStartTime = new Date();
  console.log('='.repeat(60));
  console.log('Scenario 4: Cache Performance');
  console.log('='.repeat(60));
  console.log('Testing Redis cache effectiveness');
  console.log('Phase 1 (0-33%): Cold Cache - All cache misses');
  console.log('Phase 2 (33-66%): Warm Cache - Populating cache');
  console.log('Phase 3 (66-100%): Hot Cache - High cache hit rate');
  console.log('='.repeat(60));
  console.log('');
  console.log('NOTE: For accurate results, ensure Redis is flushed before test:');
  console.log('  docker exec highth-redis redis-cli FLUSHALL');
  console.log('='.repeat(60));
}

// ===== MAIN TEST FUNCTION =====
export default function () {
  // Determine current phase based on elapsed time
  // Phase boundaries are dynamic: 0-33%, 33-66%, 66-100% of total duration
  const totalDuration = parseInt(__ENV.CUSTOM_DURATION || '45') * 1000; // Convert to ms
  const elapsed = new Date() - testStartTime;
  const phaseDuration = totalDuration / 3;

  if (elapsed < phaseDuration) {
    currentPhase = 'cold';
  } else if (elapsed < phaseDuration * 2) {
    currentPhase = 'warm';
  } else {
    currentPhase = 'hot';
  }

  const deviceId = randomHotDevice();
  const readingType = randomReadingType();

  const startTime = Date.now();
  const response = getSensorReadings(deviceId, {
    type: readingType,
    limit: 100,
  });
  const duration = Date.now() - startTime;

  // Track latency by phase
  switch (currentPhase) {
    case 'cold':
      coldCacheLatency.add(duration);
      break;
    case 'warm':
      warmCacheLatency.add(duration);
      break;
    case 'hot':
      hotCacheLatency.add(duration);
      break;
  }

  // Check cache status from response headers (if available)
  const cacheStatus = response.headers['X-Cache-Status'] || 'unknown';
  if (cacheStatus === 'HIT') {
    cacheHitRate.add(1);
  } else {
    cacheHitRate.add(0);
  }

  check(response, {
    'status is 200': (r) => r.status === 200,
    'has data': (r) => {
      try {
        return Array.isArray(r.json('data'));
      } catch {
        return false;
      }
    },
  });

  // Log phase transitions (only once per VU)
  const iter = __ITER || 0;
  if (iter === 0) {
    console.log(`VU ${__VU}: Starting in ${currentPhase.toUpperCase()} phase`);
  }

  sleep(0.01);
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Cache Performance Test Complete');
  console.log('='.repeat(60));
  console.log('Phase Analysis:');
  console.log('  Cold Cache: All database hits (baseline performance)');
  console.log('  Warm Cache: Cache populating, improving performance');
  console.log('  Hot Cache: High cache hit rate, best performance');
  console.log('');
  console.log('Target: Hot cache p95 < 100ms, hit rate > 90%');
  console.log('='.repeat(60));
}
