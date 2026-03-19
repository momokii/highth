// internal/model/sensor.go
package model

import (
	"time"
)

// SensorReading represents a single sensor reading from the database
type SensorReading struct {
	ID          string        `json:"id"`
	DeviceID    string        `json:"device_id"`
	Timestamp   time.Time     `json:"timestamp"`
	ReadingType string        `json:"reading_type"`
	Value       float64       `json:"value"`
	Unit        string        `json:"unit"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// HealthStatus represents the health check response
type HealthStatus struct {
	Status    string              `json:"status"`
	Timestamp string              `json:"timestamp"`
	Checks    map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents a single health check result
type HealthCheck struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}
