// Package handler handles HTTP requests for sensor readings.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kelanach/higth/internal/service"
)

// SensorHandler handles HTTP requests for sensor readings.
type SensorHandler struct {
	service *service.SensorService
}

// NewSensorHandler creates a new sensor handler.
func NewSensorHandler(service *service.SensorService) *SensorHandler {
	return &SensorHandler{service: service}
}

// GetSensorReadings handles GET /api/v1/sensor-readings
//
// Query parameters:
//   - device_id (optional): Device identifier (if not provided, returns readings from all devices)
//   - limit (optional): Maximum number of readings to return (1-500, default 10)
//   - reading_type (optional): Filter by reading type (temperature, humidity, pressure)
func (h *SensorHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse device_id (optional)
	deviceID := r.URL.Query().Get("device_id")

	// Parse and validate limit
	limit := h.parseIntOrDefault(r.URL.Query().Get("limit"), 10)
	if limit < 1 || limit > 500 {
		h.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "limit must be between 1 and 500", start)
		return
	}

	// Parse reading_type (optional)
	readingType := r.URL.Query().Get("reading_type")

	// Call service layer
	readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)
	if err != nil {
		h.handleServiceError(w, err, start)
		return
	}

	// Return response
	h.writeResponse(w, http.StatusOK, map[string]interface{}{
		"data": readings,
		"meta": map[string]interface{}{
			"count":        len(readings),
			"limit":        limit,
			"device_id":    deviceID,
			"reading_type": readingType,
		},
	}, start)
}

// parseIntOrDefault parses a string to int or returns the default value.
func (h *SensorHandler) parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var val int
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return defaultVal
	}
	return val
}

// writeResponse writes a successful JSON response.
func (h *SensorHandler) writeResponse(w http.ResponseWriter, status int, data interface{}, start time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response in JSON format.
func (h *SensorHandler) writeError(w http.ResponseWriter, status int, code, message string, start time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// handleServiceError maps service errors to HTTP status codes.
func (h *SensorHandler) handleServiceError(w http.ResponseWriter, err error, start time.Time) {
	switch {
	case errors.Is(err, service.ErrInvalidParameter):
		h.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error(), start)
	case errors.Is(err, service.ErrDeviceNotFound):
		h.writeError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", err.Error(), start)
	default:
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", start)
	}
}

// GetStats handles GET /api/v1/stats
func (h *SensorHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get stats", start)
		return
	}

	h.writeResponse(w, http.StatusOK, map[string]interface{}{
		"data": stats,
	}, start)
}
