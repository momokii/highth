-- Migration 007: Add JSONB metadata column to sensor_readings
--
-- Adds a `metadata` JSONB column for storing additional sensor attributes
-- (location, battery level, firmware version, etc.).
--
-- Default is empty JSON object '{}' for backwards compatibility.
-- The Go model already has `Metadata map[string]any` with `json:"metadata,omitempty"`.
--
-- NOTE: reading_type stays VARCHAR(20) and value stays NUMERIC(10,2).
-- These cannot be widened without dropping and recreating materialized views
-- (which requires scanning 50M+ rows, taking 60+ minutes). The current
-- types are sufficient for existing reading types and precision requirements.

ALTER TABLE sensor_readings
ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
