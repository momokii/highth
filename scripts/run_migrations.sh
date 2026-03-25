#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

DB_CONTAINER="highth-postgres"
DB_USER="sensor_user"
DB_NAME="sensor_db"
MIGRATION_DIR=""
DRY_RUN=false
VERBOSE=false
FORCE=false

MIGRATION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/schema/migrations" && pwd)"

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run) DRY_RUN=true; shift ;;
        --verbose) VERBOSE=true; shift ;;
        --force) FORCE=true; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "  --dry-run    Show what would be done"
            echo "  --verbose    Show detailed output"
            echo "  --force      Force re-run"
            exit 0
            ;;
        *) echo -e "${RED}[ERROR]${NC} Unknown option: $1"; exit 1 ;;
    esac
done

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_verbose() { [ "$VERBOSE" = true ] && echo -e "${CYAN}[VERBOSE]${NC} $1"; }

trim() { echo "$1" | tr -d '[:space:]'; }

db_query() {
    docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "$1" 2>/dev/null || true
}

db_execute_file() {
    docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" < "$1" 2>/dev/null
}

check_db_connection() {
    log_verbose "Checking database connection..."
    local result
    result=$(db_query "SELECT 1;")
    result=$(trim "$result")
    [ "$result" = "1" ]
}

init_schema_migrations() {
    log_verbose "Initializing schema_migrations table..."
    local check_table
    check_table=$(db_query "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'schema_migrations');")
    check_table=$(trim "$check_table")

    if [ "$check_table" = "f" ]; then
        log_info "Creating schema_migrations table..."
        db_query "CREATE TABLE schema_migrations (version VARCHAR(14) PRIMARY KEY, name VARCHAR(255) NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), execution_time_ms INTEGER, checksum VARCHAR(64));" > /dev/null
        log_success "Created schema_migrations table"
    else
        log_verbose "schema_migrations table already exists"
    fi
}

get_migration_files() {
    find "$MIGRATION_DIR" -maxdepth 1 -name "*.sql" -type f | sort -V
}

get_migration_version() {
    basename "$1" | sed 's/^\([0-9]*\)_.*/\1/'
}

get_migration_name() {
    basename "$1" | sed 's/^[0-9]*_\(.*\)\.sql/\1/'
}

is_migration_applied() {
    local version="$1"
    local result
    result=$(db_query "SELECT COUNT(*) FROM schema_migrations WHERE version = '$version';")
    result=$(trim "$result")
    [ "$result" = "1" ]
}

record_migration() {
    db_query "INSERT INTO schema_migrations (version, name, checksum, execution_time_ms) VALUES ('$1', '$2', '$3', $4);" > /dev/null
}

backfill_existing_migrations() {
    log_verbose "Checking for existing migrations to backfill..."

    for entry in "001:init_schema" "002:advanced_indexes" "004:materialized_views" "005:incremental_mv_refresh"; do
        local version=$(echo "$entry" | cut -d: -f1)
        local name=$(echo "$entry" | cut -d: -f2)

        is_migration_applied "$version" && continue

        local exists="f"
        case $version in
            001) exists=$(db_query "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sensor_readings');") ;;
            002) exists=$(db_query "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sensor_readings_timestamp_brin');") ;;
            004) exists=$(db_query "SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = 'mv_global_stats');") ;;
            005) exists=$(db_query "SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'refresh_hourly_stats_incremental');") ;;
        esac
        exists=$(trim "$exists")

        if [ "$exists" = "t" ]; then
            log_info "Backfilling migration $version..."
            db_query "INSERT INTO schema_migrations (version, name, applied_at) VALUES ('$version', '$name', NOW());" > /dev/null
            log_success "Backfilled migration $version"
        fi
    done
}

apply_migration() {
    local file="$1"
    local version=$(get_migration_version "$file")
    local name=$(get_migration_name "$file")

    log_info "Applying migration: ${version}_${name}.sql"

    if [ "$DRY_RUN" = true ]; then
        log_warning "[DRY RUN] Would apply: $file"
        return 0
    fi

    local start_time=$(date +%s%3N)
    db_execute_file "$file"
    local exit_code=$?
    local end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))

    if [ $exit_code -eq 0 ]; then
        local checksum=$(sha256sum "$file" | awk '{print $1}')
        record_migration "$version" "$name" "$checksum" "$duration"
        log_success "Applied migration ${version}_${name}.sql (${duration}ms)"
        return 0
    else
        log_error "Failed to apply migration ${version}_${name}.sql"
        return 1
    fi
}

validate_migration() {
    local file="$1"
    local version=$(get_migration_version "$file")

    if is_migration_applied "$version"; then
        if [ "$FORCE" = true ]; then
            log_warning "Migration $version already applied, forcing re-run..."
            return 0
        fi
        log_verbose "Migration $version already applied, skipping"
        return 1
    fi
    return 0
}

main() {
    printf '%b' "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}\n"
    printf '%b' "${BLUE}║            Higth Database Migration Runner                      ║${NC}\n"
    printf '%b' "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}\n"
    printf '\n'

    [ "$DRY_RUN" = true ] && log_warning "Running in DRY RUN mode"

    if ! check_db_connection; then
        log_error "Database connection failed"
        exit 1
    fi

    log_verbose "Database: $DB_NAME, Container: $DB_CONTAINER"
    printf '\n'

    init_schema_migrations
    backfill_existing_migrations
    printf '\n'

    local migration_files
    mapfile -t migration_files < <(get_migration_files)

    if [ ${#migration_files[@]} -eq 0 ]; then
        log_warning "No migration files found"
        exit 0
    fi

    log_info "Found ${#migration_files[@]} migration file(s)"
    printf '\n'

    local applied_count=0 skipped_count=0 total_count=0

    for file in "${migration_files[@]}"; do
        ((total_count++))
        local version=$(get_migration_version "$file")

        [ "$version" = "000" ] && continue

        if ! validate_migration "$file"; then
            ((skipped_count++))
            continue
        fi

        if apply_migration "$file"; then
            ((applied_count++))
        else
            log_error "Migration failed"
            exit 1
        fi
        printf '\n'
    done

    printf '%b' "${BLUE}─────────────────────────────────────────${NC}\n"
    printf '%b' "${BLUE}Migration Summary${NC}\n"
    printf '%b' "${BLUE}─────────────────────────────────────────${NC}\n"
    printf 'Total: %d, Applied: %d, Skipped: %d\n' "$total_count" "$applied_count" "$skipped_count"
    printf '\n'

    if [ $applied_count -eq 0 ] && [ "$DRY_RUN" != true ]; then
        log_success "All migrations are up to date!"
    elif [ $applied_count -gt 0 ]; then
        log_success "Applied ${applied_count} migration(s)"
    fi

    printf '%b' "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}\n"
    printf '%b' "${GREEN}║              Migration Run Complete ✓                         ║${NC}\n"
    printf '%b' "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"
}

main
