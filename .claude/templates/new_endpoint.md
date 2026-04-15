# New Endpoint Checklist — Higth

## Definition

| Field | Value |
|-------|-------|
| HTTP Method | `GET` / `POST` / etc. |
| Route | `/api/v1/...` |
| Description | What this endpoint does |
| Auth required | No (portfolio project — no auth) |

## Query Parameters (if GET)

| Param | Type | Required | Validation | Default |
|-------|------|----------|------------|---------|
| `device_id` | string | yes (or `id`) | `^[a-zA-Z0-9_-]+$`, max 50 chars | — |

## Success Response

```json
{
  "data": { ... },
  "meta": { "count": N, ... }
}
```

HTTP status: `200 OK`
Headers: `X-Cache-Status`, `X-Response-Time`, `X-Request-ID`, `Content-Type: application/json`

## Error Responses

| Code | HTTP Status | Trigger |
|------|-------------|---------|
| `INVALID_PARAMETER` | 400 | Missing or malformed parameter |
| `DEVICE_NOT_FOUND` | 404 | No readings for device (no time filters) |
| `READING_NOT_FOUND` | 404 | No reading with given ID |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

## Implementation Layers

### 1. Handler (`internal/handler/sensor_handler.go` or new file)
- [ ] Function name: `Get{Resource}` or `Get{Resource}By{Filter}`
- [ ] Parse and validate all query parameters
- [ ] Call service method
- [ ] Record cache metrics (HIT/MISS)
- [ ] Return response with correct headers

### 2. Service (`internal/service/sensor_service.go` or new file)
- [ ] Method name: `Get{Resource}`
- [ ] Input validation (device ID regex, limit range, time format)
- [ ] Cache check if applicable
- [ ] Call repository method
- [ ] Cache population if applicable
- [ ] Return `(cacheStatus, data, error)`

### 3. Repository (`internal/repository/sensor_repo.go` or new file)
- [ ] Method name: `Query{Resource}` or `Get{Resource}By{Filter}`
- [ ] Parameterized SQL query (`$1`, `$2`, etc.)
- [ ] Context with 5s timeout
- [ ] Row scanning with proper error handling
- [ ] Return data and error (nil, nil for not-found)

### 4. Route registration (`cmd/api/main.go`)
- [ ] Add to `/api/v1/` route group
- [ ] Follow existing route pattern

## Cache Behavior
- Cached: Yes/No
- Cache key format: `sensor:{device_id}:...`
- TTL: 30s (default)
- Stats endpoints: always BYPASS (read MV directly)

## Example curl Commands

```bash
# Happy path
curl -s "http://localhost:8080/api/v1/{endpoint}?{param}={value}" | jq .

# Error case
curl -s "http://localhost:8080/api/v1/{endpoint}" | jq .
```

## Verification
- [ ] Happy path returns 200 with expected data
- [ ] Missing required param returns 400 with `INVALID_PARAMETER`
- [ ] Invalid param format returns 400 with details
- [ ] Not-found returns 404 with appropriate error code
- [ ] `X-Cache-Status` header present
- [ ] Second request returns `X-Cache-Status: HIT` (if cached)
