// Package model defines data structures for the IoT sensor query system.
package model

import "time"

// ReadingType represents the type of sensor reading.
type ReadingType string

const (
	// ReadingTypeTemperature represents a temperature reading
	ReadingTypeTemperature ReadingType = "temperature"
	// ReadingTypeHumidity represents a humidity reading
	ReadingTypeHumidity ReadingType = "humidity"
	// ReadingTypePressure represents a pressure reading
	ReadingTypePressure ReadingType = "pressure"
)

// SensorReading represents a single sensor reading from the database.
type SensorReading struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	ReadingType string    `json:"reading_type"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// SensorReadingFilter represents query parameters for fetching sensor readings.
type SensorReadingFilter struct {
	DeviceID    string
	ReadingType string
	Limit       int
}

// NewSensorReading creates a new SensorReading with the given parameters.
func NewSensorReading(deviceID string, timestamp time.Time, readingType ReadingType, value float64, unit string) SensorReading {
	return SensorReading{
		DeviceID:    deviceID,
		Timestamp:   timestamp,
		ReadingType: string(readingType),
		Value:       value,
		Unit:        unit,
	}
}
