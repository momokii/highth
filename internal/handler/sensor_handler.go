// Package handler handles HTTP requests for sensor readings.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/kelanach/higth/internal/service"
)

// ErrorDetails represents additional error information for validation errors.
type ErrorDetails struct {
	Parameter   string      `json:"parameter,omitempty"`
	Provided    interface{} `json:"provided,omitempty"`
	Constraints interface{} `json:"constraints,omitempty"`
}

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
//   - device_id (required): Device identifier
//   - limit (optional): Maximum number of readings to return (1-500, default 10)
//   - reading_type (optional): Filter by reading type (temperature, humidity, pressure)
func (h *SensorHandler) GetSensorReadings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse device_id (required)
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "device_id is required", start, ErrorDetails{})
		return
	}

	// Parse and validate limit
	limit := h.parseIntOrDefault(r.URL.Query().Get("limit"), 10)
	if limit < 1 || limit > 500 {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "limit must be between 1 and 500", start, ErrorDetails{
			Parameter:   "limit",
			Provided:    r.URL.Query().Get("limit"),
			Constraints: map[string]any{"min": 1, "max": 500},
		})
		return
	}

	// Parse reading_type (optional)
	readingType := r.URL.Query().Get("reading_type")

	// Call service layer
	readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType)
	if err != nil {
		h.handleServiceError(w, r, err, start)
		return
	}

	// Return response
	h.writeResponse(w, r, http.StatusOK, map[string]interface{}{
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
func (h *SensorHandler) writeResponse(w http.ResponseWriter, r *http.Request, status int, data interface{}, start time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	w.Header().Set("Cache-Control", "public, max-age=30")

	// Get request ID from context (set by chi middleware)
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response in JSON format.
func (h *SensorHandler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, start time.Time, details ErrorDetails) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))

	// Add request ID header if available
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}

	errorResp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":      code,
			"message":   message,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}

	// Add request ID to response body if available
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		errorResp["error"].(map[string]interface{})["request_id"] = requestID
	}

	// Add details if provided
	if details.Parameter != "" {
		errorResp["error"].(map[string]interface{})["details"] = details
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResp)
}

// handleServiceError maps service errors to HTTP status codes.
func (h *SensorHandler) handleServiceError(w http.ResponseWriter, r *http.Request, err error, start time.Time) {
	switch {
	case errors.Is(err, service.ErrInvalidParameter):
		h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", err.Error(), start, ErrorDetails{})
	case errors.Is(err, service.ErrDeviceNotFound):
		h.writeError(w, r, http.StatusNotFound, "DEVICE_NOT_FOUND", err.Error(), start, ErrorDetails{})
	default:
		h.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", start, ErrorDetails{})
	}
}

// GetStats handles GET /api/v1/stats
func (h *SensorHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		// Log the actual error for debugging
		log.Printf("ERROR: GetStats failed: %v", err)
		h.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get stats", start, ErrorDetails{})
		return
	}

	h.writeResponse(w, r, http.StatusOK, map[string]interface{}{
		"data": stats,
	}, start)
}
