// Package handler handles HTTP requests for health checks.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kelanach/higth/internal/model"
	"github.com/kelanach/higth/internal/service"
)

// HealthHandler handles HTTP requests for health checks.
type HealthHandler struct {
	service service.SensorServicer
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(service service.SensorServicer) *HealthHandler {
	return &HealthHandler{service: service}
}

// GetHealth handles GET /health
//
// It returns the health status of the service and its dependencies.
// The overall status is:
//   - "healthy": All dependencies are healthy
//   - "degraded": Cache is unhealthy but service can still function
//   - "unhealthy": Database is unhealthy, service cannot function
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Ping all dependencies with timing
	results := h.service.PingWithLatency(r.Context())

	// Build response
	checks := make(map[string]model.HealthCheck)
	overallStatus := model.HealthStatusHealthy

	for name, result := range results {
		check := model.HealthCheck{
			Status:    "healthy",
			LatencyMs: result.LatencyMs,
		}
		if result.Error != nil {
			check.Status = "unhealthy"
			check.Error = result.Error.Error()
			overallStatus = model.HealthStatusDegraded
		}
		checks[name] = check
	}

	// If database is unhealthy, mark overall as unhealthy (not just degraded)
	if checks["database"].Status == "unhealthy" {
		overallStatus = model.HealthStatusUnhealthy
	}

	response := model.NewHealthStatus(overallStatus, checks)

	// Set HTTP status code based on overall health
	var httpStatus int
	switch overallStatus {
	case model.HealthStatusHealthy:
		httpStatus = http.StatusOK
	case model.HealthStatusDegraded:
		httpStatus = http.StatusServiceUnavailable
	case model.HealthStatusUnhealthy:
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(response)
}

// GetReadiness handles GET /health/ready
//
// It returns 200 if the service is ready to accept requests.
func (h *HealthHandler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	results := h.service.Ping(r.Context())

	// Service is ready if database is healthy
	if results["database"] != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// GetLiveness handles GET /health/live
//
// It returns 200 if the service is alive (always returns 200).
func (h *HealthHandler) GetLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
