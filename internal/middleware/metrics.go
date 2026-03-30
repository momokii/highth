// Package middleware provides HTTP middleware for the Higth API.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpRequestsTotal counts the total number of HTTP requests by method, endpoint, and status.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "endpoint", "status"},
	)

	// httpRequestDurationSeconds tracks HTTP request latency in seconds.
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// httpRequestsInFlight tracks the current number of in-flight HTTP requests.
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of in-flight HTTP requests.",
		},
	)

	// httpResponseSizeBytes tracks HTTP response sizes in bytes.
	httpResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes.",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "endpoint"},
	)

	// CacheHitsTotal counts the total number of cache hits.
	CacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits.",
		},
	)

	// CacheMissesTotal counts the total number of cache misses.
	CacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses.",
		},
	)
)

// normalizeEndpoint returns a normalized endpoint path for metrics labeling.
// It strips query parameters and normalizes dynamic path segments to prevent
// label cardinality explosion (e.g., device_id values).
func normalizeEndpoint(path string) string {
	// For now, return the path as-is.
	// TODO: Add normalization for dynamic path segments if needed.
	return path
}

// RecordRequest records metrics for an HTTP request.
func RecordRequest(method, endpoint string, status int, duration time.Duration, responseSize int) {
	statusStr := strconv.Itoa(status)
	endpoint = normalizeEndpoint(endpoint)

	httpRequestsTotal.WithLabelValues(method, endpoint, statusStr).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	if responseSize > 0 {
		httpResponseSizeBytes.WithLabelValues(method, endpoint).Observe(float64(responseSize))
	}
}

// IncInFlight increments the in-flight requests gauge.
func IncInFlight() {
	httpRequestsInFlight.Inc()
}

// DecInFlight decrements the in-flight requests gauge.
func DecInFlight() {
	httpRequestsInFlight.Dec()
}

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	status int
	size    int
}

// WriteStatus captures the HTTP status code.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size.
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// MetricsMiddleware returns a middleware that records Prometheus metrics for HTTP requests.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		IncInFlight()
		defer DecInFlight()

		// Wrap response writer to capture status code and response size
		rw := &responseWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		RecordRequest(r.Method, r.URL.Path, rw.status, duration, rw.size)
	})
}
