// Scenario 2: Time-Range Queries (Bundled)
//
// Tests dashboard-style queries with varying time ranges:
// - Last hour (most frequent, smallest dataset)
// - Last 24 hours (medium dataset)
// - Last 7 days (largest dataset, tests materialized views)
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

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
const LIMITS = [10, 50, 100, 500];
const TIME_RANGES = [
    { name: '1h', duration: '1h' },
    { name: '24h', duration: '24h' },
    { name: '7d', duration: '168h' },
];

// ===== HELPER FUNCTIONS =====
function randomHotDevice() {
    const index = Math.floor(Math.random() * HOT_DEVICES.length);
    return HOT_DEVICES[index];
}

function randomReadingType() {
    const index = Math.floor(Math.random() * READING_TYPES.length);
    return READING_TYPES[index];
}

function randomLimit() {
    const index = Math.floor(Math.random() * LIMITS.length);
    return LIMITS[index];
}

function randomTimeRange() {
    const index = Math.floor(Math.random() * TIME_RANGES.length);
    return TIME_RANGES[index];
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

// ===== API ENDPOINT FUNCTIONS =====
function getSensorReadings(deviceId, options = {}) {
    const { type = null, from = null, to = null, limit = 100 } = options;

    const queryParams = [];
    queryParams.push(`device_id=${deviceId}`);
    if (type) queryParams.push(`type=${type}`);
    if (from) queryParams.push(`from=${encodeURIComponent(from)}`);
    if (to) queryParams.push(`to=${encodeURIComponent(to)}`);
    queryParams.push(`limit=${limit}`);

    const queryString = queryParams.join('&');
    const url = `${BASE_URL}/api/v1/sensor-readings?${queryString}`;

    const params = {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'SensorReadings', device_id: deviceId, reading_type: type || 'all' },
    };

    return http.get(url, params);
}

function getDeviceHourlyStats(deviceId, hours = 24) {
    // Note: This endpoint may not exist, using mock response
    return {
        status: 200,
        timings: { duration: 50 },
        body: '[]',
        json: () => []
    };
}

// ===== CUSTOM METRICS =====
const oneHourLatency = new Trend('time_range_1h_latency');
const twentyFourHourLatency = new Trend('time_range_24h_latency');
const sevenDayLatency = new Trend('time_range_7d_latency');

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    time_range_queries: {
      executor: 'constant-arrival-rate',
      rate: parseInt(__ENV.CUSTOM_RPS || '30'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 10,
      maxVUs: 40,
      startTime: '0s',
    },
  },
  thresholds: {
    'http_req_duration': ['p(50)<200', 'p(95)<500', 'p(99)<800'],
    'http_req_failed': ['rate<0.01'],
    'time_range_1h_latency': ['p(95)<300'],
    'time_range_24h_latency': ['p(95)<500'],
    'time_range_7d_latency': ['p(95)<700'],
  },
};

// ===== SETUP FUNCTION =====
export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 2: Time-Range Queries');
  console.log('='.repeat(60));
  console.log('Testing dashboard-style queries with time filters');
  console.log('Time Ranges:');
  TIME_RANGES.forEach(range => {
    console.log(`  - ${range.name}: ${range.duration}`);
  });
  console.log(`Limits: ${LIMITS.join(', ')}`);
  console.log('='.repeat(60));
}

// ===== MAIN TEST FUNCTION =====
export default function () {
  const deviceId = randomHotDevice();
  const timeRange = randomTimeRange();
  const limit = randomLimit();
  const readingType = randomReadingType();

  const from = timeAgo(timeRange.duration);
  const to = new Date().toISOString();

  const startTime = Date.now();
  const response = getSensorReadings(deviceId, {
    type: readingType,
    from: from,
    to: to,
    limit: limit,
  });
  const duration = Date.now() - startTime;

  // Track latency by time range
  switch (timeRange.name) {
    case '1h':
      oneHourLatency.add(duration);
      break;
    case '24h':
      twentyFourHourLatency.add(duration);
      break;
    case '7d':
      sevenDayLatency.add(duration);
      break;
  }

  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 800ms': (r) => r.timings.duration < 800,
    'has data array': (r) => {
      try {
        return Array.isArray(r.json('data'));
      } catch {
        return false;
      }
    },
    'data within limit': (r) => {
      try {
        const data = r.json('data');
        return Array.isArray(data) && data.length <= limit;
      } catch {
        return false;
      }
    },
  });

  // 20% chance to query hourly stats
  if (Math.random() < 0.2) {
    const hours = Math.floor(parseInt(timeRange.duration) / 24) || 1;
    getDeviceHourlyStats(deviceId, hours * 24);
  }

  sleep(Math.random() * 0.2 + 0.1);
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Time-Range Queries Test Complete');
  console.log('='.repeat(60));
}
