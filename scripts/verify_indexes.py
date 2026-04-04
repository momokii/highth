#!/usr/bin/env python3
"""
PostgreSQL Index Verification Script for Higth

Verifies that all expected indexes on the sensor_readings table exist
and have the correct structure (type, columns, INCLUDE clauses).

This script serves two purposes:
1. Standalone CLI tool for manual index verification
2. Importable module for programmatic verification (e.g., from generate_data_bulk.py)

Expected indexes are defined based on the migration files:
- Migration 001: Base table + 2 initial indexes
- Migration 002: BRIN index + composite index
- Migration 006: Covering index with INCLUDE clause

Usage:
    python3 verify_indexes.py                    # Use default DATABASE_URL
    python3 verify_indexes.py --db-url "..."    # Custom connection string
    python3 verify_indexes.py --verbose         # Show detailed comparison

Exit codes:
    0: All indexes verified successfully
    1: One or more indexes missing or incorrect
"""

import argparse
import os
import sys
import re

import psycopg2


# =============================================================================
# EXPECTED INDEX DEFINITIONS
# =============================================================================
# These define the exact index structure expected based on migrations.
# All indexes are on the 'sensor_readings' table in the 'public' schema.

EXPECTED_INDEXES = [
    {
        # Migration 001 - PRIMARY KEY
        "name": "sensor_readings_pkey",
        "type": "btree",
        "columns": "id",
        "description": "Primary key on id column (BIGSERIAL)"
    },
    {
        # Migration 001 - Basic index for device queries
        "name": "idx_sensor_readings_device_timestamp",
        "type": "btree",
        "columns": "device_id, timestamp DESC",
        "description": "Composite index for device-time queries"
    },
    {
        # Migration 001 - Basic index for reading type filter
        "name": "idx_sensor_readings_reading_type",
        "type": "btree",
        "columns": "reading_type",
        "description": "Index for reading type filter"
    },
    {
        # Migration 002 - BRIN index for time-series data
        # BRIN indexes are 99% smaller than B-tree for append-only time-series
        "name": "idx_sensor_readings_timestamp_brin",
        "type": "brin",
        "columns": "timestamp",
        "description": "BRIN index for timestamp (very compact for time-series)"
    },
    {
        # Migration 002 - Composite index covering most query patterns
        "name": "idx_sensor_readings_device_type_timestamp",
        "type": "btree",
        "columns": "device_id, reading_type, timestamp DESC",
        "description": "Composite index for device+type+time queries"
    },
    {
        # Migration 006 - Covering index with INCLUDE clause
        # Enables index-only scans (no heap access needed)
        "name": "idx_sensor_readings_device_covering",
        "type": "btree",
        "columns": "device_id, timestamp DESC",
        "include_columns": "reading_type, value, unit",
        "description": "Covering index with INCLUDE for index-only scans (PG 12+)"
    },
]


# =============================================================================
# VERIFICATION FUNCTIONS
# =============================================================================

def get_actual_indexes(cursor):
    """
    Query PostgreSQL for all indexes on sensor_readings.

    Returns a dict mapping index_name -> {
        'exists': bool,
        'type': str,  # 'btree' or 'brin'
        'columns': str,  # Column definition from pg_get_indexdef
        'include_columns': str or None,  # INCLUDE clause if present
        'def': str,  # Full index definition
    }
    """
    cursor.execute("""
        SELECT
            i.indexname,
            i.indexdef,
            am.amname as index_type,
            pg_get_indexdef(c.oid) as full_def
        FROM pg_indexes i
        JOIN pg_class c ON c.relname = i.indexname
        JOIN pg_am am ON am.oid = c.relam
        WHERE i.tablename = 'sensor_readings'
        AND i.schemaname = 'public'
        ORDER BY i.indexname
    """)

    indexes = {}
    for row in cursor.fetchall():
        index_name, indexdef, index_type, full_def = row

        # Parse columns from the index definition
        # Find the column list by looking for the last '(' before any ')' or 'INCLUDE'
        match = re.search(r'\(([^)]+(?:\([^)]+\)[^)]*)*)\)', full_def)
        columns = match.group(1) if match else ""

        # Check for INCLUDE clause
        include_match = re.search(r'INCLUDE\s*\(([^)]+)\)', full_def, re.IGNORECASE)
        include_columns = include_match.group(1) if include_match else None

        indexes[index_name] = {
            'exists': True,
            'type': index_type,
            'columns': columns.strip(),
            'include_columns': include_columns,
            'def': full_def
        }

    # Mark expected indexes as not existing if they weren't found
    for expected in EXPECTED_INDEXES:
        if expected['name'] not in indexes:
            indexes[expected['name']] = {
                'exists': False,
                'type': None,
                'columns': None,
                'include_columns': None,
                'def': None
            }

    return indexes


def compare_indexes(actual_indexes):
    """
    Compare actual indexes against expected definitions.

    Returns a list of result dicts, one per expected index.
    """
    results = []

    for expected in EXPECTED_INDEXES:
        name = expected['name']
        actual = actual_indexes.get(name, {'exists': False})

        exists = actual['exists']
        type_ok = actual['type'] == expected['type'] if exists else False

        # Compare columns - normalize whitespace and remove quotes
        # PostgreSQL quotes reserved words like "timestamp" in pg_get_indexdef()
        expected_cols = ' '.join(expected['columns'].split()).replace('"', '')
        actual_cols = ' '.join(actual['columns'].split()).replace('"', '') if actual['columns'] else None
        columns_ok = actual_cols == expected_cols if exists and actual_cols else False

        # Compare INCLUDE columns - normalize whitespace and remove quotes
        expected_include = expected.get('include_columns')
        actual_include = actual.get('include_columns')
        if expected_include:
            expected_inc = ' '.join(expected_include.split()).replace('"', '')
            actual_inc = ' '.join(actual_include.split()).replace('"', '') if actual_include else None
            include_ok = actual_inc == expected_inc
        else:
            include_ok = actual_include is None

        # Overall OK status
        ok = exists and type_ok and columns_ok and include_ok

        results.append({
            'name': name,
            'expected_type': expected['type'],
            'actual_type': actual['type'] if exists else None,
            'expected_columns': expected['columns'],
            'actual_columns': actual_cols if exists else None,
            'expected_include': expected_include,
            'actual_include': actual_include,
            'exists': exists,
            'type_ok': type_ok,
            'columns_ok': columns_ok,
            'include_ok': include_ok,
            'ok': ok,
            'description': expected['description']
        })

    return results


def format_results_table(results, verbose=False):
    """Format verification results as a readable table."""
    output = []

    # Header
    output.append("=" * 78)
    output.append("Index Verification Report")
    output.append("=" * 78)
    output.append("")

    # Table header
    output.append(f"{'#':<3} {'Index Name':<50} {'Type':<8} {'OK':<3}")
    output.append("-" * 78)

    # Results
    for i, result in enumerate(results, 1):
        ok_status = "OK" if result['ok'] else "FAIL"
        type_str = result['actual_type'] if result['exists'] else "MISSING"

        output.append(f"{i:<3} {result['name']:<50} {type_str:<8} {ok_status:<3}")

        if verbose or not result['ok']:
            if not result['exists']:
                output.append(f"     Status: INDEX MISSING")
            elif not result['type_ok']:
                output.append(f"     Expected type: {result['expected_type']}, got: {result['actual_type']}")
            elif not result['columns_ok']:
                output.append(f"     Expected columns: {result['expected_columns']}")
                output.append(f"     Got columns:      {result['actual_columns']}")
            elif not result['include_ok']:
                output.append(f"     Expected INCLUDE: {result['expected_include']}")
                output.append(f"     Got INCLUDE:      {result['actual_include']}")

    # Summary
    output.append("-" * 78)
    passed_count = sum(1 for r in results if r['ok'])
    total_count = len(results)

    if passed_count == total_count:
        output.append(f"Results: {passed_count}/{total_count} indexes verified   [ALL PASSED]")
    else:
        output.append(f"Results: {passed_count}/{total_count} indexes verified   [SOME FAILED]")
    output.append("=" * 78)

    return "\n".join(output)


def verify_indexes(db_url=None, verbose=False):
    """
    Verify all expected indexes exist with correct structure.

    Args:
        db_url: PostgreSQL connection URL. If None, uses DATABASE_URL env var or default.
        verbose: If True, show detailed comparison for each index.

    Returns:
        tuple: (passed: bool, results: list[dict])
        - passed: True if all indexes verified successfully
        - results: List of result dicts (one per expected index)

    Raises:
        psycopg2.Error: Database connection or query error
    """
    # Default database URL
    if db_url is None:
        db_url = os.getenv("DATABASE_URL", "postgres://sensor_user:sensor_password@localhost:5434/sensor_db")

    # Connect to database
    conn = psycopg2.connect(db_url)
    conn.autocommit = False
    cursor = conn.cursor()

    try:
        # Get actual indexes from database
        actual_indexes = get_actual_indexes(cursor)

        # Compare with expected
        results = compare_indexes(actual_indexes)

        # Print formatted report
        print(format_results_table(results, verbose=verbose))

        # Check if all passed
        passed = all(r['ok'] for r in results)

        return passed, results

    finally:
        cursor.close()
        conn.close()


# =============================================================================
# CLI ENTRY POINT
# =============================================================================

def main():
    """Command-line interface entry point."""
    parser = argparse.ArgumentParser(
        description='Verify PostgreSQL indexes for Higth sensor_readings table',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s                              Verify indexes using default DATABASE_URL
  %(prog)s --db-url "postgres://..."    Verify with custom connection
  %(prog)s --verbose                    Show detailed comparison for each index

Exit codes:
  0 - All indexes verified successfully
  1 - One or more indexes missing or incorrect
        """
    )

    parser.add_argument(
        '--db-url',
        type=str,
        default=None,
        help='PostgreSQL connection URL (default: DATABASE_URL env var or postgres://sensor_user:sensor_password@localhost:5434/sensor_db)'
    )

    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Show detailed comparison for each index (failures and warnings)'
    )

    args = parser.parse_args()

    try:
        # Run verification
        passed, results = verify_indexes(db_url=args.db_url, verbose=args.verbose)

        # Exit with appropriate code
        sys.exit(0 if passed else 1)

    except psycopg2.Error as e:
        print(f"ERROR: Database connection or query failed: {e}", file=sys.stderr)
        sys.exit(2)
    except Exception as e:
        print(f"ERROR: Unexpected error: {e}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
