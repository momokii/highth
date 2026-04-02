// Scenario 1: Hot Device Pattern (Bundled)
//
// Tests realistic IoT traffic distribution using Zipf's law:
// - 20% of devices receive 80% of the traffic
// - Validates caching effectiveness under uneven load
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

// ===== CONFIGURATION (inline from lib/config.js) =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const HOT_DEVICE_COUNT = 20;
const COLD_DEVICE_COUNT = 80;
const TOTAL_DEVICES = 100000;
const HOT_DEVICE_TRAFFIC = 0.8;

// Reading types and limits
const READING_TYPES = ['temperature', 'humidity', 'pressure'];
const LIMITS = [10, 50, 100, 500];

// Generate device IDs inline
function generateHotDevices() {
    const devices = [];
    for (let i = 1; i <= HOT_DEVICE_COUNT; i++) {
        const deviceId = String(i).padStart(6, '0');
        devices.push(`sensor-${deviceId}`);
    }
    return devices;
}

function generateColdDevices() {
    const devices = [];
    const start = HOT_DEVICE_COUNT + 1;  // 21
    const end = TOTAL_DEVICES;             // 100000
    const step = Math.floor((end - start) / COLD_DEVICE_COUNT);
    for (let i = 0; i < COLD_DEVICE_COUNT; i++) {
        const id = start + (i * step);
        const deviceId = String(id).padStart(6, '0');
        devices.push(`sensor-${deviceId}`);
    }
    return devices;
}

const HOT_DEVICES = generateHotDevices();
const COLD_DEVICES = generateColdDevices();
const ALL_DEVICES = [...HOT_DEVICES, ...COLD_DEVICES];

// ===== HELPER FUNCTIONS (inline from lib/helpers.js) =====
function randomHotDevice() {
    const index = Math.floor(Math.random() * HOT_DEVICES.length);
    return HOT_DEVICES[index];
}

function randomColdDevice() {
    const index = Math.floor(Math.random() * COLD_DEVICES.length);
    return COLD_DEVICES[index];
}

function zipfDevice() {
    const hotTraffic = Math.random() < HOT_DEVICE_TRAFFIC;
    if (hotTraffic) {
        return randomHotDevice();
    } else {
        return randomColdDevice();
    }
}

function randomReadingType() {
    const index = Math.floor(Math.random() * READING_TYPES.length);
    return READING_TYPES[index];
}

function randomLimit() {
    const index = Math.floor(Math.random() * LIMITS.length);
    return LIMITS[index];
}

function timeAgo(duration) {
    const now = new Date();
    const match = duration.match(/^(\d+)([hdm])$/);
    if (!match) {
        throw new Error(`Invalid duration format: ${duration}`);
    }
    const value = parseInt(match[1], 10);
    const unit = match[2];
    switch (unit) {
        case 'h':
            now.setHours(now.getHours() - value);
            break;
        case 'd':
            now.setDate(now.getDate() - value);
            break;
        case 'm':
            now.setMinutes(now.getMinutes() - value);
            break;
    }
    return now.toISOString();
}

// ===== API ENDPOINT FUNCTIONS (inline from lib/endpoints.js) =====
function getSensorReadings(deviceId, options = {}) {
    const {
        reading_type = null,
        from = null,
        to = null,
        limit = 100,
    } = options;

    const queryParams = [];
    queryParams.push(`device_id=${deviceId}`);
    if (reading_type) queryParams.push(`reading_type=${reading_type}`);
    if (from) queryParams.push(`from=${encodeURIComponent(from)}`);
    if (to) queryParams.push(`to=${encodeURIComponent(to)}`);
    queryParams.push(`limit=${limit}`);

    const queryString = queryParams.join('&');
    const url = `${BASE_URL}/api/v1/sensor-readings?${queryString}`;

    const params = {
        headers: { 'Accept': 'application/json' },
        tags: {
            name: 'SensorReadings',
            device_id: deviceId,
            reading_type: reading_type || 'all',
        },
    };

    return http.get(url, params);
}

// ===== CUSTOM METRICS =====
const hotDeviceLatency = new Trend('hot_device_latency');
const coldDeviceLatency = new Trend('cold_device_latency');

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    hot_device_pattern: {
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
    // Thresholds aligned with project SLO: p50<300ms, p95<500ms, p99<800ms
    // Aspirational "excellent" values tracked via custom Trend metrics (non-failing)
    'http_req_duration': [
      'p(50)<300',   // project target (excellent: <100ms)
      'p(95)<500',   // project target (excellent: <300ms)
      'p(99)<800',   // project target
    ],
    'http_req_failed': ['rate<0.01'],              // error rate < 1%
    'hot_device_latency': ['p(95)<300'],           // aligned with project p95 target
    'cold_device_latency': ['p(95)<500'],          // aligned with project p95 target
  },
};

// ===== SETUP FUNCTION =====
export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 1: Hot Device Pattern');
  console.log('='.repeat(60));
  console.log(`Total Devices: ${ALL_DEVICES.length}`);
  console.log(`Hot Devices (20%): ${HOT_DEVICES.length}`);
  console.log(`Cold Devices (80%): ${COLD_DEVICES.length}`);
  console.log(`Traffic Distribution: ${HOT_DEVICE_TRAFFIC * 100}% to hot devices`);
  console.log('='.repeat(60));
}

// ===== MAIN TEST FUNCTION =====
export default function () {
  const deviceId = zipfDevice();
  const isHotDevice = HOT_DEVICES.includes(deviceId);

  const readingType = randomReadingType();
  const limit = randomLimit();

  const startTime = Date.now();
  const response = getSensorReadings(deviceId, {
    reading_type: readingType,  // FIX: was 'type' which didn't match destructuring
    limit: limit,
  });
  const duration = Date.now() - startTime;

  if (isHotDevice) {
    hotDeviceLatency.add(duration);
  } else {
    coldDeviceLatency.add(duration);
  }

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'has data': (r) => {
      try {
        const data = r.json('data');
        return Array.isArray(data);
      } catch {
        return false;
      }
    },
  });
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Hot Device Pattern Test Complete');
  console.log('='.repeat(60));
}
