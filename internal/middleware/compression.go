// Package middleware provides HTTP middleware for the Higth API.
package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// gzipResponseWriter wraps a http.ResponseWriter to provide gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
}

// Write writes data to the gzip writer.
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gzipWriter.Write(b)
}

// WriteHeader writes the HTTP status code and sets appropriate headers.
func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	// Only set Content-Encoding header if not already set
	if w.Header().Get("Content-Encoding") == "" {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Flush flushes the gzip writer and the underlying response writer.
func (w *gzipResponseWriter) Flush() {
	w.gzipWriter.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Close closes the gzip writer.
func (w *gzipResponseWriter) Close() {
	w.gzipWriter.Close()
}

// GzipMiddleware returns a middleware that compresses HTTP responses using gzip.
// It only compresses responses for clients that support gzip encoding.
// It skips compression for responses that are already compressed (images, videos, etc.)
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip encoding
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Don't compress if the response writer already supports compression
		// (e.g., when using http.ServeContent with pre-compressed files)
		if r.Header.Get("Content-Encoding") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Create gzip response writer
		gw := gzipResponseWriter{
			ResponseWriter: w,
			gzipWriter:     gzip.NewWriter(w),
		}
		defer gw.Close()

		// Set the header
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		next.ServeHTTP(&gw, r)
	})
}

// GzipLevelMiddleware returns a middleware that compresses HTTP responses using gzip
// with a specific compression level (0-9, where 9 is best compression but slowest).
// For most web APIs, level 4-6 provides a good balance between speed and compression.
func GzipLevelMiddleware(level int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client accepts gzip encoding
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Create gzip writer with specified level
			gz, err := gzip.NewWriterLevel(w, level)
			if err != nil {
				// If invalid level, fall back to default
				gz = gzip.NewWriter(w)
			}

			gw := gzipResponseWriter{
				ResponseWriter: w,
				gzipWriter:     gz,
			}
			defer gw.Close()

			// Set the header
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			next.ServeHTTP(&gw, r)
		})
	}
}
