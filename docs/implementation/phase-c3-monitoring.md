# Phase C3: Performance Monitoring with Prometheus

## Overview

This phase implements comprehensive Prometheus metrics collection for production monitoring and observability. The metrics middleware provides real-time insights into API performance, database health, cache effectiveness, and application behavior.

**Business Context:** Production systems require observability to:
- Detect performance degradation before users notice
- Identify bottlenecks during high-traffic periods
- Validate optimization impact with quantitative data
- Enable data-driven capacity planning
- Support incident response with historical context

**Performance Goals:**
- < 1% CPU overhead for metrics collection
- < 10 MB memory overhead for metrics registry
- Complete trace of request lifecycle (incoming → cache → DB → response)
- Support for Prometheus scraping and Grafana visualization

## Implementation

### Files Created
- `internal/middleware/metrics.go`: Prometheus metrics collection middleware
- `internal/middleware/prometheus.go`: Prometheus metrics endpoint handler
- `configs/grafana/dashboards/higth-dashboard.json`: Grafana dashboard configuration
- `docs/implementation/phase-c3-monitoring.md`: This documentation

### Files Modified
- `cmd/api/main.go`: Added metrics middleware and /metrics endpoint
- `go.mod`: Added Prometheus client dependencies

## Metrics Collected

### HTTP Metrics

#### `http_requests_total` (Counter)
**Description**: Total number of HTTP requests received
**Labels**: `method` (GET, POST, etc.), `endpoint` (request path), `status` (200, 404, 500, etc.)

**Use Cases**:
- Track request volume by endpoint
- Monitor error rates (status != 200)
- Identify traffic patterns over time
- Alert on abnormal error rates

**Example Query**:
```promql
# Error rate by endpoint
rate(http_requests_total{status=~"5.."}[5m])

# Requests per second by endpoint
sum(rate(http_requests_total[5m])) by (endpoint)
```

#### `http_request_duration_seconds` (Histogram)
**Description**: HTTP request latency in seconds
**Labels**: `method`, `endpoint`
**Buckets**: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]

**Use Cases**:
- Track latency percentiles (p50, p95, p99)
- Identify slow endpoints
- Validate performance improvements
- SLA compliance monitoring

**Example Query**:
```promql
# p95 latency by endpoint
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (endpoint, le))

# Average latency
sum(rate(http_request_duration_seconds_sum[5m])) by (endpoint) /
sum(rate(http_request_duration_seconds_count[5m])) by (endpoint)
```

#### `http_response_size_bytes` (Histogram)
**Description**: HTTP response size in bytes
**Labels**: `method`, `endpoint`
**Buckets**: [100, 1000, 10000, 100000, 1000000]

**Use Cases**:
- Track bandwidth usage
- Identify unusually large responses
- Validate compression effectiveness
- Capacity planning for network egress

**Example Query**:
```promql
# p95 response size
histogram_quantile(0.95, sum(rate(http_response_size_bytes_bucket[5m])) by (endpoint, le))

# Bytes per second
sum(rate(http_response_size_bytes_sum[5m])) by (endpoint)
```

#### `http_requests_in_flight` (Gauge)
**Description**: Current number of in-flight HTTP requests
**Labels**: None (scalar)

**Use Cases**:
- Track concurrent request count
- Detect request storms
- Identify connection leaks
- Alert on queue buildup

**Example Query**:
```promql
# Current concurrent requests
http_requests_in_flight

# Max concurrent requests over time
max_over_time(http_requests_in_flight[5m])
```

### Database Metrics

#### `db_query_duration_seconds` (Histogram)
**Description**: Database query execution time in seconds
**Labels**: None (scalar, per-query metrics added manually)
**Buckets**: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5]

**Use Cases**:
- Track database query performance
- Identify slow queries
- Detect database degradation
- Validate connection pooling effectiveness

**Example Query**:
```promql
# p95 query latency
histogram_quantile(0.95, db_query_duration_seconds)

# Average query latency
rate(db_query_duration_seconds_sum[5m]) / rate(db_query_duration_seconds_count[5m])
```

#### `db_connections_active` (Gauge)
**Description**: Number of active database connections
**Labels**: None (scalar)

**Use Cases**:
- Monitor connection pool usage
- Detect connection exhaustion
- Validate pool sizing
- Alert on high connection count

**Example Query**:
```promql
# Connection pool utilization
db_connections_active / 50  # Assuming max_connections=50

# Average connections over time
avg_over_time(db_connections_active[5m])
```

#### `db_connections_idle` (Gauge)
**Description**: Number of idle database connections
**Labels**: None (scalar)

**Use Cases**:
- Monitor pool efficiency
- Detect connection leaks (idle not decreasing)
- Validate min_connections setting

**Example Query**:
```promql
# Connection pool health
db_connections_idle / db_connections_active

# Total connections
db_connections_active + db_connections_idle
```

### Cache Metrics

#### `cache_hits_total` (Counter)
**Description**: Total number of cache hits
**Labels**: None (scalar)

**Use Cases**:
- Track cache effectiveness
- Validate cache strategy
- Identify cache warming progress

#### `cache_misses_total` (Counter)
**Description**: Total number of cache misses
**Labels**: None (scalar)

**Use Cases**:
- Track cache misses
- Calculate cache hit rate
- Identify cache warming needs

#### Derived: Cache Hit Rate
**Description**: Percentage of requests served from cache (calculated)

**Example Query**:
```promql
# Cache hit rate
cache_hits_total / (cache_hits_total + cache_misses_total) * 100

# Cache hit rate over time
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m])) * 100
```

### Application Metrics

#### `sensor_readings_served_total` (Counter)
**Description**: Total number of sensor readings served to clients
**Labels**: `reading_type` (temperature, humidity, pressure, etc.)

**Use Cases**:
- Track data volume served
- Identify popular reading types
- Validate data distribution
- Capacity planning

**Example Query**:
```promql
# Readings per second by type
sum(rate(sensor_readings_served_total[5m])) by (reading_type)

# Total readings served
sum(sensor_readings_served_total)
```

## Configuration

### Environment Variables

```bash
# Enable/disable metrics collection (default: true)
METRICS_ENABLED=true

# Metrics endpoint path (default: /metrics)
METRICS_PATH=/metrics

# Prometheus namespace (default: highth)
METRICS_NAMESPACE=highth

# Metrics subsystem (default: api)
METRICS_SUBSYSTEM=api
```

### Docker Compose Configuration

```yaml
services:
  api:
    environment:
      - METRICS_ENABLED=true
      - METRICS_PATH=/metrics
    labels:
      - "prometheus.io/scrape=true"
      - "prometheus.io/path=/metrics"
      - "prometheus.io/port=8080"

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./configs/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    volumes:
      - ./configs/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./configs/grafana/datasources:/etc/grafana/provisioning/datasources
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    ports:
      - "3000:3000"
```

### Prometheus Configuration

`configs/prometheus/prometheus.yml`:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'highth-api'
    static_configs:
      - targets: ['api:8080']
    metrics_path: /metrics
    scrape_interval: 5s
```

## Performance Impact

### Resource Overhead

| Resource | Overhead | Notes |
|----------|----------|-------|
| CPU | < 1% | Histogram operations are efficient |
| Memory | ~5-10 MB | Metrics registry and histogram buckets |
| Network | +10 KB/s | /metrics endpoint ~10 KB per scrape |
| Disk | ~1 GB/day | Prometheus retention (15s sample rate) |

### Optimization Strategies

1. **Reduce scrape interval**: Increase from 5s to 15s for lower overhead
2. **Reduce histogram buckets**: Remove unused buckets for memory savings
3. **Disable unused metrics**: Comment out metrics not needed for monitoring
4. **Use exemplars**: Add trace IDs for distributed tracing (optional)

## Grafana Dashboard

### Dashboard Panels

1. **HTTP Request Rate**: Requests per second by endpoint (graph)
2. **Error Rate**: Percentage of requests with 5xx status (gauge)
3. **Latency (p50, p95, p99)**: Request latency percentiles (graph)
4. **Response Size**: p95 response size by endpoint (graph)
5. **Concurrent Requests**: Current in-flight requests (gauge)
6. **DB Query Latency**: p95 database query latency (graph)
7. **DB Connections**: Active vs idle connections (graph)
8. **Cache Hit Rate**: Percentage of cache hits (gauge)
9. **Sensor Readings Served**: Readings per second by type (graph)

### Import Dashboard

1. Navigate to Grafana → Dashboards → Import
2. Upload `configs/grafana/dashboards/higth-dashboard.json`
3. Select Prometheus data source
4. Save and view dashboard

## Testing Steps

### 1. Verify Metrics Endpoint

```bash
# Check metrics endpoint returns data
curl http://localhost:8080/metrics

# Verify specific metrics exist
curl http://localhost:8080/metrics | grep http_request_duration_seconds
curl http://localhost:8080/metrics | grep db_query_duration_seconds
curl http://localhost:8080/metrics | grep cache_hits_total
```

### 2. Generate Test Traffic

```bash
# Generate traffic to populate metrics
for i in {1..100}; do
  curl "http://localhost:8080/api/v1/sensor-readings?device_id=device-1&limit=100" &
done
wait

# Check metrics updated
curl http://localhost:8080/metrics | grep http_requests_total
```

### 3. Validate Metrics in Prometheus

```bash
# Query Prometheus API
curl "http://localhost:9090/api/v1/query?query=http_requests_total"

# Check if metrics are being scraped
curl "http://localhost:9090/api/v1/targets"
```

### 4. Load Test with Metrics

```bash
# Run load test
./scripts/test-runner.sh

# Monitor metrics during test
watch -n 1 'curl -s http://localhost:8080/metrics | grep http_request_duration_seconds'

# Query Prometheus after test
# Open http://localhost:9090 and run:
# rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])
```

### 5. Validate Grafana Dashboard

```bash
# Access Grafana
open http://localhost:3000

# Login (admin/admin)
# Navigate to Dashboards → Higth API Dashboard
# Verify all panels show data
```

## Rollback Plan

If metrics cause issues:

### Disable Metrics at Runtime

```bash
# Set environment variable
export METRICS_ENABLED=false

# Restart API
docker-compose restart api

# Or remove middleware from cmd/api/main.go
# Comment out: r.Use(higthmiddleware.MetricsMiddleware)
```

### Remove Metrics Endpoint

```go
// In cmd/api/main.go, remove:
// r.Get("/metrics", prometheus.Handler())
```

### Clean Up Prometheus Data

```bash
# Stop Prometheus
docker-compose stop prometheus

# Remove data directory
rm -rf ./data/prometheus

# Restart without metrics
docker-compose up -d postgres redis api
```

## Monitoring and Maintenance

### Daily Checks

```bash
# Check metrics endpoint is accessible
curl -f http://localhost:8080/metrics || echo "Metrics endpoint down"

# Check Prometheus is scraping
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health=="up")'

# Check for high error rates
curl -s "http://localhost:9090/api/v1/query?query=rate(http_requests_total{status=~\"5..\"}[5m])" | jq '.data.result[0].value[1]'
```

### Weekly Maintenance

```bash
# Review metric usage and remove unused metrics
# Identify metrics with zero or minimal change

# Check Prometheus disk usage
du -sh ./data/prometheus

# Review and optimize Grafana dashboard queries
# Identify slow dashboard queries (>5s)
```

### Monthly Tasks

```bash
# Update Prometheus retention if needed
# In configs/prometheus/prometheus.yml:
# retention.time: 30d

# Review and update alerting rules
# Add new alerts based on observed patterns

# Backup Grafana dashboards
curl -u admin:admin http://localhost:3000/api/dashboards/export > dashboards-backup-$(date +%Y%m%d).json
```

## Troubleshooting

### Issue 1: Metrics Not Appearing in Prometheus

**Symptoms**: Metrics endpoint works but Prometheus shows no data

**Diagnosis**:
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job, health, lastError}'

# Check Prometheus logs
docker-compose logs prometheus | grep -i error
```

**Solution**:
- Verify Prometheus configuration: `scrape_configs` in `prometheus.yml`
- Check network connectivity: `docker-compose exec prometheus ping api`
- Verify metrics endpoint: `curl http://api:8080/metrics` from Prometheus container

### Issue 2: High Memory Usage

**Symptoms**: API container memory usage increased significantly

**Diagnosis**:
```bash
# Check container memory usage
docker stats highth-api

# Check metrics registry size
curl http://localhost:8080/metrics | wc -l
```

**Solution**:
- Reduce histogram bucket count
- Remove unused metrics
- Increase Prometheus scrape interval

### Issue 3: Missing Labels

**Symptoms**: Metrics missing expected labels (e.g., endpoint, status)

**Diagnosis**:
```bash
# Check metrics format
curl http://localhost:8080/metrics | grep http_requests_total
```

**Solution**:
- Verify middleware is wrapping requests correctly
- Check for skipped endpoints (health endpoints may bypass metrics)
- Add labels manually in handlers if needed

## References

### Prometheus Documentation
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Histogram Quantiles](https://prometheus.io/docs/practices/histograms/)

### Grafana Documentation
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)

### Industry Standards
- USE Method (Utilization, Saturation, Errors): https://www.usenix.org/conference/lisa12/technical-sessions/presentation/grennan
- RED Method (Rate, Errors, Duration): https://www.weave.works/blog/the-red-method-key-metrics-for-microservices/
- Google SRE Book: https://sre.google/sre-book/table-of-contents/

## Success Criteria

- [ ] /metrics endpoint accessible and returns Prometheus format
- [ ] All HTTP metrics collected (requests, duration, size, in-flight)
- [ ] Database metrics collected (query duration, connections)
- [ ] Cache metrics collected (hits, misses)
- [ ] Application metrics collected (readings served)
- [ ] Prometheus successfully scraping metrics
- [ ] Grafana dashboard displays all panels
- [ ] CPU overhead < 1%
- [ ] Memory overhead < 10 MB
- [ ] No latency impact on API endpoints

## Changelog

**2026-03-15**: Initial Phase C3 documentation created
- Defined comprehensive metrics collection strategy
- Created testing and validation procedures
- Documented Grafana dashboard configuration
- Added troubleshooting and maintenance procedures
