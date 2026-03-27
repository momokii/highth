// Helper Functions
//
// Utility functions for test scenarios
// Includes device selection, time range generation, and Zipf distribution

import { Config } from './config.js';

/**
 * Select a random device from the full device list
 */
export function randomDevice() {
  const index = Math.floor(Math.random() * Config.ALL_DEVICES.length);
  return Config.ALL_DEVICES[index];
}

/**
 * Select a random hot device (top 20%)
 */
export function randomHotDevice() {
  const index = Math.floor(Math.random() * Config.HOT_DEVICES.length);
  return Config.HOT_DEVICES[index];
}

/**
 * Select a random cold device (bottom 80%)
 */
export function randomColdDevice() {
  const index = Math.floor(Math.random() * Config.COLD_DEVICES.length);
  return Config.COLD_DEVICES[index];
}

/**
 * Select device using Zipf distribution
 * 80% of requests go to hot devices (top 20%), 20% to cold devices
 * This simulates real IoT traffic patterns where some devices are queried more frequently
 */
export function zipfDevice() {
  const hotTraffic = Math.random() < Config.HOT_DEVICE_TRAFFIC;

  if (hotTraffic) {
    // Select from hot devices (top 20%)
    return randomHotDevice();
  } else {
    // Select from cold devices (bottom 80%)
    return randomColdDevice();
  }
}

/**
 * Select a random reading type
 */
export function randomReadingType() {
  const index = Math.floor(Math.random() * Config.READING_TYPES.length);
  return Config.READING_TYPES[index];
}

/**
 * Select a random limit from configured limits
 */
export function randomLimit() {
  const index = Math.floor(Math.random() * Config.LIMITS.length);
  return Config.LIMITS[index];
}

/**
 * Select a random time range
 */
export function randomTimeRange() {
  const index = Math.floor(Math.random() * Config.TIME_RANGES.length);
  return Config.TIME_RANGES[index];
}

/**
 * Generate ISO 8601 timestamp for 'now'
 */
export function now() {
  return new Date().toISOString();
}

/**
 * Generate ISO 8601 timestamp for a duration ago
 * @param {string} duration - Duration string like '1h', '24h', '7d'
 */
export function timeAgo(duration) {
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
    default:
      throw new Error(`Invalid duration unit: ${unit}`);
  }

  return now.toISOString();
}

/**
 * Select a workload type based on configured mix
 * Returns: 'health_check', 'stats', or 'sensor_readings'
 */
export function selectWorkloadType() {
  const rand = Math.random();
  const cumulative =
    Config.WORKLOAD_MIX.HEALTH_CHECK +
    Config.WORKLOAD_MIX.STATS +
    Config.WORKLOAD_MIX.SENSOR_READINGS;

  // Normalize if values don't sum to 1
  const healthCheckRatio = Config.WORKLOAD_MIX.HEALTH_CHECK / cumulative;
  const statsRatio = Config.WORKLOAD_MIX.STATS / cumulative;

  if (rand < healthCheckRatio) {
    return 'health_check';
  } else if (rand < healthCheckRatio + statsRatio) {
    return 'stats';
  } else {
    return 'sensor_readings';
  }
}

/**
 * Sleep for a specified duration (k6 doesn't have native sleep)
 * Uses busy-wait for very short pauses
 * @param {number} seconds - Sleep duration in seconds
 */
export function sleep(seconds) {
  // k6 will handle this natively in the scenario executor
  // This is just a placeholder
}

/**
 * Generate a unique ID for custom metrics tracking
 */
export function generateId() {
  return `${__VU}-${__ITER}`;
}

/**
 * Calculate percentage change between two values
 */
export function percentChange(oldValue, newValue) {
  if (oldValue === 0) return newValue > 0 ? 100 : 0;
  return ((newValue - oldValue) / oldValue) * 100;
}

/**
 * Format bytes to human-readable string
 */
export function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

/**
 * Format duration to human-readable string
 */
export function formatDuration(ms) {
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = ((ms % 60000) / 1000).toFixed(0);
  return `${minutes}m ${seconds}s`;
}

/**
 * Truncate a device ID for cleaner logging
 */
export function shortDeviceId(deviceId) {
  if (deviceId.length <= 12) return deviceId;
  return `${deviceId.substring(0, 9)}...${deviceId.substring(deviceId.length - 3)}`;
}

/**
 * Validate response meets minimum criteria
 * Returns: { valid: boolean, reason: string }
 */
export function validateResponse(response, options = {}) {
  const {
    minStatus = 200,
    maxStatus = 299,
    requireData = false,
    requireJson = true,
  } = options;

  // Check status code
  if (response.status < minStatus || response.status > maxStatus) {
    return {
      valid: false,
      reason: `Status ${response.status} not in range [${minStatus}, ${maxStatus}]`,
    };
  }

  // Check JSON if required
  if (requireJson) {
    try {
      response.json();
    } catch (e) {
      return {
        valid: false,
        reason: 'Response is not valid JSON',
      };
    }
  }

  // Check data array if required
  if (requireData) {
    try {
      if (!Array.isArray(response.json('data'))) {
        return {
          valid: false,
          reason: 'Response does not contain data array',
        };
      }
    } catch (e) {
      return {
        valid: false,
        reason: 'Could not validate data array',
      };
    }
  }

  return { valid: true, reason: 'OK' };
}

/**
 * Custom rate limiter for controlling request pace
 * Unlike k6's built-in rate limiting, this can be used within scenarios
 */
export class RateLimiter {
  constructor(requestsPerSecond) {
    this.requestsPerSecond = requestsPerSecond;
    this.minInterval = 1000 / requestsPerSecond;
    this.lastRequest = 0;
  }

  wait() {
    const now = Date.now();
    const elapsed = now - this.lastRequest;

    if (elapsed < this.minInterval) {
      const sleepTime = this.minInterval - elapsed;
      // k6's sleep function
      sleep(sleepTime / 1000);
    }

    this.lastRequest = Date.now();
  }
}
