package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kelanach/higth/internal/model"
	"github.com/kelanach/higth/internal/service"
)

// ---------------------------------------------------------------------------
// Mock service
// ---------------------------------------------------------------------------

type mockService struct {
	getSensorReadingsFn    func(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) (string, []model.SensorReading, error)
	getSensorReadingByIDFn func(ctx context.Context, id int64) (string, *model.SensorReading, error)
	getStatsFn             func(ctx context.Context) (map[string]interface{}, error)
	pingFn                 func(ctx context.Context) map[string]error
	pingWithLatencyFn      func(ctx context.Context) map[string]service.PingResult
}

func (m *mockService) GetSensorReadings(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) (string, []model.SensorReading, error) {
	if m.getSensorReadingsFn != nil {
		return m.getSensorReadingsFn(ctx, deviceID, limit, readingType, from, to)
	}
	return "", nil, nil
}

func (m *mockService) GetSensorReadingByID(ctx context.Context, id int64) (string, *model.SensorReading, error) {
	if m.getSensorReadingByIDFn != nil {
		return m.getSensorReadingByIDFn(ctx, id)
	}
	return "", nil, nil
}

func (m *mockService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx)
	}
	return nil, nil
}

func (m *mockService) Ping(ctx context.Context) map[string]error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockService) PingWithLatency(ctx context.Context) map[string]service.PingResult {
	if m.pingWithLatencyFn != nil {
		return m.pingWithLatencyFn(ctx)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseJSON is a small helper to decode a JSON response body into a map.
func parseJSON(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON response: %v\nbody: %s", err, string(body))
	}
	return result
}

func newSensorHandler(ms *mockService) *SensorHandler {
	return NewSensorHandler(ms)
}

// ---------------------------------------------------------------------------
// PK lookup tests (id query param)
// ---------------------------------------------------------------------------

func TestGetSensorReadings_PKLookup(t *testing.T) {
	tests := []struct {
		name       string
		idParam    string
		mockFn     func(ctx context.Context, id int64) (string, *model.SensorReading, error)
		wantStatus int
		wantCode   string // expected error code in response body (empty for success)
	}{
		{
			name:    "valid ID returns 200 with data",
			idParam: "42",
			mockFn: func(_ context.Context, id int64) (string, *model.SensorReading, error) {
				return "MISS", &model.SensorReading{
					ID:          "42",
					DeviceID:    "dev-1",
					Timestamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					ReadingType: "temperature",
					Value:       23.5,
					Unit:        "celsius",
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "negative ID returns 400",
			idParam:    "-5",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "zero ID returns 400",
			idParam:    "0",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "non-numeric ID returns 400",
			idParam:    "abc",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:    "not found returns 404",
			idParam: "99999",
			mockFn: func(_ context.Context, id int64) (string, *model.SensorReading, error) {
				return "", nil, service.ErrReadingNotFound
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "READING_NOT_FOUND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockService{}
			if tc.mockFn != nil {
				ms.getSensorReadingByIDFn = tc.mockFn
			}

			h := newSensorHandler(ms)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sensor-readings?id="+tc.idParam, nil)
			rec := httptest.NewRecorder()

			h.GetSensorReadings(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}

			body := parseJSON(t, rec.Body.Bytes())

			// Verify structure for success vs error
			if tc.wantStatus == http.StatusOK {
				if _, ok := body["data"]; !ok {
					t.Error("response missing 'data' key")
				}
				if _, ok := body["meta"]; !ok {
					t.Error("response missing 'meta' key")
				}
			} else {
				errObj, ok := body["error"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected 'error' object in response, got: %v", body)
				}
				if code, _ := errObj["code"].(string); code != tc.wantCode {
					t.Errorf("error code = %q; want %q", code, tc.wantCode)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Device query tests (device_id query param)
// ---------------------------------------------------------------------------

func TestGetSensorReadings_DeviceQuery(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		mockFn     func(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) (string, []model.SensorReading, error)
		wantStatus int
		wantCode   string
		checkExtra func(t *testing.T, body map[string]interface{}, headers http.Header)
	}{
		{
			name: "valid device_id returns 200 with data",
			url:  "/api/v1/sensor-readings?device_id=dev-1",
			mockFn: func(_ context.Context, deviceID string, _ int, _ string, _, _ *time.Time) (string, []model.SensorReading, error) {
				return "HIT", []model.SensorReading{
					{ID: "1", DeviceID: deviceID, Value: 10.0},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing both id and device_id returns 400",
			url:        "/api/v1/sensor-readings",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "both id and device_id returns 400",
			url:        "/api/v1/sensor-readings?id=1&device_id=dev-1",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "limit too high returns 400",
			url:        "/api/v1/sensor-readings?device_id=dev-1&limit=501",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "invalid from timestamp returns 400",
			url:        "/api/v1/sensor-readings?device_id=dev-1&from=not-a-date",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "invalid to timestamp returns 400",
			url:        "/api/v1/sensor-readings?device_id=dev-1&to=bad-time",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "from > to returns 400",
			url:        "/api/v1/sensor-readings?device_id=dev-1&from=2025-12-01T00:00:00Z&to=2025-01-01T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name: "cache status header is set",
			url:  "/api/v1/sensor-readings?device_id=dev-1",
			mockFn: func(_ context.Context, _ string, _ int, _ string, _, _ *time.Time) (string, []model.SensorReading, error) {
				return "HIT", []model.SensorReading{}, nil
			},
			wantStatus: http.StatusOK,
			checkExtra: func(t *testing.T, _ map[string]interface{}, headers http.Header) {
				t.Helper()
				if v := headers.Get("X-Cache-Status"); v != "HIT" {
					t.Errorf("X-Cache-Status = %q; want %q", v, "HIT")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockService{}
			if tc.mockFn != nil {
				ms.getSensorReadingsFn = tc.mockFn
			}

			h := newSensorHandler(ms)
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()

			h.GetSensorReadings(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}

			body := parseJSON(t, rec.Body.Bytes())

			if tc.wantStatus == http.StatusOK {
				if _, ok := body["data"]; !ok {
					t.Error("response missing 'data' key")
				}
				if _, ok := body["meta"]; !ok {
					t.Error("response missing 'meta' key")
				}
			} else {
				errObj, ok := body["error"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected 'error' object in response, got: %v", body)
				}
				if code, _ := errObj["code"].(string); code != tc.wantCode {
					t.Errorf("error code = %q; want %q", code, tc.wantCode)
				}
			}

			if tc.checkExtra != nil {
				tc.checkExtra(t, body, resp.Header)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetStats tests
// ---------------------------------------------------------------------------

func TestGetStats(t *testing.T) {
	tests := []struct {
		name       string
		mockFn     func(ctx context.Context) (map[string]interface{}, error)
		wantStatus int
		wantData   bool // whether "data" key should be present in response
	}{
		{
			name: "success returns 200",
			mockFn: func(_ context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{
					"total_readings": float64(50000000),
					"total_devices":  float64(1200),
					"queried_at":     time.Now().Format(time.RFC3339),
				}, nil
			},
			wantStatus: http.StatusOK,
			wantData:   true,
		},
		{
			name: "service error returns 500",
			mockFn: func(_ context.Context) (map[string]interface{}, error) {
				return nil, errors.New("database connection lost")
			},
			wantStatus: http.StatusInternalServerError,
			wantData:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockService{getStatsFn: tc.mockFn}
			h := newSensorHandler(ms)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
			rec := httptest.NewRecorder()

			h.GetStats(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}

			body := parseJSON(t, rec.Body.Bytes())

			if tc.wantData {
				if _, ok := body["data"]; !ok {
					t.Error("response missing 'data' key")
				}
			} else {
				if _, ok := body["error"]; !ok {
					t.Error("expected 'error' key in response")
				}
			}
		})
	}
}
