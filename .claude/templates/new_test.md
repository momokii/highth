# New Test Checklist — Higth

## Test Type Selection

| Type | When to Use | Framework |
|------|-------------|-----------|
| Go unit test | Business logic, validation, error handling | `testing` package, table-driven |
| Go integration test | Database queries, cache behavior | `testing` + real postgres/redis |
| k6 benchmark | Load testing, latency validation, cache performance | k6 (Docker), JavaScript |

---

## Go Unit Tests

### File Location
- `internal/handler/sensor_handler_test.go`
- `internal/service/sensor_service_test.go`
- `internal/repository/sensor_repo_test.go`
- Same package, `_test.go` suffix

### Pattern: Table-Driven Tests

```go
func TestGetSensorReadings(t *testing.T) {
    tests := []struct {
        name        string
        deviceID    string
        limit       int
        wantErr     error
        wantStatus  int
    }{
        {"valid request", "sensor-001", 10, nil, 200},
        {"invalid device ID", "sensor 001", 10, service.ErrInvalidParameter, 400},
        {"limit too high", "sensor-001", 501, service.ErrInvalidParameter, 400},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Checklist
- [ ] Test file follows naming: `{source}_test.go`
- [ ] Table-driven pattern with `t.Run()` for each case
- [ ] Success case covered
- [ ] Validation failure cases covered
- [ ] Error wrapping verified with `errors.Is()`
- [ ] No real secrets or credentials in test fixtures
- [ ] Mock interfaces for external dependencies (repository, cache)
- [ ] Run: `go test ./internal/... -v`

---

## k6 Benchmark Scenarios

### File Location
- `tests/scenarios/NN-descriptive-name.js` (NN = next number, e.g., `07`)

### Pattern
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

export const options = {
    scenarios: {
        constant_load: {
            executor: 'constant-arrival-rate',
            rate: __ENV.CUSTOM_RPS || 50,
            timeUnit: '1s',
            duration: __ENV.CUSTOM_DURATION || '30s',
            preAllocatedVUs: 10,
            maxVUs: parseInt(__ENV.CUSTOM_VUS) || 100,
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.01'],
    },
};
```

### Checklist
- [ ] Uses `CUSTOM_RPS`, `CUSTOM_DURATION`, `CUSTOM_VUS` env vars (set by runner)
- [ ] SLO thresholds defined (p95 < 500ms, error rate < 1%)
- [ ] Registered in `tests/run-benchmarks.sh` SCENARIOS array
- [ ] Tier RPS and duration entries added to `TIER_RPS` and `TIER_DURATION` in runner
- [ ] Run: `./tests/run-benchmarks.sh --tier smoke -s {name}`

---

## Verification
- [ ] Test passes reliably (run 3+ times for k6)
- [ ] Test is isolated — no dependency on shared state
- [ ] `go test ./internal/... -v` passes all Go tests
- [ ] `./tests/run-benchmarks.sh --tier smoke` passes all k6 scenarios
