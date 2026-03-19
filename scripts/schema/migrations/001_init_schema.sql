-- Migration 001: Base Schema Initialization
-- Creates the sensor_readings table with basic structure

CREATE TABLE IF NOT EXISTS sensor_readings (
    id BIGSERIAL PRIMARY KEY,
    device_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    reading_type VARCHAR(20) NOT NULL CHECK (reading_type IN ('temperature', 'humidity', 'pressure')),
    value DECIMAL(10,2) NOT NULL,
    unit VARCHAR(20) NOT NULL
);

-- Basic indexes
CREATE INDEX IF NOT EXISTS idx_sensor_readings_device_timestamp
ON sensor_readings (device_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_sensor_readings_reading_type
ON sensor_readings (reading_type);

COMMENT ON TABLE sensor_readings IS 'IoT sensor readings table';
