// Package repository handles database queries for sensor readings.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kelanach/higth/internal/model"
)

// SensorRepository handles database queries for sensor readings.
type SensorRepository struct {
	db *pgxpool.Pool
}

// Config holds database configuration.
type Config struct {
	DatabaseURL          string
	MaxOpenConns         int32
	MinOpenConns         int32
	MaxConnLifetime      time.Duration
	MaxConnIdleTime      time.Duration
	HealthCheckPeriod    time.Duration
}

// New creates a new SensorRepository with the given configuration.
func New(cfg Config) (*SensorRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.MaxConns = cfg.MaxOpenConns
	config.MinConns = cfg.MinOpenConns
	config.MaxConnLifetime = cfg.MaxConnLifetime
	config.MaxConnIdleTime = cfg.MaxConnIdleTime
	config.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SensorRepository{db: pool}, nil
}

// Query retrieves sensor readings from the database.
// It returns up to limit readings for the specified device_id,
// ordered by timestamp DESC (newest first).
// If deviceID is empty, returns readings from all devices.
// If readingType is specified, filters by that type.
func (r *SensorRepository) Query(ctx context.Context, deviceID string, limit int, readingType string) ([]model.SensorReading, error) {
	var query string
	var args []interface{}
	var argIdx int = 1

	// Build base query
	baseQuery := `
		SELECT id, device_id, timestamp, reading_type, value, unit
		FROM sensor_readings
	`

	// Add WHERE clause only if device_id is provided
	if deviceID != "" {
		baseQuery += fmt.Sprintf(" WHERE device_id = $%d", argIdx)
		args = append(args, deviceID)
		argIdx++
	}

	// Add reading_type filter if specified
	if readingType != "" {
		if deviceID != "" {
			query = baseQuery + fmt.Sprintf(" AND reading_type = $%d", argIdx)
		} else {
			query = baseQuery + fmt.Sprintf(" WHERE reading_type = $%d", argIdx)
		}
		args = append(args, readingType)
		argIdx++
	} else {
		query = baseQuery
	}

	// Add ORDER BY and LIMIT
	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var readings []model.SensorReading
	for rows.Next() {
		var r model.SensorReading
		var id int64
		if err := rows.Scan(&id, &r.DeviceID, &r.Timestamp, &r.ReadingType, &r.Value, &r.Unit); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		r.ID = fmt.Sprintf("%d", id)
		readings = append(readings, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return readings, nil
}

// GetRowCount returns the total number of sensor readings in the database.
func (r *SensorRepository) GetRowCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM sensor_readings").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get row count: %w", err)
	}
	return count, nil
}

// GetDeviceCount returns the number of unique devices in the database.
func (r *SensorRepository) GetDeviceCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(DISTINCT device_id) FROM sensor_readings").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get device count: %w", err)
	}
	return count, nil
}

// Ping checks if the database connection is alive.
func (r *SensorRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

// Close closes the database connection pool.
func (r *SensorRepository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

// DBStats represents database connection pool statistics.
type DBStats struct {
	OpenConnections int
	IdleConnections int
	MaxConnections  int
}

// Stats returns database connection pool statistics.
// This is used for monitoring and metrics collection.
func (r *SensorRepository) Stats() DBStats {
	stat := r.db.Stat()
	return DBStats{
		OpenConnections: int(stat.TotalConns()),
		IdleConnections: int(stat.IdleConns()),
		MaxConnections:  int(stat.MaxConns()),
	}
}
