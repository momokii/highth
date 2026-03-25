// Package model defines data structures for the IoT sensor query system.
package model

import "time"

// HealthStatus represents the health check response.
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Checks    map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// HealthStatusValues represents possible health status values.
type HealthStatusValues string

const (
	// HealthStatusHealthy represents a fully healthy system
	HealthStatusHealthy HealthStatusValues = "healthy"
	// HealthStatusDegraded represents a partially functional system
	HealthStatusDegraded HealthStatusValues = "degraded"
	// HealthStatusUnhealthy represents an unhealthy system
	HealthStatusUnhealthy HealthStatusValues = "unhealthy"
)

// NewHealthStatus creates a new HealthStatus with the given checks.
func NewHealthStatus(status HealthStatusValues, checks map[string]HealthCheck) HealthStatus {
	return HealthStatus{
		Status:    string(status),
		Timestamp: time.Now().Format(time.RFC3339),
		Checks:    checks,
	}
}

// NewHealthCheck creates a new HealthCheck with the given parameters.
func NewHealthCheck(status string, latencyMs int64, err string) HealthCheck {
	return HealthCheck{
		Status:    status,
		LatencyMs: latencyMs,
		Error:     err,
	}
}
