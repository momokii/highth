# Phase 2: Data Generation Tasks

**Goal:** Generate 50M rows of realistic test data with Zipf-like distribution for hot keys.

**Estimated Time:** 1-3 hours
**Total Tasks:** 5
**Entry Criteria:** Phase 1 complete

---

## TASK-019: Create Data Generation Script

**Status:** pending
**Dependencies:** TASK-018
**Estimated Time:** 30 minutes

**Description:**
Create a Go program to generate 50M sensor readings with Zipf distribution.

**Steps:**
1. Create `scripts/data-gen.go`
2. Implement Zipf distribution for device_id selection
3. Implement batch insertion (1000 rows per transaction)
4. Add progress reporting

**Output Definition:**
- data-gen.go file exists
- Implements batch insertion
- Implements Zipf distribution

**File:** `scripts/data-gen.go`

**Key Implementation Points:**
```go
// Zipf distribution: 1,000 devices, non-uniform distribution
// Top 1% (10 devices) get ~4% of readings
// Top 5% (50 devices) get ~15% of readings
// Bottom 40% get ~10% of readings

// Batch size: 1000 rows
// Disable autovacuum during load
// Use prepared statements
```

**Verification Commands:**
```bash
cat scripts/data-gen.go
```

**Next Task:** TASK-020

---

## TASK-020: Configure Zipf Distribution Parameters

**Status:** pending
**Dependencies:** TASK-019
**Estimated Time:** 20 minutes

**Description:**
Verify the Zipf distribution parameters match the specification.

**Distribution Target:**
| Percentile | Devices | Readings/Device | Total | % of Data |
|-----------|---------|-----------------|-------|-----------|
| Top 1% | 10 | 200,000 | 2,000,000 | 4% |
| Top 5% | 50 | 150,000 | 7,500,000 | 15% |
| Top 20% | 200 | 75,000 | 15,000,000 | 30% |
| Middle 40% | 400 | 40,000 | 16,000,000 | 32% |
| Bottom 40% | 400 | 12,500 | 5,000,000 | 10% |
| **Total** | **1,000** | **50,000 avg** | **50,000,000** | **100%** |

**Output Definition:**
- Zipf parameters configured correctly
- Distribution matches specification

**Verification:**
Review the data-gen.go code to confirm Zipf distribution implementation.

**Next Task:** TASK-021

---

## TASK-021: Run Data Generation (50M Rows)

**Status:** pending
**Dependencies:** TASK-020
**Estimated Time:** 1-2 hours

**Description:**
Execute the data generation script to insert 50M rows.

**Steps:**
1. Set DATABASE_URL in .env
2. Run data generation script
3. Monitor progress
4. Wait for completion

**Output Definition:**
- 50,000,000 rows inserted
- Script completes without errors

**Commands:**
```bash
# Update .env with database URL
export DATABASE_URL="postgres://sensor_user:sensor_password@localhost:5432/sensor_db"

# Run data generation
cd scripts
go run data-gen.go
```

**Expected Output:**
```
Starting data generation...
Target: 50,000,000 rows
Batch size: 1000 rows

Progress: 10% (5,000,000 rows) - ETA: 45 minutes
Progress: 20% (10,000,000 rows) - ETA: 40 minutes
...
Progress: 100% (50,000,000 rows) - Complete!

Data generation complete in 48 minutes 32 seconds.
```

**Note:** Actual time varies by hardware:
- HDD + 2 cores: 4-6 hours (not recommended)
- SSD + 4 cores: 1-2 hours
- SSD + 8 cores: 30-60 minutes
- NVMe + 8 cores: 20-40 minutes

**Troubleshooting:**
- If connection drops: Script should reconnect and resume
- If disk space issue: Check available space with `df -h`

**Next Task:** TASK-022

---

## TASK-022: Verify Row Count and Distribution

**Status:** pending
**Dependencies:** TASK-021
**Estimated Time:** 15 minutes

**Description:**
Verify the data was inserted correctly and matches the Zipf distribution.

**Steps:**
1. Verify total row count = 50M
2. Verify distribution is non-uniform
3. Check percentiles match specification

**Output Definition:**
- Row count = 50,000,000
- Distribution is non-uniform (Zipf-like)
- Top percentile has significantly more readings than bottom

**Verification Commands:**
```bash
# Total row count
psql "$DATABASE_URL" -c "SELECT count(*) FROM sensor_readings;"

# Distribution check (percentiles)
psql "$DATABASE_URL" -c "
SELECT
    percentile_cont(0.50) WITHIN GROUP (ORDER BY reading_count) as p50_median,
    percentile_cont(0.90) WITHIN GROUP (ORDER BY reading_count) as p90_top10,
    percentile_cont(0.10) WITHIN GROUP (ORDER BY reading_count) as p10_bottom10,
    max(reading_count) as max_readings,
    min(reading_count) as min_readings
FROM (SELECT device_id, count(*) as reading_count
      FROM sensor_readings
      GROUP BY device_id) counts;"

# Top devices
psql "$DATABASE_URL" -c "
SELECT device_id, count(*) as reading_count
FROM sensor_readings
GROUP BY device_id
ORDER BY reading_count DESC
LIMIT 10;"
```

**Expected Output:**
```
 count
----------
 50000000

 p50_median | p90_top10 | p10_bottom10 | max_readings | min_readings
------------+-----------+--------------+--------------+--------------
      40000 |    150000 |        12500 |       200000 |        10000
```

**Note:** Exact values will vary, but distribution should show:
- Median ~40K readings/device
- Top 10% devices have significantly more than bottom 10%
- Max ~200K, min ~10K

**Next Task:** TASK-023

---

## TASK-023: Run ANALYZE After Data Load

**Status:** pending
**Dependencies:** TASK-022
**Estimated Time:** 5 minutes

**Description:**
Run ANALYZE to update statistics after data load.

**Steps:**
1. Run ANALYZE on sensor_readings table
2. Verify statistics updated

**Output Definition:**
- ANALYZE completed successfully
- Query planner has accurate statistics

**Verification Commands:**
```bash
psql "$DATABASE_URL" -c "ANALYZE sensor_readings;"
psql "$DATABASE_URL" -c "SELECT schemaname, tablename, n_live_tup, n_dead_tup FROM pg_stat_user_tables WHERE tablename = 'sensor_readings';"
```

**Expected Output:**
```
ANALYZE

 schemaname |   tablename    | n_live_tup | n_dead_tup
------------+----------------+------------+------------
 public     | sensor_readings |   50000000 |          0
```

**Note:** This ensures the query optimizer has accurate statistics for 50M rows.

**Next Task:** TASK-024 (Phase 3)

---

## Phase 2 Completion Checklist

- [ ] TASK-019: Data generation script created
- [ ] TASK-020: Zipf distribution parameters configured
- [ ] TASK-021: 50M rows inserted
- [ ] TASK-022: Row count and distribution verified
- [ ] TASK-023: ANALYZE run

**When all tasks complete:** Update `.claude/state/progress.json` and proceed to Phase 3.

---

**Phase Document Version:** 1.0
**Last Updated:** 2026-03-11
