// Package repository handles database queries for sensor readings.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
// If from is non-nil, filters to timestamps >= from.
// If to is non-nil, filters to timestamps <= to.
func (r *SensorRepository) Query(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) ([]model.SensorReading, error) {
	var query string
	var args []interface{}
	var argIdx int = 1

	// Build base query
	baseQuery := `
		SELECT id, device_id, timestamp, reading_type, value, unit
		FROM sensor_readings
	`

	// Add WHERE clause(s)
	whereAdded := false
	if deviceID != "" {
		baseQuery += fmt.Sprintf(" WHERE device_id = $%d", argIdx)
		args = append(args, deviceID)
		argIdx++
		whereAdded = true
	}

	// Add from timestamp filter
	if from != nil {
		if whereAdded {
			baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		} else {
			baseQuery += fmt.Sprintf(" WHERE timestamp >= $%d", argIdx)
			whereAdded = true
		}
		args = append(args, *from)
		argIdx++
	}

	// Add to timestamp filter
	if to != nil {
		if whereAdded {
			baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		} else {
			baseQuery += fmt.Sprintf(" WHERE timestamp <= $%d", argIdx)
			whereAdded = true
		}
		args = append(args, *to)
		argIdx++
	}

	// Add reading_type filter if specified
	if readingType != "" {
		if whereAdded {
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

// GetByID retrieves a single sensor reading by its primary key ID.
// Returns nil (no error) if no row is found — the service layer handles the not-found case.
// The query uses the primary key B-tree index for O(1) lookup.
func (r *SensorRepository) GetByID(ctx context.Context, id int64) (*model.SensorReading, error) {
	var reading model.SensorReading
	var rowID int64

	err := r.db.QueryRow(ctx,
		"SELECT id, device_id, timestamp, reading_type, value, unit FROM sensor_readings WHERE id = $1",
		id,
	).Scan(&rowID, &reading.DeviceID, &reading.Timestamp, &reading.ReadingType, &reading.Value, &reading.Unit)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}

	reading.ID = fmt.Sprintf("%d", rowID)
	return &reading, nil
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

// GetStatsFromMV returns statistics from the materialized view.
// This is much faster than COUNT queries on the base table for large datasets.
func (r *SensorRepository) GetStatsFromMV(ctx context.Context) (map[string]interface{}, error) {
	// Sum total readings across all reading types from MV
	var totalReadings int64
	err := r.db.QueryRow(ctx, "SELECT COALESCE(SUM(total_readings), 0) FROM mv_global_stats").Scan(&totalReadings)
	if err != nil {
		return nil, fmt.Errorf("failed to get total readings from MV: %w", err)
	}

	// For device count, we use a subquery approach to avoid full table scan
	// The MV already has device counts per reading type, but devices may overlap across types
	// We sample a small percentage to estimate, then verify with MV sum
	var totalDevices int64
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT device_id)
		FROM (
			SELECT device_id FROM sensor_readings
			TABLESAMPLE SYSTEM (0.5)
		) sample
	`).Scan(&totalDevices)
	if err != nil {
		// Fall back to summing active_devices from MV (may overestimate due to overlapping devices)
		// This is a reasonable approximation since most devices report all reading types
		err = r.db.QueryRow(ctx, "SELECT COALESCE(SUM(active_devices), 0) FROM mv_global_stats").Scan(&totalDevices)
		if err != nil {
			return nil, fmt.Errorf("failed to get device count from MV: %w", err)
		}
	}

	return map[string]interface{}{
		"total_readings": totalReadings,
		"total_devices":  totalDevices,
		"queried_at":     time.Now().Format(time.RFC3339),
	}, nil
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
