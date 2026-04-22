// Scenario 7: Stress Ceiling Finder (Bundled)
//
// Ramps up request rate until the system breaks to find the actual
// hardware ceiling. Reports the maximum sustainable RPS before errors
// or latency degradation exceed acceptable thresholds.
//
// This answers: "At what RPS does the system fail?"
// The answer directly identifies which hardware component gives out first.

import http from 'k6/http';
import { check, fail } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';

// ===== CONFIGURATION =====
const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const START_RPS = parseInt(__ENV.STRESS_START_RPS || '50');
const RAMP_STEP = parseInt(__ENV.STRESS_RAMP_STEP || '50');
const STEP_DURATION = __ENV.STRESS_STEP_DURATION || '30s';
const MAX_RPS = parseInt(__ENV.STRESS_MAX_RPS || '2000');

// ===== CUSTOM METRICS =====
const stressLatency = new Trend('stress_latency');
const stressErrors = new Rate('stress_errors');
const stressRps = new Trend('stress_rps');

// ===== API ENDPOINT FUNCTIONS =====
function getSensorReadings(deviceId) {
    const url = `${BASE_URL}/api/v1/sensor-readings?device_id=${deviceId}&limit=10`;
    return http.get(url, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'StressReadings', endpoint: 'stress' },
    });
}

function getSensorReadingById(id) {
    const url = `${BASE_URL}/api/v1/sensor-readings?id=${id}`;
    return http.get(url, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'StressPKLookup', endpoint: 'stress-pk' },
    });
}

function getStats() {
    const url = `${BASE_URL}/api/v1/stats`;
    return http.get(url, {
        headers: { 'Accept': 'application/json' },
        tags: { name: 'Stats' },
    });
}

// ===== BUILD RAMPING STAGES =====
// Generates stages: 50 RPS for 30s, 100 RPS for 30s, 150 RPS for 30s, ...
function buildRampingStages() {
    const stages = [];
    for (let rps = START_RPS; rps <= MAX_RPS; rps += RAMP_STEP) {
        stages.push({ duration: STEP_DURATION, target: rps });
    }
    return stages;
}

// ===== TEST CONFIGURATION =====
export const options = {
  scenarios: {
    stress_ceiling: {
      executor: 'ramping-arrival-rate',
      startRate: START_RPS,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: parseInt(__ENV.CUSTOM_VUS) || 500,
      stages: buildRampingStages(),
    },
  },
  thresholds: {
    // Soft thresholds — the point is to FIND the ceiling, not pass at all costs
    // These will trigger warnings but won't abort the test early
    'http_req_duration': [
      'p(95)<2000',   // abort if p95 exceeds 2s (system is clearly broken)
    ],
    'http_req_failed': ['rate<0.10'],  // abort if >10% errors
  },
};

// ===== SETUP =====
export function setup() {
    console.log('='.repeat(60));
    console.log('Scenario 7: Stress Ceiling Finder');
    console.log('='.repeat(60));
    console.log(`Ramp: ${START_RPS} → ${MAX_RPS} RPS (+${RAMP_STEP} per step)`);
    console.log(`Step duration: ${STEP_DURATION}`);
    console.log(`Max VUs: ${parseInt(__ENV.CUSTOM_VUS) || 500}`);

    // Detect dataset size for PK lookups
    const statsResp = getStats();
    let maxID = 100000000;
    if (statsResp.status === 200) {
        try {
            const body = statsResp.json();
            if (body.data && body.data.total_readings > 0) {
                maxID = body.data.total_readings;
                console.log(`Dataset: ${maxID} rows`);
            }
        } catch (e) {
            console.log(`Warning: Could not detect dataset size, using fallback maxID=${maxID}`);
        }
    }

    console.log('='.repeat(60));
    return { maxID };
}

// ===== MAIN TEST =====
export default function (data) {
    // Mix of query types: 40% PK lookup, 50% device query, 10% stats
    const rand = Math.random();
    let response;

    if (rand < 0.4) {
        // PK lookup
        const id = Math.floor(Math.random() * data.maxID) + 1;
        response = getSensorReadingById(id);
    } else if (rand < 0.9) {
        // Device query
        const deviceId = String(Math.floor(Math.random() * 1000) + 1).padStart(6, '0');
        response = getSensorReadings(`sensor-${deviceId}`);
    } else {
        // Stats
        response = getStats();
    }

    stressLatency.add(response.timings.duration);
    stressRps.add(1);

    const isSuccess = response.status >= 200 && response.status < 400;
    stressErrors.add(!isSuccess);

    check(response, {
        'status is 2xx': (r) => r.status >= 200 && r.status < 400,
    });
}

// ===== TEARDOWN =====
export function teardown() {
    console.log('='.repeat(60));
    console.log('Stress Ceiling Test Complete');
    console.log('Check the output above for the RPS level where errors/latency spiked.');
    console.log('That is your hardware ceiling.');
    console.log('='.repeat(60));
}
