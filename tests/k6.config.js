// k6 Configuration
//
// Global configuration for k6 benchmark tests
// Defines thresholds, options, and external resource requirements

import { Config } from './lib/config.js';

// k6 options
export const options = {
  // Key thresholds for all tests
  thresholds: {
    // HTTP request duration percentiles
    'http_req_duration': [
      'p(95)<500',  // 95% of requests must complete below 500ms
      'p(50)<300',  // 50% of requests must complete below 300ms
    ],
    // HTTP errors should be minimal
    'http_req_failed': ['rate<0.05'],  // Error rate must be below 5%
    // No DNS resolution failures
    'http_req_connecting': ['rate<0.1'],  // Connection failures < 10%
  },

  // Scenarios for multi-scenario tests
  scenarios: {
    // Default scenario configuration
    default: {
      executor: 'constant-arrival-rate',
      rate: Config.DEFAULT_RPS,
      timeUnit: '1s',
      duration: Config.DEFAULT_DURATION,
      preAllocatedVUs: Config.MIN_VUS,
      maxVUs: Config.MAX_VUS,
      exec: Config.SCENARIOS,
    },
  },

  // External resources (don't confuse these with your application metrics)
  ext: {
    // Load impact zones
    loadimpact: {
      projectID: 0, // LoadImpact project ID (not used)
      distribution: {
        'amazon:us:ashburn-1': { loadZone: 'amazon:us:ashburn-1', percent: 100 },
      },
    },
  },
};

// Global setup function - runs once before all tests
export function setup() {
  // Validate environment
  const targetUrl = __ENV.TARGET_URL || 'http://localhost:8080';

  // Log test start
  console.log('='.repeat(60));
  console.log('Higth IoT Benchmark Test Suite');
  console.log('='.repeat(60));
  console.log(`Target: ${targetUrl}`);
  console.log(`Test Duration: ${Config.DEFAULT_DURATION}`);
  console.log(`Target RPS: ${Config.DEFAULT_RPS}`);
  console.log(`Max VUs: ${Config.MAX_VUS}`);
  console.log('='.repeat(60));
}

// Global teardown function - runs once after all tests
export function teardown() {
  console.log('='.repeat(60));
  console.log('Benchmark Test Complete');
  console.log('HTML Report: results/report.html');
  console.log('='.repeat(60));
}
