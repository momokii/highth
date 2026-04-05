# API Specification

This document defines the complete API contract for the High-Performance IoT Sensor Query System.

## Table of Contents

- [Overview](#overview)
- [Base URL](#base-url)
- [Authentication](#authentication)
- [Endpoints](#endpoints)
- [Request/Response Conventions](#requestresponse-conventions)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)

---

## Overview

This API provides a RESTful interface for querying IoT sensor telemetry data. All data access is mediated through the API — no direct database access is permitted.

### Core Query Pattern

The primary use case is retrieving the most recent N readings for a specific device:

```
GET /api/v1/sensor-readings?device_id={device_id}&limit={limit}
```

### API Design Principles

1. **RESTful** — Resource-based URLs with standard HTTP methods
2. **JSON** — Request and response bodies use JSON format
3. **Versioned** — URL path includes API version (v1)
4. **Stateless** — No server-side session state
5. **Idempotent** — Safe methods (GET, HEAD, OPTIONS) are idempotent

---

## Base URL

### Development

```
http://localhost:8080/api/v1
```

### Production

```
https://api.example.com/api/v1
```

---

## Authentication

> **Note:** For this portfolio project, authentication is not implemented. In production, you would typically use:
>
> - JWT tokens passed via `Authorization: Bearer {token}` header
> - API keys passed via `X-API-Key: {key}` header
> - OAuth 2.0 for third-party access

---

## Endpoints

### Get Sensor Readings (Unified Endpoint)

Unified endpoint supporting two modes via query parameters for maximum flexibility and consistent metrics.

#### Endpoint

```
GET /api/v1/sensor-readings
```

#### Request Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `id` | integer | Conditional* | - | Primary key ID for single-row lookup |
| `device_id` | string | Conditional* | - | Device identifier for device-filtered query |
| `limit` | integer | No | 10 | Number of records to return (1-500, device_id mode only) |
| `reading_type` | string | No | - | Filter by reading type (device_id mode only) |
| `from` | string | No | - | ISO 8601 timestamp for start of time range (device_id mode only) |
| `to` | string | No | - | ISO 8601 timestamp for end of time range (device_id mode only) |

*Exactly one of `id` or `device_id` must be provided. They are mutually exclusive.

#### Mode Selection

The endpoint operates in one of two modes based on the query parameters:

**PK Lookup Mode** (when `id` is provided):
- Retrieves a single sensor reading by its primary key ID
- Fast B-tree index scan on the `id` column
- Single-row cache entries with 30s TTL

**Device Query Mode** (when `device_id` is provided):
- Retrieves multiple sensor readings for a specific device
- Uses covering index on `(device_id, timestamp DESC)`
- Supports time-range filtering and reading type filtering
- Multi-row result sets with pagination via `limit`

#### Mutual Exclusivity

**Validation Rules:**
- Both `id` and `device_id` provided → `400 INVALID_PARAMETER`: "id and device_id are mutually exclusive"
- Neither `id` nor `device_id` provided → `400 INVALID_PARAMETER`: "id or device_id is required"

---

### Mode 1: PK Lookup (Single Reading)

Retrieve a single sensor reading by its unique primary key identifier.

#### Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | integer | Yes | Primary key ID of the sensor reading (must be positive) |

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sensor-readings?id=12345678"
```

#### Success Response (200 OK)

Returns a single sensor reading.

```json
{
  "data": {
    "id": "12345678",
    "device_id": "sensor-000001",
    "timestamp": "2026-03-19T08:19:17Z",
    "reading_type": "temperature",
    "value": 34.63,
    "unit": "°C"
  },
  "meta": {
    "id": "12345678"
  }
}
```

**Response Headers:**
- `X-Cache-Status`: `HIT` (Redis cache hit) or `MISS` (database query)
- `X-Response-Time`: Request processing time in milliseconds
- `X-Request-ID`: Unique request identifier
- `Cache-Control`: `public, max-age=30`

**Cache Behavior:**
- Cache key format: `sensor:id:{id}`
- Single-row cache entries, efficient for repeated lookups
- 30-second TTL

#### Error Responses (PK Mode)

##### 400 Bad Request - Invalid ID

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "id must be a positive integer",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "req_abc123",
    "details": {
      "parameter": "id",
      "provided": "abc",
      "constraints": { "type": "integer", "min": 1 }
    }
  }
}
```

##### 400 Bad Request - Mutual Exclusivity Violation

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "id and device_id are mutually exclusive",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "req_abc123",
    "details": {
      "parameter": "id,device_id",
      "provided": {"id": "123", "device_id": "sensor-001"},
      "constraints": {"rule": "provide exactly one"}
    }
  }
}
```

##### 404 Not Found

```json
{
  "error": {
    "code": "READING_NOT_FOUND",
    "message": "reading not found: no sensor reading exists with id 99999999",
    "timestamp": "2026-03-19T09:02:28Z",
    "request_id": "req_abc123"
  }
}
```

---

### Mode 2: Device Query (Multiple Readings)

Retrieve the most recent N sensor readings for a specific device.

#### Request Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `device_id` | string | Yes | - | Device identifier to fetch readings for |
| `limit` | integer | No | 10 | Number of records to return (1-500) |
| `reading_type` | string | No | - | Filter by reading type (e.g., "temperature", "humidity") |
| `from` | string | No | - | ISO 8601 timestamp for start of time range |
| `to` | string | No | - | ISO 8601 timestamp for end of time range |

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sensor-readings?device_id=sensor-001&limit=10"
```

#### Success Response (200 OK)

Returns an array of sensor readings in reverse chronological order (newest first).

```json
{
  "data": [
    {
      "id": "12345678",
      "device_id": "sensor-001",
      "timestamp": "2025-01-15T10:30:00Z",
      "reading_type": "temperature",
      "value": 23.45,
      "unit": "celsius"
    },
    {
      "id": "12345677",
      "device_id": "sensor-001",
      "timestamp": "2025-01-15T10:29:00Z",
      "reading_type": "temperature",
      "value": 23.42,
      "unit": "celsius"
    }
  ],
  "meta": {
    "count": 2,
    "limit": 10,
    "device_id": "sensor-001",
    "reading_type": null
  }
}
```

#### Response Fields

##### data array

Each object in the `data` array represents a single sensor reading:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier for the reading |
| `device_id` | string | Device identifier |
| `timestamp` | string | ISO 8601 timestamp in UTC |
| `reading_type` | string | Type of sensor reading |
| `value` | number | Sensor value |
| `unit` | string | Unit of measurement |

##### meta object

| Field | Type | Description |
|-------|------|-------------|
| `count` | integer | Number of readings returned |
| `limit` | integer | Maximum number requested |
| `device_id` | string | Device identifier from request |
| `reading_type` | string (nullable) | Reading type filter from request |

#### Error Responses (Device Mode)

##### 400 Bad Request

Invalid request parameters.

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "limit must be between 1 and 500",
    "timestamp": "2025-01-15T10:30:00Z",
    "details": {
      "parameter": "limit",
      "provided": 0,
      "constraints": {
        "min": 1,
        "max": 500
      }
    }
  }
}
```

##### 404 Not Found

Device has no readings in the database.

```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "No readings found for device_id: sensor-001",
    "timestamp": "2025-01-15T10:30:00Z"
  }
}
```

#### Common Error Responses

##### 500 Internal Server Error

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred",
    "timestamp": "2025-01-15T10:30:00Z",
    "request_id": "req_abc123"
  }
}
```

---

### Get Statistics

Retrieve aggregate statistics from materialized views.

#### Endpoint

```
GET /api/v1/stats
```

#### Query Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `device_id` | string | No | - | Filter statistics for specific device |
| `period` | string | No | hour | Aggregation period (hour, day, all) |

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/stats?period=hour"
```

#### Success Response (200 OK)

```json
{
  "data": {
    "device_hourly": [
      {
        "device_id": "sensor-001",
        "reading_type": "temperature",
        "hour": "2025-01-15T10:00:00Z",
        "avg_value": 23.5,
        "min_value": 20.1,
        "max_value": 26.8,
        "count": 60
      }
    ],
    "device_daily": [],
    "global": {
      "total_readings": 50000000,
      "total_devices": 1000,
      "avg_reading_value": 45.2
    }
  },
  "meta": {
    "period": "hour",
    "generated_at": "2025-01-15T10:30:00Z"
  }
}
```

#### Error Responses

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INVALID_PERIOD` | 400 | Period must be one of: hour, day, all |
| `STATS_UNAVAILABLE` | 503 | Materialized views not refreshed |

---

### Health Check

Check API health status (useful for load balancers and monitoring).

#### Endpoints

```
GET /health           # Full health check with component status
GET /health/ready     # Readiness probe (Kubernetes)
GET /health/live      # Liveness probe (Kubernetes)
```

#### Example Request

```bash
curl -X GET "http://localhost:8080/health"
curl -X GET "http://localhost:8080/health/ready"
curl -X GET "http://localhost:8080/health/live"
```

#### Success Response (200 OK)

```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5
    },
    "cache": {
      "status": "healthy",
      "latency_ms": 1
    }
  }
}
```

#### Readiness Probe Response (200 OK)

```json
{
  "status": "ready"
}
```

#### Liveness Probe Response (200 OK)

```json
{
  "status": "alive"
}
```

#### Degraded Response (503 Service Unavailable)

```json
{
  "status": "degraded",
  "timestamp": "2025-01-15T10:30:00Z",
  "checks": {
    "database": {
      "status": "unhealthy",
      "error": "connection timeout"
    },
    "cache": {
      "status": "healthy",
      "latency_ms": 1
    }
  }
}
```

#### Probe Behavior

| Probe | Behavior | Use Case |
|-------|----------|----------|
| `/health` | Full check with details | Load balancer health checks, monitoring dashboards |
| `/health/ready` | Checks if service can accept traffic | Kubernetes readiness probes, startup validation |
| `/health/live` | Simple alive check | Kubernetes liveness probes, restart detection |

---

## Request/Response Conventions

### HTTP Status Codes

| Status Code | Usage |
|-------------|-------|
| 200 OK | Successful request |
| 400 Bad Request | Invalid request parameters |
| 404 Not Found | Resource not found |
| 500 Internal Server Error | Unexpected server error |
| 503 Service Unavailable | Service unavailable or unhealthy |

### Request Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | Must be `application/json` for request bodies |
| `Accept` | Should be `application/json` for JSON responses |
| `User-Agent` | Optional client identifier |

### Response Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | Always `application/json` |
| `X-Request-ID` | Unique request identifier for debugging |
| `X-Response-Time` | Server processing time in milliseconds |
| `Cache-Control` | Cache directives (e.g., `max-age=30`) |

### Date/Time Format

All timestamps use ISO 8601 format in UTC:

```
2025-01-15T10:30:00Z
```

### Number Format

- Numeric values use JSON number type
- Floating point values may have up to 6 decimal places
- Large integers are represented as strings to preserve precision

---

## Error Handling

### Error Response Structure

All error responses follow this structure:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "timestamp": "2025-01-15T10:30:00Z",
    "details": {}
  }
}
```

### Error Codes

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INVALID_PARAMETER` | 400 | Request parameter validation failed |
| `DEVICE_NOT_FOUND` | 404 | No readings found for device |
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `DATABASE_UNAVAILABLE` | 503 | Database connection failed |
| `CACHE_UNAVAILABLE` | 503 | Cache connection failed |

### Validation Rules

#### device_id

- **Type:** String
- **Required:** Yes
- **Format:** Alphanumeric with hyphens and underscores
- **Length:** 1-50 characters
- **Pattern:** `^[a-zA-Z0-9_-]+$`

#### limit

- **Type:** Integer
- **Required:** No
- **Default:** 10
- **Range:** 1-500
- **Example:** `limit=50`

#### reading_type

- **Type:** String
- **Required:** No
- **Format:** Alphanumeric
- **Length:** 1-30 characters
- **Common values:** `temperature`, `humidity`, `pressure`, `voltage`

---

## Rate Limiting

> **Note:** Rate limiting is implemented at the Nginx reverse proxy layer.

### Rate Limits

| Client | Rate Limit |
|--------|------------|
| Unauthenticated | 10 requests/second |
| Authenticated (future) | 100 requests/second |

### Rate Limit Headers

Rate limit information is returned in response headers:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1705300200
```

### Rate Limit Exceeded

When rate limit is exceeded, returns 429 Too Many Requests:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Try again in 1 second.",
    "timestamp": "2025-01-15T10:30:00Z",
    "details": {
      "retry_after": 1
    }
  }
}
```

---

## Pagination

> **Note:** For the primary query pattern (get last N readings), offset-based pagination is not used because new readings are constantly being added. Instead, use timestamp-based pagination for traversing historical data.

### Timestamp-Based Pagination (Future Enhancement)

For fetching readings older than a certain timestamp:

```
GET /api/v1/sensor-readings?device_id=sensor-001&limit=10&before=2025-01-15T10:00:00Z
```

This would return the 10 readings immediately before the specified timestamp.

---

## Caching Behavior

### Cache-Control Headers

The API includes cache headers to inform client caching:

```
Cache-Control: public, max-age=30
```

- `public` — Response may be cached by any cache
- `max-age=30` — Cache for 30 seconds

### ETag Support (Future Enhancement)

For conditional requests, the API may return an ETag:

```
ETag: "33a64df551425fcc55e4d42a148795d9f25f89d4"
```

Clients can use `If-None-Match` for conditional requests:

```
If-None-Match: "33a64df551425fcc55e4d42a148795d9f25f89d4"
```

Returns `304 Not Modified` if data hasn't changed.

---

## OpenAPI/Swagger Specification

For integration with API documentation tools, here's the OpenAPI 3.0 specification:

```yaml
openapi: 3.0.0
info:
  title: IoT Sensor Query API
  version: 1.0.0
  description: High-performance API for querying IoT sensor telemetry

servers:
  - url: http://localhost:8080/api/v1
    description: Development server

paths:
  /sensor-readings:
    get:
      summary: Get sensor readings
      operationId: getSensorReadings
      parameters:
        - name: device_id
          in: query
          required: true
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 500
            default: 10
        - name: reading_type
          in: query
          schema:
            type: string
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/SensorReading'
                  meta:
                    $ref: '#/components/schemas/ResponseMetadata'
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '404':
          description: Device not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /health:
    get:
      summary: Health check
      operationId: healthCheck
      responses:
        '200':
          description: Healthy
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HealthStatus'

components:
  schemas:
    SensorReading:
      type: object
      properties:
        id:
          type: string
        device_id:
          type: string
        timestamp:
          type: string
          format: date-time
        reading_type:
          type: string
        value:
          type: number
        unit:
          type: string

    ResponseMetadata:
      type: object
      properties:
        count:
          type: integer
        limit:
          type: integer
        device_id:
          type: string
        reading_type:
          type: string
          nullable: true

    Error:
      type: object
      properties:
        error:
          type: object
          properties:
            code:
              type: string
            message:
              type: string
            timestamp:
              type: string
              format: date-time

    HealthStatus:
      type: object
      properties:
        status:
          type: string
          enum: [healthy, degraded]
        timestamp:
          type: string
          format: date-time
        checks:
          type: object
```

---

## Related Documentation

- [architecture.md](architecture.md) — System architecture and caching strategy
- [stack.md](stack.md) — Technology stack and API framework details
- [testing.md](testing.md) — Load testing methodology for API validation
