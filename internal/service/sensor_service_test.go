package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kelanach/higth/internal/model"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockRepo implements repository.Querier using function fields.
type mockRepo struct {
	queryFn          func(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) ([]model.SensorReading, error)
	getByIDFn        func(ctx context.Context, id int64) (*model.SensorReading, error)
	getStatsFromMVFn func(ctx context.Context) (map[string]interface{}, error)
	getRowCountFn    func(ctx context.Context) (int64, error)
	getDeviceCountFn func(ctx context.Context) (int64, error)
	pingFn           func(ctx context.Context) error
}

func (m *mockRepo) Query(ctx context.Context, deviceID string, limit int, readingType string, from, to *time.Time) ([]model.SensorReading, error) {
	if m.queryFn == nil {
		return nil, nil
	}
	return m.queryFn(ctx, deviceID, limit, readingType, from, to)
}

func (m *mockRepo) GetByID(ctx context.Context, id int64) (*model.SensorReading, error) {
	if m.getByIDFn == nil {
		return nil, nil
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockRepo) GetStatsFromMV(ctx context.Context) (map[string]interface{}, error) {
	if m.getStatsFromMVFn == nil {
		return nil, nil
	}
	return m.getStatsFromMVFn(ctx)
}

func (m *mockRepo) GetRowCount(ctx context.Context) (int64, error) {
	if m.getRowCountFn == nil {
		return 0, nil
	}
	return m.getRowCountFn(ctx)
}

func (m *mockRepo) GetDeviceCount(ctx context.Context) (int64, error) {
	if m.getDeviceCountFn == nil {
		return 0, nil
	}
	return m.getDeviceCountFn(ctx)
}

func (m *mockRepo) Ping(ctx context.Context) error {
	if m.pingFn == nil {
		return nil
	}
	return m.pingFn(ctx)
}

func (m *mockRepo) Close() {}

// mockCache implements cache.Cache using function fields.
type mockCache struct {
	getFn        func(ctx context.Context, key string, dest interface{}) error
	setFn        func(ctx context.Context, key string, value interface{}) error
	setWithTTLFn func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	deleteFn     func(ctx context.Context, key string) error
	flushAllFn   func(ctx context.Context) error
	enabled      bool
	pingFn       func(ctx context.Context) error
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	if m.getFn == nil {
		return fmt.Errorf("key not found")
	}
	return m.getFn(ctx, key, dest)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}) error {
	if m.setFn == nil {
		return nil
	}
	return m.setFn(ctx, key, value)
}

func (m *mockCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.setWithTTLFn == nil {
		return nil
	}
	return m.setWithTTLFn(ctx, key, value, ttl)
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.deleteFn == nil {
		return nil
	}
	return m.deleteFn(ctx, key)
}

func (m *mockCache) FlushAll(ctx context.Context) error {
	if m.flushAllFn == nil {
		return nil
	}
	return m.flushAllFn(ctx)
}

func (m *mockCache) IsEnabled() bool { return m.enabled }

func (m *mockCache) Ping(ctx context.Context) error {
	if m.pingFn == nil {
		return nil
	}
	return m.pingFn(ctx)
}

func (m *mockCache) Close() error { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newSvc creates a SensorService with given mocks. Pass nil for cache to test
// the no-cache path. The cache is enabled by default when non-nil.
func newSvc(repo *mockRepo, mc *mockCache) *SensorService {
	var c interface {
		Get(context.Context, string, interface{}) error
		Set(context.Context, string, interface{}) error
		SetWithTTL(context.Context, string, interface{}, time.Duration) error
		Delete(context.Context, string) error
		FlushAll(context.Context) error
		IsEnabled() bool
		Ping(context.Context) error
		Close() error
	}
	if mc != nil {
		c = mc
	}
	return New(repo, c, Config{CacheEnabled: true})
}

// ptrTime returns a pointer to the given time.Time.
func ptrTime(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// Tests: isValidDeviceID
// ---------------------------------------------------------------------------

func TestIsValidDeviceID(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)

	tests := []struct {
		name     string
		deviceID string
		want     bool
	}{
		{"alphanumeric and hyphen", "sensor-001", true},
		{"alphanumeric and underscore", "device_123", true},
		{"uppercase only", "ABC", true},
		{"all numeric", "12345", true},
		{"mixed valid chars", "aB3-d_e4", true},
		{"exactly 50 chars", strings.Repeat("a", 50), true},
		{"empty string", "", false},
		{"51 chars too long", strings.Repeat("a", 51), false},
		{"contains space", "sensor 001", false},
		{"contains @", "sensor@001", false},
		{"contains dot", "sensor.001", false},
		{"contains slash", "sensor/001", false},
		{"single hyphen", "-", true},
		{"single underscore", "_", true},
		{"single letter", "a", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isValidDeviceID(tt.deviceID)
			if got != tt.want {
				t.Errorf("isValidDeviceID(%q) = %v, want %v", tt.deviceID, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: isValidReadingType
// ---------------------------------------------------------------------------

func TestIsValidReadingType(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)

	tests := []struct {
		name        string
		readingType string
		want        bool
	}{
		{"temperature", "temperature", true},
		{"humidity", "humidity", true},
		{"short alphabetic", "abc", true},
		{"single letter", "a", true},
		{"all numeric", "123", true},
		{"mixed alphanumeric", "temp42", true},
		{"empty string", "", false},
		{"31 chars too long", strings.Repeat("a", 31), false},
		{"exactly 30 chars", strings.Repeat("a", 30), true},
		{"contains hyphen", "temp-high", false},
		{"contains underscore", "temp_high", false},
		{"contains space", "temp high", false},
		{"contains dot", "temp.high", false},
		{"uppercase letters", "TEMPERATURE", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isValidReadingType(tt.readingType)
			if got != tt.want {
				t.Errorf("isValidReadingType(%q) = %v, want %v", tt.readingType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: cacheKey
// ---------------------------------------------------------------------------

func TestCacheKey(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	later := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		deviceID    string
		limit       int
		readingType string
		from        *time.Time
		to          *time.Time
		want        string
	}{
		{
			name:        "no reading type, no time filters",
			deviceID:    "dev-1",
			limit:       10,
			readingType: "",
			from:        nil,
			to:          nil,
			want:        "sensor:dev-1:readings:10",
		},
		{
			name:        "with reading type, no time filters",
			deviceID:    "dev-1",
			limit:       20,
			readingType: "temperature",
			from:        nil,
			to:          nil,
			want:        "sensor:dev-1:readings:20:temperature",
		},
		{
			name:        "no reading type, with from only",
			deviceID:    "dev-1",
			limit:       10,
			readingType: "",
			from:        ptrTime(now),
			to:          nil,
			want:        fmt.Sprintf("sensor:dev-1:readings:10:%d", now.Unix()),
		},
		{
			name:        "no reading type, with to only",
			deviceID:    "dev-1",
			limit:       10,
			readingType: "",
			from:        nil,
			to:          ptrTime(later),
			want:        fmt.Sprintf("sensor:dev-1:readings:10:%d", later.Unix()),
		},
		{
			name:        "no reading type, with both time filters",
			deviceID:    "dev-1",
			limit:       10,
			readingType: "",
			from:        ptrTime(now),
			to:          ptrTime(later),
			want:        fmt.Sprintf("sensor:dev-1:readings:10:%d:%d", now.Unix(), later.Unix()),
		},
		{
			name:        "with reading type and both time filters",
			deviceID:    "dev-1",
			limit:       50,
			readingType: "humidity",
			from:        ptrTime(now),
			to:          ptrTime(later),
			want:        fmt.Sprintf("sensor:dev-1:readings:50:humidity:%d:%d", now.Unix(), later.Unix()),
		},
		{
			name:        "with reading type and from only",
			deviceID:    "dev-2",
			limit:       100,
			readingType: "pressure",
			from:        ptrTime(now),
			to:          nil,
			want:        fmt.Sprintf("sensor:dev-2:readings:100:pressure:%d", now.Unix()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.cacheKey(tt.deviceID, tt.limit, tt.readingType, tt.from, tt.to)
			if got != tt.want {
				t.Errorf("cacheKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: pkCacheKey
// ---------------------------------------------------------------------------

func TestPkCacheKey(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)

	tests := []struct {
		id   int64
		want string
	}{
		{1, "sensor:id:1"},
		{42, "sensor:id:42"},
		{999999999, "sensor:id:999999999"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("id_%d", tt.id), func(t *testing.T) {
			got := svc.pkCacheKey(tt.id)
			if got != tt.want {
				t.Errorf("pkCacheKey(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: GetSensorReadings
// ---------------------------------------------------------------------------

func TestGetSensorReadings_Validation(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		deviceID    string
		limit       int
		readingType string
		wantErr     error
	}{
		{"empty device ID", "", 10, "", ErrInvalidParameter},
		{"device ID too long", strings.Repeat("a", 51), 10, "", ErrInvalidParameter},
		{"device ID with space", "sensor 001", 10, "", ErrInvalidParameter},
		{"device ID with @", "sensor@001", 10, "", ErrInvalidParameter},
		{"limit zero", "sensor-1", 0, "", ErrInvalidParameter},
		{"limit negative", "sensor-1", -1, "", ErrInvalidParameter},
		{"limit 501", "sensor-1", 501, "", ErrInvalidParameter},
		{"limit 500 is ok", "sensor-1", 500, "", nil},
		{"limit 1 is ok", "sensor-1", 1, "", nil},
		{"invalid reading type hyphen", "sensor-1", 10, "temp-high", ErrInvalidParameter},
		{"invalid reading type space", "sensor-1", 10, "temp high", ErrInvalidParameter},
		{"invalid reading type underscore", "sensor-1", 10, "temp_high", ErrInvalidParameter},
		{"empty reading type is ok", "sensor-1", 10, "", nil},
		{"valid reading type", "sensor-1", 10, "temperature", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.GetSensorReadings(ctx, tt.deviceID, tt.limit, tt.readingType, nil, nil)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					// Validation-only test that expects no error: we still need a
					// repo that returns data, otherwise we get ErrDeviceNotFound.
					// Only check that the error is NOT ErrInvalidParameter.
					if errors.Is(err, ErrInvalidParameter) {
						t.Fatalf("did not expect ErrInvalidParameter, got %v", err)
					}
				}
			}
		})
	}
}

func TestGetSensorReadings_CacheHit(t *testing.T) {
	repo := &mockRepo{}
	mc := &mockCache{
		enabled: true,
		getFn: func(_ context.Context, key string, dest interface{}) error {
			// Simulate a cache hit by populating dest.
			readings, ok := dest.(*[]model.SensorReading)
			if !ok {
				return fmt.Errorf("key not found")
			}
			*readings = []model.SensorReading{
				{ID: "1", DeviceID: "sensor-1", ReadingType: "temperature", Value: 22.5},
			}
			return nil
		},
	}
	svc := newSvc(repo, mc)

	status, readings, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "HIT" {
		t.Errorf("status = %q, want %q", status, "HIT")
	}
	if len(readings) != 1 {
		t.Fatalf("len(readings) = %d, want 1", len(readings))
	}
	if readings[0].DeviceID != "sensor-1" {
		t.Errorf("readings[0].DeviceID = %q, want %q", readings[0].DeviceID, "sensor-1")
	}
}

func TestGetSensorReadings_CacheMiss_PopulatesCache(t *testing.T) {
	var setKey string
	repo := &mockRepo{
		queryFn: func(_ context.Context, deviceID string, _ int, _ string, _, _ *time.Time) ([]model.SensorReading, error) {
			return []model.SensorReading{
				{ID: "1", DeviceID: deviceID, ReadingType: "temperature", Value: 23.0},
			}, nil
		},
	}
	mc := &mockCache{
		enabled: true,
		getFn: func(_ context.Context, _ string, _ interface{}) error {
			return fmt.Errorf("key not found") // simulate miss
		},
		setFn: func(_ context.Context, key string, _ interface{}) error {
			setKey = key
			return nil
		},
	}
	svc := newSvc(repo, mc)

	status, readings, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MISS" {
		t.Errorf("status = %q, want %q", status, "MISS")
	}
	if len(readings) != 1 {
		t.Fatalf("len(readings) = %d, want 1", len(readings))
	}
	if setKey != "sensor:sensor-1:readings:10" {
		t.Errorf("cache set key = %q, want %q", setKey, "sensor:sensor-1:readings:10")
	}
}

func TestGetSensorReadings_NoTimeFilters_NoResults_DeviceNotFound(t *testing.T) {
	repo := &mockRepo{
		queryFn: func(_ context.Context, _ string, _ int, _ string, _, _ *time.Time) ([]model.SensorReading, error) {
			return nil, nil
		},
	}
	svc := newSvc(repo, nil)

	_, _, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestGetSensorReadings_WithTimeFilters_NoResults_EmptyArray(t *testing.T) {
	repo := &mockRepo{
		queryFn: func(_ context.Context, _ string, _ int, _ string, _, _ *time.Time) ([]model.SensorReading, error) {
			return nil, nil
		},
	}
	svc := newSvc(repo, nil)

	now := time.Now()
	status, readings, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", &now, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MISS" {
		t.Errorf("status = %q, want %q", status, "MISS")
	}
	if readings == nil {
		t.Fatal("readings is nil, expected empty non-nil slice")
	}
	if len(readings) != 0 {
		t.Errorf("len(readings) = %d, want 0", len(readings))
	}
}

func TestGetSensorReadings_CacheDisabled_NilCache(t *testing.T) {
	repo := &mockRepo{
		queryFn: func(_ context.Context, deviceID string, _ int, _ string, _, _ *time.Time) ([]model.SensorReading, error) {
			return []model.SensorReading{{ID: "1", DeviceID: deviceID}}, nil
		},
	}
	svc := newSvc(repo, nil) // nil cache

	status, readings, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MISS" {
		t.Errorf("status = %q, want %q", status, "MISS")
	}
	if len(readings) != 1 {
		t.Fatalf("len(readings) = %d, want 1", len(readings))
	}
}

func TestGetSensorReadings_RepoError(t *testing.T) {
	dbErr := errors.New("connection refused")
	repo := &mockRepo{
		queryFn: func(_ context.Context, _ string, _ int, _ string, _, _ *time.Time) ([]model.SensorReading, error) {
			return nil, dbErr
		},
	}
	svc := newSvc(repo, nil)

	_, _, err := svc.GetSensorReadings(context.Background(), "sensor-1", 10, "", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped db error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetSensorReadingByID
// ---------------------------------------------------------------------------

func TestGetSensorReadingByID_Validation(t *testing.T) {
	svc := newSvc(&mockRepo{}, nil)

	tests := []struct {
		name    string
		id      int64
		wantErr error
	}{
		{"zero id", 0, ErrInvalidParameter},
		{"negative id", -1, ErrInvalidParameter},
		{"id 1 is valid", 1, nil}, // will get ErrReadingNotFound but not ErrInvalidParameter
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.GetSensorReadingByID(context.Background(), tt.id)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestGetSensorReadingByID_CacheHit(t *testing.T) {
	repo := &mockRepo{}
	mc := &mockCache{
		enabled: true,
		getFn: func(_ context.Context, key string, dest interface{}) error {
			reading, ok := dest.(*model.SensorReading)
			if !ok {
				return fmt.Errorf("key not found")
			}
			reading.ID = "42"
			reading.DeviceID = "sensor-1"
			reading.Value = 99.9
			return nil
		},
	}
	svc := newSvc(repo, mc)

	status, reading, err := svc.GetSensorReadingByID(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "HIT" {
		t.Errorf("status = %q, want %q", status, "HIT")
	}
	if reading == nil {
		t.Fatal("reading is nil")
	}
	if reading.ID != "42" {
		t.Errorf("reading.ID = %q, want %q", reading.ID, "42")
	}
}

func TestGetSensorReadingByID_CacheMiss(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, id int64) (*model.SensorReading, error) {
			return &model.SensorReading{ID: "7", DeviceID: "sensor-7", Value: 1.0}, nil
		},
	}
	mc := &mockCache{
		enabled: true,
		getFn: func(_ context.Context, _ string, _ interface{}) error {
			return fmt.Errorf("key not found")
		},
	}
	svc := newSvc(repo, mc)

	status, reading, err := svc.GetSensorReadingByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MISS" {
		t.Errorf("status = %q, want %q", status, "MISS")
	}
	if reading == nil {
		t.Fatal("reading is nil")
	}
	if reading.ID != "7" {
		t.Errorf("reading.ID = %q, want %q", reading.ID, "7")
	}
}

func TestGetSensorReadingByID_NotFound(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ int64) (*model.SensorReading, error) {
			return nil, nil
		},
	}
	svc := newSvc(repo, nil)

	_, _, err := svc.GetSensorReadingByID(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrReadingNotFound) {
		t.Fatalf("expected ErrReadingNotFound, got %v", err)
	}
}

func TestGetSensorReadingByID_RepoError(t *testing.T) {
	dbErr := errors.New("connection refused")
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ int64) (*model.SensorReading, error) {
			return nil, dbErr
		},
	}
	svc := newSvc(repo, nil)

	_, _, err := svc.GetSensorReadingByID(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped db error, got %v", err)
	}
}

func TestGetSensorReadingByID_NilCache(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, id int64) (*model.SensorReading, error) {
			return &model.SensorReading{ID: "1", DeviceID: "dev", Value: 5.0}, nil
		},
	}
	svc := newSvc(repo, nil)

	status, reading, err := svc.GetSensorReadingByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MISS" {
		t.Errorf("status = %q, want %q", status, "MISS")
	}
	if reading == nil {
		t.Fatal("reading is nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetStats
// ---------------------------------------------------------------------------

func TestGetStats(t *testing.T) {
	expected := map[string]interface{}{
		"total_readings": int64(50000000),
		"device_count":   int64(120),
	}
	repo := &mockRepo{
		getStatsFromMVFn: func(_ context.Context) (map[string]interface{}, error) {
			return expected, nil
		},
	}
	svc := newSvc(repo, nil)

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats["total_readings"].(int64) != int64(50000000) {
		t.Errorf("total_readings = %v, want 50000000", stats["total_readings"])
	}
	if stats["device_count"].(int64) != int64(120) {
		t.Errorf("device_count = %v, want 120", stats["device_count"])
	}
}

func TestGetStats_RepoError(t *testing.T) {
	dbErr := errors.New("mv not refreshed")
	repo := &mockRepo{
		getStatsFromMVFn: func(_ context.Context) (map[string]interface{}, error) {
			return nil, dbErr
		},
	}
	svc := newSvc(repo, nil)

	_, err := svc.GetStats(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped db error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	repo := &mockRepo{}
	cfg := Config{CacheEnabled: true}
	svc := New(repo, nil, cfg)
	if svc == nil {
		t.Fatal("New returned nil")
	}
	if svc.repo == nil {
		t.Error("repo is nil")
	}
	if svc.cache != nil {
		t.Error("cache should be nil when nil is passed")
	}
}

// ---------------------------------------------------------------------------
// Tests: Ping
// ---------------------------------------------------------------------------

func TestPing(t *testing.T) {
	t.Run("all healthy", func(t *testing.T) {
		repo := &mockRepo{pingFn: func(_ context.Context) error { return nil }}
		mc := &mockCache{
			enabled: true,
			pingFn:  func(_ context.Context) error { return nil },
		}
		svc := newSvc(repo, mc)

		results := svc.Ping(context.Background())
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results["database"] != nil {
			t.Errorf("database error: %v", results["database"])
		}
		if results["cache"] != nil {
			t.Errorf("cache error: %v", results["cache"])
		}
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("db down")
		repo := &mockRepo{pingFn: func(_ context.Context) error { return dbErr }}
		svc := newSvc(repo, nil)

		results := svc.Ping(context.Background())
		if !errors.Is(results["database"], dbErr) {
			t.Errorf("database error = %v, want %v", results["database"], dbErr)
		}
	})

	t.Run("cache disabled nil error", func(t *testing.T) {
		repo := &mockRepo{pingFn: func(_ context.Context) error { return nil }}
		svc := newSvc(repo, nil)

		results := svc.Ping(context.Background())
		if results["cache"] != nil {
			t.Errorf("cache should be nil when disabled, got %v", results["cache"])
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: PingWithLatency
// ---------------------------------------------------------------------------

func TestPingWithLatency(t *testing.T) {
	repo := &mockRepo{pingFn: func(_ context.Context) error { return nil }}
	mc := &mockCache{
		enabled: true,
		pingFn:  func(_ context.Context) error { return nil },
	}
	svc := newSvc(repo, mc)

	results := svc.PingWithLatency(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["database"].Error != nil {
		t.Errorf("database error: %v", results["database"].Error)
	}
	if results["cache"].Error != nil {
		t.Errorf("cache error: %v", results["cache"].Error)
	}
	// Latency should be non-negative (>= 0 is fine, even 0 on fast machines).
	if results["database"].LatencyMs < 0 {
		t.Errorf("database latency = %d, want >= 0", results["database"].LatencyMs)
	}
}
