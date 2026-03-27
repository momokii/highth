// Test Configuration
//
// Central configuration for all benchmark tests
// Defines endpoints, devices, thresholds, and test parameters

export const Config = {
  // API Configuration
  BASE_URL: __ENV.TARGET_URL || 'http://localhost:8080',
  TIMEOUT: '30s',

  // Test Parameters
  DEFAULT_RPS: 50,           // Default requests per second
  DEFAULT_DURATION: '60s',    // Default test duration
  MIN_VUS: 10,                // Minimum virtual users
  MAX_VUS: 100,              // Maximum virtual users

  // Latency Thresholds (milliseconds)
  THRESHOLDS: {
    P50: 300,   // 50th percentile (median)
    P95: 500,   // 95th percentile
    P99: 800,   // 99th percentile
  },

  // Device Configuration
  TOTAL_DEVICES: 100000,     // Total devices in database
  HOT_DEVICE_COUNT: 20,      // Number of "hot" devices (20%)
  COLD_DEVICE_COUNT: 80,     // Number of "cold" devices (80%)

  // Reading Types
  READING_TYPES: ['temperature', 'humidity', 'pressure'],

  // Query Limits
  LIMITS: [10, 50, 100, 500],

  // Time Ranges
  TIME_RANGES: [
    { name: '1h', duration: '1h' },
    { name: '24h', duration: '24h' },
    { name: '7d', duration: '168h' },
  ],

  // Hot Device Pattern (Zipf distribution)
  // 20% of devices get 80% of traffic
  HOT_DEVICE_TRAFFIC: 0.8,

  // Mixed Workload Distribution
  WORKLOAD_MIX: {
    HEALTH_CHECK: 0.10,    // 10% health checks
    STATS: 0.20,            // 20% stats queries
    SENSOR_READINGS: 0.70, // 70% sensor readings
  },

  // Cache Test Phases
  CACHE_PHASES: {
    COLD: '60s',   // Cold cache phase
    WARM: '60s',   // Warming up phase
    HOT: '60s',    // Warm cache phase
  },

  // Scenarios
  SCENARIOS: [
    '01-hot-device-pattern',
    '02-time-range-queries',
    '03-mixed-workload',
    '04-cache-performance',
  ],

  // Report Configuration
  REPORT_DIR: './test-results',
  HTML_REPORT: 'report.html',
  SUMMARY_REPORT: 'summary.json',
};

// Generate hot device IDs (top 20%)
export function generateHotDevices() {
  const devices = [];
  for (let i = 1; i <= Config.HOT_DEVICE_COUNT; i++) {
    const deviceId = String(i).padStart(6, '0');
    devices.push(`sensor-${deviceId}`);
  }
  return devices;
}

// Generate cold device IDs (bottom 80%)
export function generateColdDevices() {
  const devices = [];
  const start = Config.HOT_DEVICE_COUNT + 1;
  const end = Config.HOT_DEVICE_COUNT + Config.COLD_DEVICE_COUNT;
  for (let i = start; i <= end; i++) {
    const deviceId = String(i).padStart(6, '0');
    devices.push(`sensor-${deviceId}`);
  }
  return devices;
}

// All device IDs
export const HOT_DEVICES = generateHotDevices();
export const COLD_DEVICES = generateColdDevices();
export const ALL_DEVICES = [...HOT_DEVICES, ...COLD_DEVICES];
