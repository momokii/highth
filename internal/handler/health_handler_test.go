package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kelanach/higth/internal/service"
)

// ---------------------------------------------------------------------------
// GetHealth tests
// ---------------------------------------------------------------------------

func TestGetHealth(t *testing.T) {
	tests := []struct {
		name       string
		mockPing   func(ctx context.Context) map[string]service.PingResult
		wantStatus int
		wantStatusStr string // expected "status" value in JSON body
	}{
		{
			name: "healthy returns 200",
			mockPing: func(_ context.Context) map[string]service.PingResult {
				return map[string]service.PingResult{
					"database": {Error: nil, LatencyMs: 1},
					"cache":    {Error: nil, LatencyMs: 2},
				}
			},
			wantStatus:    http.StatusOK,
			wantStatusStr: "healthy",
		},
		{
			name: "database unhealthy returns 503",
			mockPing: func(_ context.Context) map[string]service.PingResult {
				return map[string]service.PingResult{
					"database": {Error: errFoo("connection refused"), LatencyMs: 0},
					"cache":    {Error: nil, LatencyMs: 1},
				}
			},
			wantStatus:    http.StatusServiceUnavailable,
			wantStatusStr: "unhealthy",
		},
		{
			name: "cache unhealthy returns 503 degraded",
			mockPing: func(_ context.Context) map[string]service.PingResult {
				return map[string]service.PingResult{
					"database": {Error: nil, LatencyMs: 1},
					"cache":    {Error: errFoo("redis down"), LatencyMs: 0},
				}
			},
			wantStatus:    http.StatusServiceUnavailable,
			wantStatusStr: "degraded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockService{pingWithLatencyFn: tc.mockPing}
			h := NewHealthHandler(ms)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			h.GetHealth(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}

			body := parseJSON(t, rec.Body.Bytes())

			gotStatus, _ := body["status"].(string)
			if gotStatus != tc.wantStatusStr {
				t.Errorf("body status = %q; want %q", gotStatus, tc.wantStatusStr)
			}

			// Verify checks map is present
			if _, ok := body["checks"]; !ok {
				t.Error("response missing 'checks' key")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetReadiness tests
// ---------------------------------------------------------------------------

func TestGetReadiness(t *testing.T) {
	tests := []struct {
		name       string
		mockPing   func(ctx context.Context) map[string]error
		wantStatus int
		wantBody   string // expected "status" value in JSON body
	}{
		{
			name: "ready returns 200",
			mockPing: func(_ context.Context) map[string]error {
				return map[string]error{
					"database": nil,
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "ready",
		},
		{
			name: "db error returns 503",
			mockPing: func(_ context.Context) map[string]error {
				return map[string]error{
					"database": errFoo("connection refused"),
				}
			},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "not ready",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockService{pingFn: tc.mockPing}
			h := NewHealthHandler(ms)

			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			rec := httptest.NewRecorder()

			h.GetReadiness(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}

			body := parseJSON(t, rec.Body.Bytes())

			gotStatus, _ := body["status"].(string)
			if gotStatus != tc.wantBody {
				t.Errorf("body status = %q; want %q", gotStatus, tc.wantBody)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetLiveness tests
// ---------------------------------------------------------------------------

func TestGetLiveness(t *testing.T) {
	t.Run("always returns 200", func(t *testing.T) {
		ms := &mockService{}
		h := NewHealthHandler(ms)

		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rec := httptest.NewRecorder()

		h.GetLiveness(rec, req)

		resp := rec.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d; want %d", resp.StatusCode, http.StatusOK)
		}

		body := parseJSON(t, rec.Body.Bytes())

		gotStatus, _ := body["status"].(string)
		if gotStatus != "alive" {
			t.Errorf("body status = %q; want %q", gotStatus, "alive")
		}
	})
}

// ---------------------------------------------------------------------------
// test error type
// ---------------------------------------------------------------------------

// errFoo is a minimal error type used in mock return values.
type errFoo string

func (e errFoo) Error() string { return string(e) }
