#!/bin/bash
# Automated Materialized View Refresh Script
#
# This script refreshes PostgreSQL materialized views with the appropriate strategy:
# - Hourly stats: CONCURRENTLY (every 15 minutes recommended)
# - Daily stats: Full refresh (once daily recommended)
# - Global stats: CONCURRENTLY (every 5 minutes recommended)
#
# Usage: ./scripts/refresh_materialized_views.sh [hourly|daily|global|all]
#
# Examples:
#   ./scripts/refresh_materialized_views.sh hourly   # Refresh hourly stats only
#   ./scripts/refresh_materialized_views.sh daily    # Refresh daily stats only
#   ./scripts/refresh_materialized_views.sh all      # Refresh all views
#
# Cron Examples:
#   */15 * * * * /path/to/scripts/refresh_materialized_views.sh hourly
#   0 2 * * * /path/to/scripts/refresh_materialized_views.sh daily
#   */5 * * * * /path/to/scripts/refresh_materialized_views.sh global
#
# Author: Higth Optimization Team
# Date: 2026-03-15
# Version: 1.0.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DB_CONTAINER="${DB_CONTAINER:-highth-postgres}"
DB_USER="${DB_USER:-sensor_user}"
DB_NAME="${DB_NAME:-sensor_db}"
REFRESH_TYPE="${1:-all}"

# Function to log messages
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to refresh materialized view
refresh_mv() {
    local mv_name=$1
    local concurrently=$2

    log_info "Refreshing materialized view: $mv_name"

    local start_time=$(date +%s)

    if [ "$concurrently" = "true" ]; then
        docker exec "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c \
            "REFRESH MATERIALIZED VIEW CONCURRENTLY ${mv_name};" > /dev/null 2>&1
    else
        docker exec "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c \
            "REFRESH MATERIALIZED VIEW ${mv_name};" > /dev/null 2>&1
    fi

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    if [ $? -eq 0 ]; then
        log_success "Refreshed $mv_name (${duration}s)"
    else
        log_error "Failed to refresh $mv_name"
        return 1
    fi
}

# Function to get materialized view info
get_mv_info() {
    local mv_name=$1

    docker exec "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -t -c "
    SELECT
        pg_size_pretty(pg_relation_size('${mv_name}'::regclass)) as size,
        (SELECT count(*) FROM ${mv_name}) as row_count
    "
}

# Main script
echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         Materialized View Refresh Script                        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

log_info "Refresh type: ${REFRESH_TYPE}"
log_info "Database: ${DB_NAME}"
log_info "Started at: $(date)"
echo ""

# Track total time
script_start_time=$(date +%s)
total_views_refreshed=0
failed_views=()

# Refresh hourly stats
if [ "$REFRESH_TYPE" = "hourly" ] || [ "$REFRESH_TYPE" = "all" ]; then
    log_info "Refreshing hourly statistics..."

    mv_info=$(get_mv_info "mv_device_hourly_stats")
    log_info "Current size: $(echo $mv_info | cut -d'|' -f1 | xargs)"
    log_info "Current rows: $(echo $mv_info | cut -d'|' -f2 | xargs)"

    if refresh_mv "mv_device_hourly_stats" "true"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_device_hourly_stats")
    fi
    echo ""
fi

# Refresh daily stats
if [ "$REFRESH_TYPE" = "daily" ] || [ "$REFRESH_TYPE" = "all" ]; then
    log_info "Refreshing daily statistics..."

    mv_info=$(get_mv_info "mv_device_daily_stats")
    log_info "Current size: $(echo $mv_info | cut -d'|' -f1 | xargs)"
    log_info "Current rows: $(echo $mv_info | cut -d'|' -f2 | xargs)"

    if refresh_mv "mv_device_daily_stats" "false"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_device_daily_stats")
    fi
    echo ""
fi

# Refresh global stats
if [ "$REFRESH_TYPE" = "global" ] || [ "$REFRESH_TYPE" = "all" ]; then
    log_info "Refreshing global statistics..."

    mv_info=$(get_mv_info "mv_global_stats")
    log_info "Current size: $(echo $mv_info | cut -d'|' -f1 | xargs)"
    log_info "Current rows: $(echo $mv_info | cut -d'|' -f2 | xargs)"

    if refresh_mv "mv_global_stats" "true"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_global_stats")
    fi
    echo ""
fi

# Calculate total time
script_end_time=$(date +%s)
total_duration=$((script_end_time - script_start_time))

# Summary
echo -e "${BLUE}─────────────────────────────────────────${NC}"
echo -e "${BLUE}Refresh Summary${NC}"
echo -e "${BLUE}─────────────────────────────────────────${NC}"
echo "Views refreshed: ${total_views_refreshed}"
echo "Total time: ${total_duration}s"
echo "Completed at: $(date)"

# Handle failures
if [ ${#failed_views[@]} -gt 0 ]; then
    log_error "Failed to refresh the following views:"
    for view in "${failed_views[@]}"; do
        log_error "  - ${view}"
    done
    exit 1
fi

echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║              Refresh Complete ✓                                 ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"

exit 0
