// Package service handles business logic for sensor readings.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
func (s *SensorService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
	// Validate input
	if !s.isValidDeviceID(deviceID) {
		return nil, fmt.Errorf("%w: invalid device_id", ErrInvalidParameter)
	}

	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 500", ErrInvalidParameter)
	}

	if readingType != "" && !s.isValidReadingType(readingType) {
		return nil, fmt.Errorf("%w: invalid reading_type", ErrInvalidParameter)
	}

	// Check cache first if enabled
	if s.cache != nil && s.cache.IsEnabled() {
		key := s.cacheKey(deviceID, limit, readingType)
		var cached []model.SensorReading
		if err := s.cache.Get(ctx, key, &cached); err == nil {
			return cached, nil
		}
	}

	// Cache miss - query database
	readings, err := s.repo.Query(ctx, deviceID, limit, readingType)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor readings: %w", err)
	}

	// Empty result is valid - return empty list instead of error

	// Populate cache (fire and forget - don't fail if cache is down)
	if s.cache != nil && s.cache.IsEnabled() {
		key := s.cacheKey(deviceID, limit, readingType)
		_ = s.cache.Set(ctx, key, readings)
	}

	return readings, nil
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
func (s *SensorService) isValidReadingType(readingType string) bool {
	validTypes := []string{"temperature", "humidity", "pressure"}
	for _, t := range validTypes {
		if strings.EqualFold(readingType, t) {
			return true
		}
	}
	return false
}

// cacheKey generates a consistent cache key for sensor readings.
// Format: sensor:{device_id}:readings:{limit}[:{reading_type}]
func (s *SensorService) cacheKey(deviceID string, limit int, readingType string) string {
	if readingType != "" {
		return fmt.Sprintf("sensor:%s:readings:%d:%s", deviceID, limit, readingType)
	}
	return fmt.Sprintf("sensor:%s:readings:%d", deviceID, limit)
}

// GetStats returns database statistics.
func (s *SensorService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	rowCount, err := s.repo.GetRowCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get row count: %w", err)
	}

	deviceCount, err := s.repo.GetDeviceCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device count: %w", err)
	}

	return map[string]interface{}{
		"total_readings": rowCount,
		"total_devices":  deviceCount,
		"queried_at":     time.Now().Format(time.RFC3339),
	}, nil
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
