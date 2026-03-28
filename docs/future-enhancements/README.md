# Future Enhancements

This folder documents features that were designed but not yet implemented, or enhancements that would make the system more production-ready for real-world deployment.

## Overview

The Higth IoT system is currently functional and demonstrates high-performance time-series query capabilities. However, the original design included several additional features that were not implemented in the initial version. This document serves as a roadmap for future enhancements.

## Enhancement List

1. **[Nginx Reverse Proxy](./01-nginx-reverse-proxy.md)** - SSL termination, rate limiting, HTTP/2 support
2. **[Metadata JSONB Column](./02-metadata-jsonb-column.md)** - Flexible device-specific data storage
3. **[Partitioning Strategy](./03-partitioning-strategy.md)** - Table partitioning for 100M+ row scale
4. **[Schema Type Corrections](./04-schema-type-corrections.md)** - Align schema types with original documentation

## Implementation Priority

| Priority | Enhancement | Impact | Complexity | Status |
|----------|-------------|--------|------------|--------|
| **High** | Nginx Reverse Proxy | Production readiness (SSL, rate limiting) | Medium | Not Implemented |
| **Medium** | Multi-Table Joins | Production-realistic testing with JOIN pressure | Medium | Not Implemented |
| **Medium** | Metadata JSONB Column | Flexibility for device-specific data | Low | Not Implemented |
| **Low** | Partitioning Strategy | Required only for 100M+ row scale | High | Not Implemented |
| **Low** | Schema Type Corrections | Documentation alignment | Medium | Not Implemented |

## What's Already Implemented

The following features are fully implemented and documented in the main docs:

- ✅ BRIN indexes for time-series queries
- ✅ Composite B-tree indexes for device lookups
- ✅ **Covering index for index-only scans** (added in migration 006)
- ✅ Redis caching with 30s TTL
- ✅ Materialized views for dashboard queries
- ✅ Incremental MV refresh functions
- ✅ Connection pooling (pgx)
- ✅ Health check endpoints
- ✅ k6 load testing scenarios
- ✅ Automated migration system

## Quick Reference

### When to Implement Each Enhancement

| Enhancement | Implement When... |
|-------------|-------------------|
| Nginx Reverse Proxy | Deploying to production with HTTPS requirements |
| Multi-Table Joins | Testing production-realistic workloads with JOIN operations |
| Metadata JSONB Column | Need to store device-specific configuration or calibration data |
| Partitioning Strategy | Dataset exceeds 100M rows or query performance degrades |
| Schema Type Corrections | Need exact precision to 6 decimal places or >10 digit values |

## Decision Notes

### Why These Were Not Initially Implemented

1. **Nginx Reverse Proxy**: Not needed for local development or HTTP-only environments
2. **Multi-Table Joins**: Single-table approach successfully demonstrates IoT time-series query performance and matches real-world IoT systems (InfluxDB, TimescaleDB)
3. **Metadata JSONB Column**: Base sensor readings work fine without it for demonstration purposes
4. **Partitioning Strategy**: Current scale (50M rows) performs well without partitioning
5. **Schema Type Corrections**: Current types (DECIMAL(10,2), VARCHAR(20)) are sufficient for demo

### Trade-offs Considered

| Enhancement | Benefit | Cost | Decision |
|-------------|---------|------|----------|
| Nginx | SSL termination, rate limiting | Additional infrastructure layer | Deferred to production deployment |
| Multi-Table Joins | Production-realistic testing, JOIN pressure | 2-5× query overhead, FK constraint cost | Single-table sufficient for current goals |
| Metadata | Flexible device data | Increased storage, query complexity | Deferred until actual use case |
| Partitioning | Better performance at 100M+ scale | Complex management, migration overhead | Deferred until needed |
| Schema Corrections | Matches documentation exactly | Potential data migration | Current types are sufficient |

## Related Documentation

- **[../architecture.md](../architecture.md)** - System architecture and database design
- **[../api-spec.md](../api-spec.md)** - API endpoint specifications
- **[../implementation/README.md](../implementation/README.md)** - Implementation guide

## Next Steps

To implement any of these enhancements:

1. Read the specific enhancement document
2. Review the implementation steps
3. Test in development environment first
4. Update relevant documentation after implementation
5. Run validation checklist from `../implementation/validation-checklist.md`
