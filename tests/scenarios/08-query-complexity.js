// Scenario 8: Query Complexity Tiers (Bundled)
//
// Tests each hardware path explicitly through 4 sub-scenarios:
//   1. PK Lookup    (?id=N)           → B-tree index scan      → CPU + memory
//   2. Device Filter (?device_id=X)    → Composite index scan   → CPU + memory
//   3. Time Range    (?device_id&from/to) → BRIN index scan     → Disk I/O
//   4. Stats Aggregation (/api/v1/stats) → Materialized view    → MV freshness
//
// Each sub-scenario uses a separate exported function via k6's `exec` option.
// This is the standard k6 pattern for multi-scenario tests.

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// ===== CONFIGURATION =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

// ===== CUSTOM METRICS (one per complexity tier) =====
const pkLookupLatency = new Trend('complexity_pk_lookup');
const deviceFilterLatency = new Trend('complexity_device_filter');
const timeRangeLatency = new Trend('complexity_time_range');
const statsAggLatency = new Trend('complexity_stats_agg');
const complexityErrors = new Rate('complexity_errors');

// ===== API HELPERS =====
function getStats() {
    return http.get(`${BASE_URL}/api/v1/stats`, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'Stats', tier: 'stats-agg' },
    });
}

// ===== TEST CONFIGURATION =====
// Each scenario uses `exec` to route to its own exported function.
export const options = {
  scenarios: {
    // Tier 1: PK Lookup — simplest query, should be fastest
    pk_lookup: {
      executor: 'constant-arrival-rate',
      exec: 'pkLookup',          // runs the pkLookup() function
      rate: parseInt(__ENV.CUSTOM_RPS || '30'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 5,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 30,
      startTime: '0s',
      tags: { complexity: 'pk-lookup' },
    },
    // Tier 2: Device Filter — composite index, small result set
    device_filter: {
      executor: 'constant-arrival-rate',
      exec: 'deviceFilter',      // runs the deviceFilter() function
      rate: parseInt(__ENV.CUSTOM_RPS || '30'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 5,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 30,
      startTime: '35s',  // starts after pk_lookup + cooldown
      tags: { complexity: 'device-filter' },
    },
    // Tier 3: Time Range — BRIN scan, larger result set, more disk I/O
    time_range: {
      executor: 'constant-arrival-rate',
      exec: 'timeRange',         // runs the timeRange() function
      rate: parseInt(__ENV.CUSTOM_RPS || '20'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 5,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 20,
      startTime: '70s',  // starts after device_filter + cooldown
      tags: { complexity: 'time-range' },
    },
    // Tier 4: Stats Aggregation — materialized view read
    stats_agg: {
      executor: 'constant-arrival-rate',
      exec: 'statsAgg',          // runs the statsAgg() function
      rate: parseInt(__ENV.CUSTOM_RPS || '10'),
      timeUnit: '1s',
      duration: __ENV.CUSTOM_DURATION || '30s',
      preAllocatedVUs: 3,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 10,
      startTime: '105s',  // starts after time_range + cooldown
      tags: { complexity: 'stats-agg' },
    },
  },
  thresholds: {
    // Per-tier thresholds reflecting hardware path complexity
    'complexity_pk_lookup': ['p(95)<100', 'p(99)<200'],      // B-tree: fast
    'complexity_device_filter': ['p(95)<200', 'p(99)<400'],  // Composite index: moderate
    'complexity_time_range': ['p(95)<400', 'p(99)<600'],     // BRIN scan: slower
    'complexity_stats_agg': ['p(95)<500', 'p(99)<800'],      // MV: depends on freshness
    'http_req_failed': ['rate<0.01'],
  },
};

// ===== SETUP =====
export function setup() {
    console.log('='.repeat(60));
    console.log('Scenario 8: Query Complexity Tiers');
    console.log('='.repeat(60));
    console.log('Tier 1 (0-30s):    PK Lookup     - CPU + memory (B-tree)');
    console.log('Tier 2 (35-65s):   Device Filter - CPU + memory (composite)');
    console.log('Tier 3 (70-100s):  Time Range    - Disk I/O (BRIN scan)');
    console.log('Tier 4 (105-135s): Stats Agg     - MV read');
    console.log('='.repeat(60));

    // Detect dataset size
    const resp = getStats();
    let maxID = 100000000;
    if (resp.status === 200) {
        try {
            const body = resp.json();
            if (body.data && body.data.total_readings > 0) {
                maxID = body.data.total_readings;
            }
        } catch (e) { /* use fallback */ }
    }
    console.log(`Dataset: ~${Math.round(maxID / 1000000)}M rows (maxID=${maxID})`);
    console.log('='.repeat(60));
    return { maxID };
}

// ===== SCENARIO FUNCTIONS =====
// Each function is routed via k6's `exec` option — no scenario detection needed.

// Tier 1: PK Lookup — B-tree single-row scan → CPU + memory
export function pkLookup(data) {
    const id = Math.floor(Math.random() * data.maxID) + 1;
    const response = http.get(`${BASE_URL}/api/v1/sensor-readings?id=${id}`, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'PKLookup', tier: 'pk-lookup' },
    });

    pkLookupLatency.add(response.timings.duration);
    complexityErrors.add(response.status < 200 || response.status >= 400);

    check(response, {
        'pk_lookup: status is 2xx': (r) => r.status >= 200 && r.status < 400,
    });
}

// Tier 2: Device Filter — composite index scan → CPU + memory
export function deviceFilter(data) {
    const deviceId = String(Math.floor(Math.random() * 100) + 1).padStart(6, '0');
    const response = http.get(`${BASE_URL}/api/v1/sensor-readings?device_id=sensor-${deviceId}&limit=10`, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'DeviceFilter', tier: 'device-filter' },
    });

    deviceFilterLatency.add(response.timings.duration);
    complexityErrors.add(response.status < 200 || response.status >= 400);

    check(response, {
        'device_filter: status is 2xx': (r) => r.status >= 200 && r.status < 400,
    });
}

// Tier 3: Time Range — BRIN scan → Disk I/O
export function timeRange(data) {
    const deviceId = String(Math.floor(Math.random() * 100) + 1).padStart(6, '0');
    const from = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(); // 7 days ago
    const to = new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString();   // 1 day ago
    const response = http.get(
        `${BASE_URL}/api/v1/sensor-readings?device_id=sensor-${deviceId}&from=${from}&to=${to}&limit=100`,
        {
            headers: { 'Accept': 'application/json' },
            tags: { name: 'TimeRange', tier: 'time-range' },
        },
    );

    timeRangeLatency.add(response.timings.duration);
    complexityErrors.add(response.status < 200 || response.status >= 400);

    check(response, {
        'time_range: status is 2xx': (r) => r.status >= 200 && r.status < 400,
    });
}

// Tier 4: Stats Aggregation — materialized view read → MV freshness
export function statsAgg(data) {
    const response = http.get(`${BASE_URL}/api/v1/stats`, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'StatsAgg', tier: 'stats-agg' },
    });

    statsAggLatency.add(response.timings.duration);
    complexityErrors.add(response.status < 200 || response.status >= 400);

    check(response, {
        'stats_agg: status is 2xx': (r) => r.status >= 200 && r.status < 400,
    });
}

// ===== TEARDOWN =====
export function teardown() {
    console.log('='.repeat(60));
    console.log('Query Complexity Tiers Complete');
    console.log('='.repeat(60));
    console.log('Compare p95 latencies across tiers:');
    console.log('  PK Lookup < Device Filter < Time Range < Stats Agg');
    console.log('If PK Lookup is slow -> CPU/memory bottleneck');
    console.log('If Time Range is disproportionately slow -> Disk I/O bottleneck');
    console.log('='.repeat(60));
}
