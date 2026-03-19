#!/usr/bin/env python3
"""
Fast IoT sensor data generator using PostgreSQL COPY.
Expected performance: 100,000+ rows/sec (50M rows in ~8-10 minutes)

Usage:
    python3 generate_data_fast.py [rows] [options]

Examples:
    python3 generate_data_fast.py 1000000           # Generate 1M rows
    python3 generate_data_fast.py 10000000 --days 30 # Generate 10M rows over 30 days
    python3 generate_data_fast.py                      # Generate default 50M rows
"""

import argparse
import csv
import io
import os
import random
import sys
import time
from datetime import datetime, timedelta

import psycopg2

# Parse command line arguments
parser = argparse.ArgumentParser(
    description='Generate IoT sensor data for Higth experiments',
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
args = parser.parse_args()

# Configuration from command line
TOTAL_ROWS = args.rows
NUM_DEVICES = args.devices
DATA_DURATION_DAYS = args.days
BATCH_SIZE = args.batch_size

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

def main():
    # Get database connection
    db_url = os.getenv("DATABASE_URL", "postgres://sensor_user:sensor_password@localhost:5434/sensor_db")

    print("=" * 70)
    print("Fast IoT Sensor Data Generator")
    print("=" * 70)
    print(f"Total rows to generate: {TOTAL_ROWS:,}")
    print(f"Number of devices: {NUM_DEVICES:,}")
    print(f"Data duration: {DATA_DURATION_DAYS} days")
    print(f"Batch size: {BATCH_SIZE:,}")
    print(f"Expected rate: 100,000+ rows/sec")
    if TOTAL_ROWS >= 10_000_000:
        print(f"Expected time: ~{TOTAL_ROWS / 100000 / 60:.0f} minutes")
    else:
        print(f"Expected time: <1 minute")
    print("=" * 70)

    print("\nConnecting to database...")
    conn = psycopg2.connect(db_url)
    conn.autocommit = False
    cursor = conn.cursor()

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

    start_time = time.time()
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
                elapsed = time.time() - start_time
                rate = rows_generated / elapsed if elapsed > 0 else 0
                progress = rows_generated / TOTAL_ROWS * 100
                remaining = (TOTAL_ROWS - rows_generated) / rate if rate > 0 else 0

                print(f"Progress: {progress:5.1f}% | Rows: {rows_generated:>12,} | "
                      f"Rate: {rate:,.0f} rows/sec | ETA: {remaining/60:4.1f} min")

        # Final stats
        print("-" * 70)
        print("\nRunning ANALYZE on sensor_readings table...")
        cursor.execute("ANALYZE sensor_readings")
        conn.commit()

        # Verify count
        cursor.execute("SELECT COUNT(*) FROM sensor_readings")
        count = cursor.fetchone()[0]

        cursor.execute("SELECT COUNT(DISTINCT device_id) FROM sensor_readings")
        device_count = cursor.fetchone()[0]

        elapsed = time.time() - start_time
        rate = TOTAL_ROWS / elapsed if elapsed > 0 else 0

        print("\n" + "=" * 70)
        print("DATA GENERATION COMPLETED SUCCESSFULLY!")
        print("=" * 70)
        print(f"Total rows:      {count:,}")
        print(f"Unique devices:  {device_count:,}")
        print(f"Total time:      {elapsed/60:.1f} minutes ({elapsed:.1f} seconds)")
        print(f"Average rate:    {rate:,.0f} rows/sec")
        print("=" * 70)

    except Exception as e:
        print(f"\nERROR: {e}")
        conn.rollback()
        raise
    finally:
        cursor.close()
        conn.close()

if __name__ == "__main__":
    main()
