// API Endpoint Functions
//
// Provides wrapper functions for all Higth API endpoints
// Handles request construction, parameter building, and response validation

import http from 'k6/http';
import { check } from 'k6';
import { Config } from './config.js';

/**
 * Health Check Endpoint
 * GET /health
 * Returns: { status: 'ok', timestamp: '...' }
 */
export function getHealth() {
  const url = `${Config.BASE_URL}/health`;
  const params = {
    headers: { 'Accept': 'application/json' },
    tags: { name: 'HealthCheck' },
  };

  const response = http.get(url, params);

  check(response, {
    'health status is 200': (r) => r.status === 200,
    'health returns ok': (r) => r.json('status') === 'ok',
  });

  return response;
}

/**
 * Sensor Readings Endpoint
 * GET /api/v1/sensor-readings?device_id=X&reading_type=Y&limit=Z
 * Query params:
 *   - device_id: Device identifier (required)
 *   - reading_type: Filter by reading type (temperature, humidity, pressure)
 *   - from: Start time (ISO 8601)
 *   - to: End time (ISO 8601)
 *   - limit: Max records to return
 * Returns: { data: [...], meta: { count, limit, device_id } }
 */
export function getSensorReadings(deviceId, options = {}) {
  const {
    reading_type = null,
    from = null,
    to = null,
    limit = 100,
  } = options;

  // Build query string
  const queryParams = [];
  queryParams.push(`device_id=${deviceId}`);
  if (reading_type) queryParams.push(`reading_type=${reading_type}`);
  if (from) queryParams.push(`from=${encodeURIComponent(from)}`);
  if (to) queryParams.push(`to=${encodeURIComponent(to)}`);
  queryParams.push(`limit=${limit}`);

  const queryString = queryParams.join('&');
  const url = `${Config.BASE_URL}/api/v1/sensor-readings?${queryString}`;

  const params = {
    headers: { 'Accept': 'application/json' },
    tags: {
      name: 'SensorReadings',
      device_id: deviceId,
      reading_type: reading_type || 'all',
    },
  };

  const response = http.get(url, params);

  check(response, {
    'sensor readings status is 200': (r) => r.status === 200,
    'sensor readings has data array': (r) => {
      try {
        return Array.isArray(r.json('data'));
      } catch {
        return false;
      }
    },
  });

  return response;
}

/**
 * Device Stats Endpoint
 * GET /api/v1/sensors/{deviceId}/stats
 * Returns: { device_id, reading_count, min_value, max_value, avg_value, latest_reading }
 */
export function getDeviceStats(deviceId) {
  const url = `${Config.BASE_URL}/api/v1/sensors/${deviceId}/stats`;

  const params = {
    headers: { 'Accept': 'application/json' },
    tags: {
      name: 'DeviceStats',
      device_id: deviceId,
    },
  };

  const response = http.get(url, params);

  check(response, {
    'device stats status is 200': (r) => r.status === 200,
    'device stats has device_id': (r) => {
      try {
        return r.json('device_id') === deviceId;
      } catch {
        return false;
      }
    },
  });

  return response;
}

/**
 * Global Stats Endpoint
 * GET /api/v1/stats
 * Returns: { total_devices, total_readings, reading_types, devices_online }
 */
export function getGlobalStats() {
  const url = `${Config.BASE_URL}/api/v1/stats`;

  const params = {
    headers: { 'Accept': 'application/json' },
    tags: { name: 'GlobalStats' },
  };

  const response = http.get(url, params);

  check(response, {
    'global stats status is 200': (r) => r.status === 200,
    'global stats has total_devices': (r) => {
      try {
        return typeof r.json('total_devices') === 'number';
      } catch {
        return false;
      }
    },
  });

  return response;
}

/**
 * Device Hourly Stats Endpoint (Materialized View)
 * GET /api/v1/sensors/{deviceId}/stats/hourly
 * Query params:
 *   - hours: number of hours to look back (default: 24)
 * Returns: Array of hourly aggregates
 */
export function getDeviceHourlyStats(deviceId, hours = 24) {
  const url = `${Config.BASE_URL}/api/v1/sensors/${deviceId}/stats/hourly?hours=${hours}`;

  const params = {
    headers: { 'Accept': 'application/json' },
    tags: {
      name: 'DeviceHourlyStats',
      device_id: deviceId,
    },
  };

  const response = http.get(url, params);

  check(response, {
    'hourly stats status is 200': (r) => r.status === 200,
    'hourly stats is array': (r) => {
      try {
        return Array.isArray(r.json());
      } catch {
        return false;
      }
    },
  });

  return response;
}

/**
 * Global Hourly Stats Endpoint (Materialized View)
 * GET /api/v1/stats/hourly
 * Query params:
 *   - hours: number of hours to look back (default: 24)
 * Returns: Array of global hourly aggregates
 */
export function getGlobalHourlyStats(hours = 24) {
  const url = `${Config.BASE_URL}/api/v1/stats/hourly?hours=${hours}`;

  const params = {
    headers: { 'Accept': 'application/json' },
    tags: { name: 'GlobalHourlyStats' },
  };

  const response = http.get(url, params);

  check(response, {
    'global hourly stats status is 200': (r) => r.status === 200,
    'global hourly stats is array': (r) => {
      try {
        return Array.isArray(r.json());
      } catch {
        return false;
      }
    },
  });

  return response;
}

/**
 * Extract cache hit/miss information from response headers
 * Returns: { hits: number, misses: number, hit_rate: number }
 */
export function extractCacheMetrics(response) {
  const cacheHits = parseInt(response.headers['X-Cache-Hits'] || '0', 10);
  const cacheMisses = parseInt(response.headers['X-Cache-Misses'] || '0', 10);
  const total = cacheHits + cacheMisses;
  const hitRate = total > 0 ? (cacheHits / total) * 100 : 0;

  return {
    hits: cacheHits,
    misses: cacheMisses,
    hit_rate: hitRate,
  };
}

/**
 * Extract database query time from response headers
 * Returns: query time in milliseconds
 */
export function extractDbQueryTime(response) {
  const queryTime = response.headers['X-Db-Query-Time'] || '0';
  return parseFloat(queryTime);
}
