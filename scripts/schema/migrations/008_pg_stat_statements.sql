-- Migration 008: Enable pg_stat_statements for query performance tracking
--
-- Records execution statistics for all SQL queries: total time, calls,
-- rows returned, cache hit/miss ratio per query. Essential for identifying
-- slow queries and proving database performance is hardware-limited.
--
-- Prerequisite: shared_preload_libraries=pg_stat_statements must be set
-- in PostgreSQL config (already added to docker-compose.yml postgres command).
--
-- Overhead: <1% CPU. Read-only observability extension — does not change
-- query behavior or results.

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
