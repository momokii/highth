// Package middleware provides HTTP middleware for the Higth API.
package middleware

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler returns the Prometheus metrics handler.
// It exposes the /metrics endpoint for Prometheus scraping.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
