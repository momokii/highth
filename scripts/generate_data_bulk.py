#!/usr/bin/env python3
"""
Bulk IoT sensor data generator with index optimization.

Drops secondary indexes before loading, recreates them after.
Much faster for large datasets because PostgreSQL builds indexes from
a single table scan instead of incremental updates per row.

Usage:
    python3 generate_data_bulk.py [rows] [options]

Examples:
    python3 generate_data_bulk.py 1000000           # Generate 1M rows
    python3 generate_data_bulk.py 10000000 --days 30 # Generate 10M rows over 30 days
    python3 generate_data_bulk.py                      # Generate default 50M rows
"""

import argparse
import csv
import io
import os
import random
import sys
import time
from datetime import datetime, timedelta
from dotenv import load_dotenv

# Load variables from .env
load_dotenv()

import psycopg2

# Add script directory to path for importing verify_indexes
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from verify_indexes import verify_indexes

# Indexes to drop/recreate (exclude primary key)
SECONDARY_INDEXES = [
    'idx_sensor_readings_device_timestamp',
    'idx_sensor_readings_reading_type',
    'idx_sensor_readings_timestamp_brin',
    'idx_sensor_readings_device_type_timestamp',
    'idx_sensor_readings_device_covering',
]

# Parse command line arguments
parser = argparse.ArgumentParser(
    description='Generate IoT sensor data with index optimization (drop before load, recreate after)',
    formatter_class=argparse.RawDescriptionHelpFormatter,
    epilog="""
Examples:
  %(prog)s 1000000              Generate 1M rows with default settings
  %(prog)s 10000000 --days 30     Generate 10M rows over 30 days
  %(prog)s 50000000 --devices 1000  Generate 50M rows for 1000 devices
    """
)
parser.add_argument('rows', type=int, nargs='?', default=50_000_000,
                   help='Number of rows to generate (default: 50000000)')
parser.add_argument('--devices', type=int, default=100_000,
                   help='Number of IoT devices (default: 100000)')
parser.add_argument('--days', type=int, default=90,
                   help='Data duration in days (default: 90)')
parser.add_argument('--batch-size', type=int, default=100_000,
                   help='Batch size for COPY operations (default: 100000)')
parser.add_argument('--skip-index-drop', action='store_true',
                   help='Skip dropping indexes (useful if indexes already dropped)')
parser.add_argument('--skip-index-create', action='store_true',
                   help='Skip recreating indexes (useful for debugging)')
parser.add_argument('--skip-index-verify', action='store_true',
                   help='Skip index verification after generation')
args = parser.parse_args()

# Configuration from command line
TOTAL_ROWS = args.rows
NUM_DEVICES = args.devices
DATA_DURATION_DAYS = args.days
BATCH_SIZE = args.batch_size
SKIP_INDEX_DROP = args.skip_index_drop
SKIP_INDEX_CREATE = args.skip_index_create
SKIP_INDEX_VERIFY = args.skip_index_verify

# Reading types
READING_TYPES = ["temperature", "humidity", "pressure"]
UNITS = ["°C", "%", "hPa"]


def generate_device_ids():
    """Generate device IDs."""
    return [f"sensor-{i:06d}" for i in range(NUM_DEVICES)]


def generate_batch_csv(device_ids, batch_size, time_start, time_end):
    """Generate a batch of data as CSV string."""
    output = io.StringIO()
    writer = csv.writer(output, quoting=csv.QUOTE_MINIMAL)

    time_delta = (time_end - time_start).total_seconds()

    for _ in range(batch_size):
        # Use Zipf-like distribution for device selection
        # First try to select from top 1000 devices (hot devices)
        if random.random() < 0.3:  # 30% chance for hot devices
            device_idx = random.randint(0, min(999, len(device_ids) - 1))
        else:
            device_idx = random.randint(0, len(device_ids) - 1)

        device_id = device_ids[device_idx]

        # Random timestamp
        random_seconds = random.randint(0, int(time_delta))
        timestamp = time_start + timedelta(seconds=random_seconds)

        # Reading type
        type_idx = random.randint(0, len(READING_TYPES) - 1)
        reading_type = READING_TYPES[type_idx]
        unit = UNITS[type_idx]

        # Value
        if reading_type == "temperature":
            value = -20 + random.random() * 60
        elif reading_type == "humidity":
            value = random.random() * 100
        else:  # pressure
            value = 950 + random.random() * 100

        writer.writerow([
            device_id,
            timestamp.strftime('%Y-%m-%d %H:%M:%S'),
            reading_type,
            round(value, 2),
            unit
        ])

    return output.getvalue()


def drop_secondary_indexes(cursor):
    """Drop all secondary indexes to speed up bulk loading."""
    cursor.execute("""
        SELECT indexname
        FROM pg_indexes
        WHERE tablename = 'sensor_readings'
        AND schemaname = 'public'
        AND indexname IN %s
    """, (tuple(SECONDARY_INDEXES),))

    existing_indexes = [row[0] for row in cursor.fetchall()]
    dropped = []

    for index_name in existing_indexes:
        try:
            cursor.execute(f"DROP INDEX IF EXISTS {index_name}")
            dropped.append(index_name)
            print(f"  Dropped: {index_name}")
        except Exception as e:
            print(f"  Warning: Failed to drop {index_name}: {e}")

    return dropped


def recreate_indexes(cursor, conn):
    """
    Recreate all secondary indexes with CONCURRENTLY where supported.
    
    Returns:
        tuple: (indexes_created: list, indexes_failed: list)
    """
    # CONCURRENTLY requires autocommit, so we need to handle this specially
    # We'll use autocommit for each index creation
    original_autocommit = conn.autocommit
    conn.autocommit = True

    indexes_created = []
    indexes_failed = []

    try:
        # idx_sensor_readings_device_timestamp
        print("  Creating idx_sensor_readings_device_timestamp...")
        try:
            cursor.execute("""
                CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sensor_readings_device_timestamp
                ON sensor_readings (device_id, timestamp DESC)
            """)
            indexes_created.append('idx_sensor_readings_device_timestamp')
        except Exception as e:
            print(f"    Warning: Failed to create idx_sensor_readings_device_timestamp: {e}")
            indexes_failed.append('idx_sensor_readings_device_timestamp')

        # idx_sensor_readings_reading_type
        print("  Creating idx_sensor_readings_reading_type...")
        try:
            cursor.execute("""
                CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sensor_readings_reading_type
                ON sensor_readings (reading_type)
            """)
            indexes_created.append('idx_sensor_readings_reading_type')
        except Exception as e:
            print(f"    Warning: Failed to create idx_sensor_readings_reading_type: {e}")
            indexes_failed.append('idx_sensor_readings_reading_type')

        # idx_sensor_readings_timestamp_brin (BRIN doesn't support CONCURRENTLY, but it's fast)
        print("  Creating idx_sensor_readings_timestamp_brin...")
        try:
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_sensor_readings_timestamp_brin
                ON sensor_readings USING BRIN (timestamp)
            """)
            indexes_created.append('idx_sensor_readings_timestamp_brin')
        except Exception as e:
            print(f"    Warning: Failed to create idx_sensor_readings_timestamp_brin: {e}")
            indexes_failed.append('idx_sensor_readings_timestamp_brin')

        # idx_sensor_readings_device_type_timestamp
        print("  Creating idx_sensor_readings_device_type_timestamp...")
        try:
            cursor.execute("""
                CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sensor_readings_device_type_timestamp
                ON sensor_readings (device_id, reading_type, timestamp DESC)
            """)
            indexes_created.append('idx_sensor_readings_device_type_timestamp')
        except Exception as e:
            print(f"    Warning: Failed to create idx_sensor_readings_device_type_timestamp: {e}")
            indexes_failed.append('idx_sensor_readings_device_type_timestamp')

        # idx_sensor_readings_device_covering (requires PostgreSQL 12+)
        print("  Creating idx_sensor_readings_device_covering...")
        try:
            cursor.execute("""
                CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sensor_readings_device_covering
                ON sensor_readings (device_id, timestamp DESC)
                INCLUDE (reading_type, value, unit)
            """)
            indexes_created.append('idx_sensor_readings_device_covering')
        except psycopg2.errors.FeatureNotSupported:
            # INCLUDE syntax requires PostgreSQL 12+
            print("    Warning: INCLUDE syntax not supported (requires PG 12+), skipping covering index")
            indexes_failed.append('idx_sensor_readings_device_covering')
        except psycopg2.errors.SyntaxError:
            print("    Warning: INCLUDE syntax not supported (requires PG 12+), skipping covering index")
            indexes_failed.append('idx_sensor_readings_device_covering')
        except Exception as e:
            print(f"    Warning: Failed to create idx_sensor_readings_device_covering: {e}")
            indexes_failed.append('idx_sensor_readings_device_covering')

    finally:
        conn.autocommit = original_autocommit

    return indexes_created, indexes_failed


def main():
    # Get database connection
    db_url = os.getenv("DATABASE_URL", "postgres://sensor_user:CHANGE_ME_POSTGRES_PASSWORD@localhost:5434/sensor_db")

    print("=" * 70)
    print("Bulk IoT Sensor Data Generator (with Index Optimization)")
    print("=" * 70)
    print(f"Total rows to generate: {TOTAL_ROWS:,}")
    print(f"Number of devices: {NUM_DEVICES:,}")
    print(f"Data duration: {DATA_DURATION_DAYS} days")
    print(f"Batch size: {BATCH_SIZE:,}")
    print(f"Skip index drop: {SKIP_INDEX_DROP}")
    print(f"Skip index create: {SKIP_INDEX_CREATE}")
    if TOTAL_ROWS >= 10_000_000:
        print(f"Expected time: ~{TOTAL_ROWS / 100000 / 60:.0f} minutes")
    else:
        print(f"Expected time: <1 minute")
    print("=" * 70)

    print("\nConnecting to database...")
    conn = psycopg2.connect(db_url)
    conn.autocommit = False
    cursor = conn.cursor()

    overall_start = time.time()
    conn_closed_for_phase5 = False  # Track connection state for Phase 5

    # ========================================================================
    # Phase 1: Drop secondary indexes
    # ========================================================================
    if not SKIP_INDEX_DROP:
        print("\n" + "=" * 70)
        print("PHASE 1: Dropping secondary indexes")
        print("=" * 70)
        dropped = drop_secondary_indexes(cursor)
        conn.commit()
        print(f"Dropped {len(dropped)} indexes")
    else:
        print("\nSkipping index drop (--skip-index-drop flag set)")

    # ========================================================================
    # Phase 2: Generate and load data
    # ========================================================================
    print("\n" + "=" * 70)
    print("PHASE 2: Generating and loading data")
    print("=" * 70)

    # Time range
    time_end = datetime.utcnow()
    time_start = time_end - timedelta(days=DATA_DURATION_DAYS)

    print(f"Time range: {time_start} to {time_end}")

    # Device IDs
    print(f"\nGenerating {NUM_DEVICES:,} device IDs...")
    device_ids = generate_device_ids()

    # Generate data in batches
    print(f"\nStarting data generation...")
    print("-" * 70)

    phase2_start = time.time()
    rows_generated = 0

    try:
        while rows_generated < TOTAL_ROWS:
            batch_size = min(BATCH_SIZE, TOTAL_ROWS - rows_generated)

            # Generate CSV batch
            csv_data = generate_batch_csv(device_ids, batch_size, time_start, time_end)

            # Use COPY for fast insert
            csv_file = io.StringIO(csv_data)
            cursor.copy_expert(
                "COPY sensor_readings (device_id, timestamp, reading_type, value, unit) FROM STDIN WITH CSV",
                csv_file
            )

            conn.commit()
            rows_generated += batch_size

            # Progress report
            if rows_generated % 100_000 == 0 or rows_generated == TOTAL_ROWS:
                elapsed = time.time() - phase2_start
                rate = rows_generated / elapsed if elapsed > 0 else 0
                progress = rows_generated / TOTAL_ROWS * 100
                remaining = (TOTAL_ROWS - rows_generated) / rate if rate > 0 else 0

                print(f"Progress: {progress:5.1f}% | Rows: {rows_generated:>12,} | "
                      f"Rate: {rate:,.0f} rows/sec | ETA: {remaining/60:4.1f} min")

        phase2_time = time.time() - phase2_start

        # ========================================================================
        # Phase 3: Recreate indexes
        # ========================================================================
        if not SKIP_INDEX_CREATE:
            print("\n" + "=" * 70)
            print("PHASE 3: Recreating indexes")
            print("=" * 70)
            phase3_start = time.time()
            created, failed = recreate_indexes(cursor, conn)
            phase3_time = time.time() - phase3_start
            print(f"Created {len(created)} indexes in {phase3_time/60:.1f} minutes")
            if failed:
                print(f"Failed to create {len(failed)} indexes: {', '.join(failed)}")
        else:
            print("\nSkipping index creation (--skip-index-create flag set)")
            phase3_time = 0
            created, failed = [], []

        # ========================================================================
        # Phase 4: ANALYZE and verify
        # ========================================================================
        print("\n" + "=" * 70)
        print("PHASE 4: ANALYZE and verification")
        print("=" * 70)

        cursor.execute("ANALYZE sensor_readings")
        conn.commit()

        # Verify count
        cursor.execute("SELECT COUNT(*) FROM sensor_readings")
        count = cursor.fetchone()[0]

        cursor.execute("SELECT COUNT(DISTINCT device_id) FROM sensor_readings")
        device_count = cursor.fetchone()[0]

        # ========================================================================
        # Phase 5: Index Verification
        # ========================================================================
        if not SKIP_INDEX_VERIFY:
            print("\n" + "=" * 70)
            print("PHASE 5: Verifying indexes")
            print("=" * 70)
            print("Checking that all expected indexes exist with correct structure...\n")

            # Close current connection to avoid connection issues
            cursor.close()
            conn.close()
            conn_closed_for_phase5 = True

            # Run index verification (creates its own connection)
            passed, results = verify_indexes(db_url=db_url)

            if not passed:
                print("\n" + "!" * 35)
                print("WARNING: Some indexes are missing or incorrect!")
                print("!" * 35)
                print("\nRun 'python3 scripts/verify_indexes.py --verbose' for details.")
                print("This may affect query performance.")
                print("\nYou can recreate indexes by running:")
                print("  ./scripts/run_migrations.sh")
        else:
            print("\nSkipping index verification (--skip-index-verify flag set)")

        # Reconnect for final summary (only if we closed it in Phase 5)
        if conn_closed_for_phase5:
            conn = psycopg2.connect(db_url)
            conn.autocommit = False
            cursor = conn.cursor()

        total_time = time.time() - overall_start
        rate = TOTAL_ROWS / total_time if total_time > 0 else 0

        print("\n" + "=" * 70)
        print("BULK DATA GENERATION COMPLETED!")
        print("=" * 70)
        print(f"Total rows:      {count:,}")
        print(f"Unique devices:  {device_count:,}")
        print(f"Phase 2 (load):  {phase2_time/60:.1f} minutes")
        if not SKIP_INDEX_CREATE:
            print(f"Phase 3 (index): {phase3_time/60:.1f} minutes")
        print(f"Total time:      {total_time/60:.1f} minutes ({total_time:.1f} seconds)")
        print(f"Overall rate:    {rate:,.0f} rows/sec")
        if not SKIP_INDEX_VERIFY:
            print(f"Phase 5 (verify): Index verification completed")
        print("=" * 70)

    except Exception as e:
        print(f"\nERROR: {e}")
        # Only rollback if connection is still open
        if not conn_closed_for_phase5:
            try:
                conn.rollback()
            except:
                pass  # Connection may already be closed
        raise
    finally:
        # Only close if connection is still open
        if not conn_closed_for_phase5:
            try:
                cursor.close()
            except:
                pass
            try:
                conn.close()
            except:
                pass


if __name__ == "__main__":
    main()
