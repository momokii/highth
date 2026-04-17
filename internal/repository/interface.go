// Package repository handles database queries for sensor readings.
package repository

import (
	"context"
	"time"

	"github.com/kelanach/higth/internal/model"
)

// Querier defines the interface for sensor reading database operations.
// SensorRepository implements this interface.
type Querier interface {
	Query(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) ([]model.SensorReading, error)
	GetByID(ctx context.Context, id int64) (*model.SensorReading, error)
	GetStatsFromMV(ctx context.Context) (map[string]interface{}, error)
	GetRowCount(ctx context.Context) (int64, error)
	GetDeviceCount(ctx context.Context) (int64, error)
	Ping(ctx context.Context) error
	Close()
}
