// Package handler handles HTTP requests for sensor readings.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/kelanach/higth/internal/middleware"
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
//   - from (optional): ISO 8601 timestamp for start of time range
//   - to (optional): ISO 8601 timestamp for end of time range
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

	// Parse from (optional)
	var from *time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		} else {
			h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "from must be a valid ISO 8601 timestamp", start, ErrorDetails{
				Parameter:   "from",
				Provided:    fromStr,
				Constraints: map[string]any{"format": "ISO 8601 (RFC3339)"},
			})
			return
		}
	}

	// Parse to (optional)
	var to *time.Time
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		} else {
			h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "to must be a valid ISO 8601 timestamp", start, ErrorDetails{
				Parameter:   "to",
				Provided:    toStr,
				Constraints: map[string]any{"format": "ISO 8601 (RFC3339)"},
			})
			return
		}
	}

	// Call service layer
	cacheStatus, readings, err := h.service.GetSensorReadings(r.Context(), deviceID, limit, readingType, from, to)
	if err != nil {
		h.handleServiceError(w, r, err, start)
		return
	}

	// Record cache hit/miss metrics
	switch cacheStatus {
	case "HIT":
		middleware.CacheHitsTotal.Inc()
	case "MISS":
		middleware.CacheMissesTotal.Inc()
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
	}, start, cacheStatus)
}

// GetSensorReadingByID handles GET /api/v1/sensor-readings/{id}
//
// Path parameters:
//   - id (required): Primary key ID of the sensor reading
func (h *SensorHandler) GetSensorReadingByID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse id from URL path
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "id is required", start, ErrorDetails{})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_PARAMETER", "id must be a positive integer", start, ErrorDetails{
			Parameter:   "id",
			Provided:    idStr,
			Constraints: map[string]any{"type": "integer", "min": 1},
		})
		return
	}

	// Call service layer
	cacheStatus, reading, err := h.service.GetSensorReadingByID(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err, start)
		return
	}

	// Record cache hit/miss metrics
	switch cacheStatus {
	case "HIT":
		middleware.CacheHitsTotal.Inc()
	case "MISS":
		middleware.CacheMissesTotal.Inc()
	}

	// Return response
	h.writeResponse(w, r, http.StatusOK, map[string]interface{}{
		"data": reading,
		"meta": map[string]interface{}{
			"id": fmt.Sprintf("%d", id),
		},
	}, start, cacheStatus)
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
// If cacheStatus is non-empty, sets X-Cache-Status header (HIT, MISS, or BYPASS).
func (h *SensorHandler) writeResponse(w http.ResponseWriter, r *http.Request, status int, data interface{}, start time.Time, cacheStatus string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	w.Header().Set("Cache-Control", "public, max-age=30")

	// Set cache status header if provided
	if cacheStatus != "" {
		w.Header().Set("X-Cache-Status", cacheStatus)
	}

	// Get request ID from context (set by chi middleware)
	if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
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
	if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
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
	if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
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
	case errors.Is(err, service.ErrReadingNotFound):
		h.writeError(w, r, http.StatusNotFound, "READING_NOT_FOUND", err.Error(), start, ErrorDetails{})
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
	}, start, "BYPASS") // Stats use MV, not sensor cache
}
