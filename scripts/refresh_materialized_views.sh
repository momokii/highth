#!/bin/bash
# Materialized View Refresh Script
#
# Refreshes PostgreSQL materialized views with CONCURRENTLY (non-blocking).
# Each refresh performs a FULL table scan of sensor_readings.
#
# The script detects your row count and shows estimated time before starting.
# You can cancel with Ctrl+C during the countdown if the estimate is too long.
#
# Usage: ./scripts/refresh_materialized_views.sh [hourly|daily|global|all] [options]
#
# Options:
#   --status    Show current MV sizes without refreshing
#   --yes, -y   Skip the confirmation countdown (for cron/automation)
#   --help, -h  Show help with timing table
#
# Cron Examples (production):
#   0 */6 * * * /path/to/scripts/refresh_materialized_views.sh global -y
#   0 2 * * * /path/to/scripts/refresh_materialized_views.sh hourly -y
#   0 3 * * 0 /path/to/scripts/refresh_materialized_views.sh daily -y

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
DB_CONTAINER="${DB_CONTAINER:-highth-postgres}"
DB_USER="${DB_USER:-sensor_user}"
DB_NAME="${DB_NAME:-sensor_db}"

# Help function
show_help() {
    echo -e "${BLUE}Higth — Materialized View Refresh Script${NC}"
    echo ""
    echo "Refreshes PostgreSQL materialized views with CONCURRENTLY (non-blocking)."
    echo "Each refresh performs a FULL table scan of sensor_readings — the time"
    echo "depends on your total row count, not the materialized view size."
    echo ""
    echo -e "${BLUE}Usage:${NC}"
    echo "  ./scripts/refresh_materialized_views.sh [TYPE] [OPTION]"
    echo ""
    echo -e "${BLUE}Types:${NC}"
    echo "  all       Refresh all 3 views (default)"
    echo "  global    Refresh mv_global_stats only (total readings, device count)"
    echo "  hourly    Refresh mv_device_hourly_stats only (avg/min/max per hour)"
    echo "  daily     Refresh mv_device_daily_stats only (percentiles per day)"
    echo ""
    echo -e "${BLUE}Options:${NC}"
    echo "  --status  Show current MV sizes and row counts without refreshing"
    echo "  --yes, -y Skip confirmation countdown (for cron/automation)"
    echo "  --help    Show this help message"
    echo ""
    echo -e "${BLUE}Timing expectations (approximate):${NC}"
    echo "  Dataset    | global   | hourly     | daily      | all"
    echo "  -----------|----------|------------|------------|---------------"
    echo "  1M rows    | <1s      | ~2s        | ~1s        | ~3s"
    echo "  10M rows   | ~3s      | ~15s       | ~10s       | ~30s"
    echo "  50M rows   | ~1-2min  | ~3-5min    | ~2-3min    | ~8-12min"
    echo "  100M rows  | ~2-4min  | ~5-10min   | ~3-5min    | ~12-20min"
    echo "  200M rows  | ~4-8min  | ~10-20min  | ~6-12min   | ~25-45min"
    echo "  300M rows  | ~5-15min | ~15-30min  | ~10-20min  | ~30-60+min"
    echo ""
    echo -e "${BLUE}What each view powers:${NC}"
    echo "  global  -> /api/v1/stats (total_readings, total_devices)"
    echo "  hourly  -> /api/v1/stats (per-hour aggregation trends)"
    echo "  daily   -> /api/v1/stats (daily p50/p95/p99 percentiles)"
    echo ""
    echo -e "${BLUE}Important:${NC}"
    echo "  - Only the /api/v1/stats endpoint uses MVs. The main sensor-readings"
    echo "    endpoint (/api/v1/sensor-readings) queries the base table with indexes."
    echo "  - For quick testing, you can skip the refresh entirely — only stats"
    echo "    will show stale data."
    echo "  - The script shows a countdown with estimated time before starting."
    echo "    Press Ctrl+C during the countdown to cancel."
    echo ""
    echo -e "${BLUE}Production cron schedule:${NC}"
    echo "  0 */6 * * *   .../refresh_materialized_views.sh global -y"
    echo "  0 2 * * *     .../refresh_materialized_views.sh hourly -y"
    echo "  0 3 * * 0     .../refresh_materialized_views.sh daily -y"
    echo ""
    echo -e "${BLUE}Examples:${NC}"
    echo "  ./scripts/refresh_materialized_views.sh              # Refresh all (with warning)"
    echo "  ./scripts/refresh_materialized_views.sh global       # Quick stat update"
    echo "  ./scripts/refresh_materialized_views.sh all -y       # Skip countdown (cron)"
    echo "  ./scripts/refresh_materialized_views.sh --status     # Check MV sizes"
}

# Parse arguments
REFRESH_TYPE="all"
SHOW_STATUS=false
SHOW_HELP=false
SKIP_CONFIRM=false

for arg in "$@"; do
    case $arg in
        hourly|daily|global|all)
            REFRESH_TYPE="$arg"
            ;;
        --status)
            SHOW_STATUS=true
            ;;
        --help|-h)
            SHOW_HELP=true
            ;;
        --yes|-y)
            SKIP_CONFIRM=true
            ;;
    esac
done

if [ "$SHOW_HELP" = true ]; then
    show_help
    exit 0
fi

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

run_sql() {
    docker exec "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "$1"
}

run_sql_quiet() {
    docker exec "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "$1" > /dev/null
}

# Estimate refresh time based on row count (in millions)
estimate_time() {
    local rows_m=$1
    local type=$2

    if [ "$rows_m" -le 1 ]; then
        case $type in
            global) echo "<1s" ;;
            hourly) echo "~2s" ;;
            daily)  echo "~1s" ;;
            all)    echo "~3s" ;;
        esac
    elif [ "$rows_m" -le 10 ]; then
        case $type in
            global) echo "~3s" ;;
            hourly) echo "~15s" ;;
            daily)  echo "~10s" ;;
            all)    echo "~30s" ;;
        esac
    elif [ "$rows_m" -le 50 ]; then
        case $type in
            global) echo "~1-2min" ;;
            hourly) echo "~3-5min" ;;
            daily)  echo "~2-3min" ;;
            all)    echo "~8-12min" ;;
        esac
    elif [ "$rows_m" -le 100 ]; then
        case $type in
            global) echo "~2-4min" ;;
            hourly) echo "~5-10min" ;;
            daily)  echo "~3-5min" ;;
            all)    echo "~12-20min" ;;
        esac
    elif [ "$rows_m" -le 200 ]; then
        case $type in
            global) echo "~4-8min" ;;
            hourly) echo "~10-20min" ;;
            daily)  echo "~6-12min" ;;
            all)    echo "~25-45min" ;;
        esac
    else
        case $type in
            global) echo "~5-15min" ;;
            hourly) echo "~15-30min" ;;
            daily)  echo "~10-20min" ;;
            all)    echo "~30-60+min" ;;
        esac
    fi
}

# Show status of all materialized views
show_status() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║         Materialized View Status                                 ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    for mv in mv_device_hourly_stats mv_device_daily_stats mv_global_stats; do
        local info=$(run_sql "
            SELECT pg_size_pretty(pg_relation_size('${mv}'::regclass)) || ' | ' ||
                   (SELECT count(*) FROM ${mv}) || ' rows'
        " | xargs)
        log_info "${mv}: ${info:-not found}"
    done
    echo ""

    local row_count=$(run_sql "SELECT reltuples::bigint FROM pg_class WHERE relname = 'sensor_readings';" | xargs)
    if [ -n "$row_count" ] && [ "$row_count" -gt 0 ]; then
        local rows_m=$((row_count / 1000000))
        log_info "sensor_readings table: ~${rows_m}M rows (estimated)"
    fi
}

if [ "$SHOW_STATUS" = true ]; then
    show_status
    exit 0
fi

# ── Main ──────────────────────────────────────────────────────────────

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         Materialized View Refresh Script                         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Detect row count
log_info "Detecting row count..."
ROW_COUNT=$(run_sql "SELECT reltuples::bigint FROM pg_class WHERE relname = 'sensor_readings';" | xargs)

if [ -z "$ROW_COUNT" ] || [ "$ROW_COUNT" -eq 0 ] 2>/dev/null; then
    log_warning "Could not detect row count. Proceeding without time estimate."
    ROW_COUNT=0
    ROWS_M=0
    ESTIMATE="unknown"
else
    ROWS_M=$((ROW_COUNT / 1000000))
    ESTIMATE=$(estimate_time "$ROWS_M" "$REFRESH_TYPE")
    log_info "sensor_readings: ~${ROWS_M}M rows (${ROW_COUNT})"
fi

log_info "Refresh type: ${REFRESH_TYPE}"
log_info "Estimated time: ${ESTIMATE}"
echo ""

# Show warning and countdown for large datasets (skip with --yes)
if [ "$SKIP_CONFIRM" = false ] && [ "$ROWS_M" -ge 10 ]; then
    log_warning "This will scan all ${ROWS_M}M rows in sensor_readings."
    log_warning "Estimated time: ${ESTIMATE}"
    echo ""
    log_warning "Press Ctrl+C to cancel, or wait to continue..."

    for i in 5 4 3 2 1; do
        echo -ne "\r  Starting in ${i}s..."
        sleep 1
    done
    echo -e "\r  Starting...      "
    echo ""
fi

log_info "Database: ${DB_NAME}"
log_info "Started at: $(date)"
echo ""

script_start_time=$(date +%s)
total_views_refreshed=0
failed_views=()

# Function to refresh a materialized view
refresh_mv() {
    local mv_name=$1
    local start_time=$(date +%s)

    log_info "Refreshing ${mv_name} (CONCURRENTLY — non-blocking reads)..."
    run_sql_quiet "REFRESH MATERIALIZED VIEW CONCURRENTLY ${mv_name};"

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    log_success "Refreshed ${mv_name} (${duration}s)"
}

# Refresh hourly stats
if [ "$REFRESH_TYPE" = "hourly" ] || [ "$REFRESH_TYPE" = "all" ]; then
    mv_info=$(run_sql "SELECT pg_size_pretty(pg_relation_size('mv_device_hourly_stats'::regclass)) || ' | ' || (SELECT count(*) FROM mv_device_hourly_stats) || ' rows';" | xargs)
    log_info "Hourly stats: ${mv_info}"

    if refresh_mv "mv_device_hourly_stats"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_device_hourly_stats")
    fi
    echo ""
fi

# Refresh daily stats
if [ "$REFRESH_TYPE" = "daily" ] || [ "$REFRESH_TYPE" = "all" ]; then
    mv_info=$(run_sql "SELECT pg_size_pretty(pg_relation_size('mv_device_daily_stats'::regclass)) || ' | ' || (SELECT count(*) FROM mv_device_daily_stats) || ' rows';" | xargs)
    log_info "Daily stats: ${mv_info}"

    if refresh_mv "mv_device_daily_stats"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_device_daily_stats")
    fi
    echo ""
fi

# Refresh global stats
if [ "$REFRESH_TYPE" = "global" ] || [ "$REFRESH_TYPE" = "all" ]; then
    mv_info=$(run_sql "SELECT pg_size_pretty(pg_relation_size('mv_global_stats'::regclass)) || ' | ' || (SELECT count(*) FROM mv_global_stats) || ' rows';" | xargs)
    log_info "Global stats: ${mv_info}"

    if refresh_mv "mv_global_stats"; then
        total_views_refreshed=$((total_views_refreshed + 1))
    else
        failed_views+=("mv_global_stats")
    fi
    echo ""
fi

# Summary
script_end_time=$(date +%s)
total_duration=$((script_end_time - script_start_time))

echo -e "${BLUE}─────────────────────────────────────────${NC}"
echo -e "${BLUE}Refresh Summary${NC}"
echo -e "${BLUE}─────────────────────────────────────────${NC}"
echo "Dataset:        ~${ROWS_M}M rows"
echo "Views refreshed: ${total_views_refreshed}"
echo "Total time:      ${total_duration}s"
echo "Completed at:    $(date)"

if [ ${#failed_views[@]} -gt 0 ]; then
    log_error "Failed to refresh:"
    for view in "${failed_views[@]}"; do
        log_error "  - ${view}"
    done
    exit 1
fi

echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║              Refresh Complete ✓                                 ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"

exit 0
