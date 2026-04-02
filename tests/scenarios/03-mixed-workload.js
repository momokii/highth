// Scenario 3: Mixed Workload (Bundled)
//
// Tests real-world API usage patterns:
// - 10% health checks (lightweight)
// - 20% stats queries (moderate, uses materialized views)
// - 70% sensor readings (heavy, main workload)
//
// This is a bundled version with all dependencies inline.

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ===== CONFIGURATION =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const HOT_DEVICE_COUNT = 20;
const COLD_DEVICE_COUNT = 80;
const TOTAL_DEVICES = 100000;
const HOT_DEVICE_TRAFFIC = 0.8;
const WORKLOAD_MIX = {
    HEALTH_CHECK: 0.10,
    STATS: 0.20,
    SENSOR_READINGS: 0.70,
};

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
const READING_TYPES = ['temperature', 'humidity', 'pressure'];
const LIMITS = [10, 50, 100, 500];

// ===== HELPER FUNCTIONS =====
function zipfDevice() {
    const hotTraffic = Math.random() < HOT_DEVICE_TRAFFIC;
    if (hotTraffic) {
        const index = Math.floor(Math.random() * HOT_DEVICES.length);
        return HOT_DEVICES[index];
    } else {
        const index = Math.floor(Math.random() * COLD_DEVICES.length);
        return COLD_DEVICES[index];
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

function selectWorkloadType() {
    const rand = Math.random();
    const cumulative = WORKLOAD_MIX.HEALTH_CHECK + WORKLOAD_MIX.STATS + WORKLOAD_MIX.SENSOR_READINGS;
    const healthCheckRatio = WORKLOAD_MIX.HEALTH_CHECK / cumulative;
    const statsRatio = WORKLOAD_MIX.STATS / cumulative;

    if (rand < healthCheckRatio) {
        return 'health_check';
    } else if (rand < healthCheckRatio + statsRatio) {
        return 'stats';
    } else {
        return 'sensor_readings';
    }
}

// ===== API ENDPOINT FUNCTIONS =====
function getHealth() {
    const url = `${BASE_URL}/health`;
    const params = {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'HealthCheck' },
    };
    return http.get(url, params);
}

function getSensorReadings(deviceId, options = {}) {
    const { reading_type = null, from = null, to = null, limit = 100 } = options;

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
        tags: { name: 'SensorReadings', device_id: deviceId, reading_type: reading_type || 'all' },
    };

    return http.get(url, params);
}

function getStats() {
    const url = `${BASE_URL}/api/v1/stats`;
    const params = {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'GetStats' },
    };
    return http.get(url, params);
}

// ===== CUSTOM METRICS =====
const healthCheckLatency = new Trend('health_check_latency');
const statsLatency = new Trend('stats_query_latency');
const sensorReadingsLatency = new Trend('sensor_readings_latency');

const healthCheckRate = new Rate('health_check_rate');
const statsRate = new Rate('stats_rate');
const sensorReadingsRate = new Rate('sensor_readings_rate');

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    mixed_workload: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 10,
      maxVUs: 100,
      stages: [
        { duration: '10s', target: 25 },
        { duration: '10s', target: 50 },
        { duration: '10s', target: 100 },
        { duration: '10s', target: 50 },
        { duration: '10s', target: 0 },
      ],
      startTime: '0s',
    },
  },
  thresholds: {
    'http_req_duration': ['p(50)<20', 'p(95)<100', 'p(99)<300'],
    'http_req_failed': ['rate<0.02'],
    'health_check_latency': ['p(95)<20'],
    'stats_query_latency': ['p(95)<100'],
    'sensor_readings_latency': ['p(95)<200'],
  },
};

// Apply custom RPS if provided (scales the target rates)
if (__ENV.CUSTOM_RPS) {
    const customRPS = parseInt(__ENV.CUSTOM_RPS);
    const scale = customRPS / 100; // Base peak is 100 RPS
    options.scenarios.mixed_workload.stages.forEach(stage => {
        stage.target = Math.round(stage.target * scale);
    });
}

// Apply custom duration if provided
if (__ENV.CUSTOM_DURATION) {
    options.scenarios.mixed_workload.stages.forEach(stage => {
        stage.duration = __ENV.CUSTOM_DURATION;
    });
}

// Apply custom VUs if provided
if (__ENV.CUSTOM_VUS) {
    options.scenarios.mixed_workload.maxVUs = parseInt(__ENV.CUSTOM_VUS);
}

// ===== SETUP FUNCTION =====
export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 3: Mixed Workload');
  console.log('='.repeat(60));
  console.log('Workload Distribution:');
  console.log(`  - Health Checks: ${(WORKLOAD_MIX.HEALTH_CHECK * 100).toFixed(0)}%`);
  console.log(`  - Stats Queries: ${(WORKLOAD_MIX.STATS * 100).toFixed(0)}%`);
  console.log(`  - Sensor Readings: ${(WORKLOAD_MIX.SENSOR_READINGS * 100).toFixed(0)}%`);
  console.log('='.repeat(60));
}

// ===== MAIN TEST FUNCTION =====
export default function () {
  const workloadType = selectWorkloadType();
  const startTime = Date.now();
  let response;

  switch (workloadType) {
    case 'health_check':
      response = getHealth();
      healthCheckLatency.add(Date.now() - startTime);
      healthCheckRate.add(1);
      check(response, {
        'health check is fast': (r) => r.timings.duration < 100,
      });
      break;

    case 'stats':
      response = getStats();
      statsLatency.add(Date.now() - startTime);
      statsRate.add(1);
      check(response, {
        'stats query is fast': (r) => r.timings ? r.timings.duration < 300 : true,
        'stats query succeeds': (r) => r.status === 200,
      });
      break;

    case 'sensor_readings':
      const deviceId = zipfDevice();
      const readingType = randomReadingType();
      const limit = randomLimit();
      const from = new Date(Date.now() - 60 * 60 * 1000).toISOString(); // 1 hour ago
      const to = new Date().toISOString();

      response = getSensorReadings(deviceId, {
        reading_type: readingType,
        from: from,
        to: to,
        limit: limit,
      });
      sensorReadingsLatency.add(Date.now() - startTime);
      sensorReadingsRate.add(1);
      check(response, {
        'sensor readings < 500ms': (r) => r.timings.duration < 500,
        'sensor readings has data': (r) => {
          try {
            return Array.isArray(r.json('data'));
          } catch {
            return false;
          }
        },
      });
      break;
  }
}

// ===== TEARDOWN FUNCTION =====
export function teardown() {
  console.log('='.repeat(60));
  console.log('Mixed Workload Test Complete');
  console.log('='.repeat(60));
}
