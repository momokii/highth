// Package service handles business logic for sensor readings.
package service

import (
	"context"
	"time"

	"github.com/kelanach/higth/internal/model"
)

// SensorServicer defines the interface for sensor service operations.
// SensorService implements this interface.
type SensorServicer interface {
	GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) (string, []model.SensorReading, error)
	GetSensorReadingByID(ctx context.Context, id int64) (string, *model.SensorReading, error)
	GetStats(ctx context.Context) (map[string]interface{}, error)
	Ping(ctx context.Context) map[string]error
	PingWithLatency(ctx context.Context) map[string]PingResult
}
