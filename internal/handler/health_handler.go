// Package handler handles HTTP requests for health checks.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kelanach/higth/internal/model"
	"github.com/kelanach/higth/internal/service"
)

// HealthHandler handles HTTP requests for health checks.
type HealthHandler struct {
	service *service.SensorService
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(service *service.SensorService) *HealthHandler {
	return &HealthHandler{service: service}
}

// GetHealth handles GET /health
//
// It returns the health status of the service and its dependencies.
// The overall status is:
//   - "passing": All dependencies are healthy
//   - "degraded": At least one dependency is unhealthy but the service can still function
//   - "failing": The service cannot function
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Ping all dependencies
	results := h.service.Ping(r.Context())

	// Build response
	checks := make(map[string]model.HealthCheck)
	overallStatus := model.HealthStatusPassing

	for name, err := range results {
		check := model.HealthCheck{Status: "passing"}
		if err != nil {
			check.Status = "failing"
			check.Error = err.Error()
			overallStatus = model.HealthStatusDegraded
		}
		checks[name] = check
	}

	// If database is failing, the overall status is "failing"
	if checks["database"].Status == "failing" {
		overallStatus = model.HealthStatusFailing
	}

	response := model.NewHealthStatus(overallStatus, checks)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", string(rune(time.Since(start).Milliseconds())))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetReadiness handles GET /health/ready
//
// It returns 200 if the service is ready to accept requests.
func (h *HealthHandler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	results := h.service.Ping(r.Context())

	// Service is ready if database is healthy
	if results["database"] != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// GetLiveness handles GET /health/live
//
// It returns 200 if the service is alive (always returns 200).
func (h *HealthHandler) GetLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
