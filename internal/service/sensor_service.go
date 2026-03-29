// Package service handles business logic for sensor readings.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/kelanach/higth/internal/cache"
	"github.com/kelanach/higth/internal/model"
	"github.com/kelanach/higth/internal/repository"
)

var (
	// ErrInvalidParameter is returned when a parameter is invalid.
	ErrInvalidParameter = errors.New("invalid parameter")
	// ErrDeviceNotFound is returned when a device has no readings.
	ErrDeviceNotFound = errors.New("device not found")
)

// SensorService handles business logic for sensor readings.
type SensorService struct {
	repo  *repository.SensorRepository
	cache *cache.RedisCache
}

// Config holds service configuration.
type Config struct {
	CacheEnabled bool
}

// New creates a new SensorService.
func New(repo *repository.SensorRepository, cache *cache.RedisCache, cfg Config) *SensorService {
	return &SensorService{
		repo:  repo,
		cache: cache,
	}
}

// GetSensorReadings retrieves the most recent N sensor readings for a device.
//
// It uses the cache-aside pattern:
// 1. Check cache first
// 2. If cache miss, query database
// 3. Populate cache for next request
//
// Results are cached for 30 seconds by default.
// Returns (cacheStatus, readings, error) where cacheStatus is "HIT", "MISS", or "".
func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) (string, []model.SensorReading, error) {
	// Validate input
	if !s.isValidDeviceID(deviceID) {
		return "", nil, fmt.Errorf("%w: invalid device_id", ErrInvalidParameter)
	}

	if limit < 1 || limit > 500 {
		return "", nil, fmt.Errorf("%w: limit must be between 1 and 500", ErrInvalidParameter)
	}

	if readingType != "" && !s.isValidReadingType(readingType) {
		return "", nil, fmt.Errorf("%w: invalid reading_type", ErrInvalidParameter)
	}

	// Check cache first if enabled
	if s.cache != nil && s.cache.IsEnabled() {
		key := s.cacheKey(deviceID, limit, readingType, from, to)
		var cached []model.SensorReading
		if err := s.cache.Get(ctx, key, &cached); err == nil {
			return "HIT", cached, nil
		}
	}

	// Cache miss - query database
	readings, err := s.repo.Query(ctx, deviceID, limit, readingType, from, to)
	if err != nil {
		return "", nil, fmt.Errorf("failed to query sensor readings: %w", err)
	}

	// Empty result handling
	// If no time filters specified, empty result means device has no data at all -> 404
	// If time filters are specified, empty result means no data in time window -> 200 with empty array
	if len(readings) == 0 {
		if from == nil && to == nil {
			return "", nil, fmt.Errorf("%w: no readings found for device_id: %s", ErrDeviceNotFound, deviceID)
		}
		// Time-filtered query with no results - return empty array instead of 404
		return "MISS", []model.SensorReading{}, nil
	}

	// Populate cache (fire and forget - don't fail if cache is down)
	if s.cache != nil && s.cache.IsEnabled() {
		key := s.cacheKey(deviceID, limit, readingType, from, to)
		_ = s.cache.Set(ctx, key, readings)
	}

	return "MISS", readings, nil
}

// isValidDeviceID checks if the device ID is valid.
// Valid device IDs contain only alphanumeric characters, hyphens, and underscores.
// Length must be between 1 and 50 characters.
// Empty string is valid (returns readings from all devices).
func (s *SensorService) isValidDeviceID(deviceID string) bool {
	if deviceID == "" {
		return true // Empty device_id means all devices
	}
	if len(deviceID) > 50 {
		return false
	}
	// Check for valid characters (alphanumeric, hyphen, underscore)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, deviceID)
	return matched
}

// isValidReadingType checks if the reading type is valid.
// Valid reading types are alphanumeric strings 1-30 characters.
func (s *SensorService) isValidReadingType(readingType string) bool {
	if len(readingType) == 0 || len(readingType) > 30 {
		return false
	}
	// Check if alphanumeric only
	for _, r := range readingType {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// cacheKey generates a consistent cache key for sensor readings.
// Format: sensor:{device_id}:readings:{limit}[:{reading_type}]:{from_unix}:{to_unix}
// Time ranges are included as Unix timestamps if specified.
func (s *SensorService) cacheKey(deviceID string, limit int, readingType string, from, to *time.Time) string {
	var fromStr, toStr string
	if from != nil {
		fromStr = fmt.Sprintf(":%d", from.Unix())
	}
	if to != nil {
		toStr = fmt.Sprintf(":%d", to.Unix())
	}

	if readingType != "" {
		return fmt.Sprintf("sensor:%s:readings:%d:%s%s%s", deviceID, limit, readingType, fromStr, toStr)
	}
	return fmt.Sprintf("sensor:%s:readings:%d%s%s", deviceID, limit, fromStr, toStr)
}

// GetStats returns database statistics from the materialized view.
// This is much faster than COUNT queries on the base table for large datasets.
func (s *SensorService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStatsFromMV(ctx)
}

// Ping checks if the service dependencies are healthy.
func (s *SensorService) Ping(ctx context.Context) map[string]error {
	results := make(map[string]error)

	// Check database
	if err := s.repo.Ping(ctx); err != nil {
		results["database"] = err
	} else {
		results["database"] = nil
	}

	// Check cache
	if s.cache != nil && s.cache.IsEnabled() {
		if err := s.cache.Ping(ctx); err != nil {
			results["cache"] = err
		} else {
			results["cache"] = nil
		}
	} else {
		results["cache"] = nil // Cache disabled, not an error
	}

	return results
}

// PingResult represents the result of a health check with latency.
type PingResult struct {
	Error     error
	LatencyMs int64
}

// PingWithLatency checks dependencies and returns results with timing.
func (s *SensorService) PingWithLatency(ctx context.Context) map[string]PingResult {
	results := make(map[string]PingResult)

	// Check database with timing
	start := time.Now()
	dbErr := s.repo.Ping(ctx)
	results["database"] = PingResult{
		Error:     dbErr,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	// Check cache with timing
	start = time.Now()
	var cacheErr error
	if s.cache != nil && s.cache.IsEnabled() {
		cacheErr = s.cache.Ping(ctx)
	}
	results["cache"] = PingResult{
		Error:     cacheErr,
		LatencyMs: time.Since(start).Milliseconds(),
	}

	return results
}
