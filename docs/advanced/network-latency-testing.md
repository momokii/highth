# Network Latency Testing

## Overview

All Higth benchmarks run on `localhost` with near-zero network latency (~0.01ms). Real deployments have 1-50ms network latency between the API, database, and Redis. This guide shows how to inject network latency to test realistic scenarios.

## Why Test with Latency

- **Cache effectiveness**: At 10ms network latency, Redis saves 10ms per request on cache hits
- **Connection pooling**: Higher latency means connections are held longer → pool may exhaust sooner
- **Database queries**: Network RTT adds directly to query latency
- **Throughput ceiling**: Latency reduces the maximum sustainable RPS

## Using tc (Traffic Control)

`tc` is a Linux kernel feature for controlling network traffic. It can add delay, loss, and bandwidth limits to any network interface.

### Add Latency Between Containers

```bash
# Find the Docker bridge network interface
INTERFACE=$(docker network inspect highth-network -f '{{range .Options}}{{.}}{{end}}' 2>/dev/null)
# Or use the default docker0 bridge
INTERFACE=docker0

# Add 10ms one-way latency (20ms round-trip) to all traffic on the interface
sudo tc qdisc add dev $INTERFACE root netem delay 10ms

# Verify
sudo tc qdisc show dev $INTERFACE
```

### Add Latency with Jitter

```bash
# 10ms ± 2ms jitter (more realistic)
sudo tc qdisc add dev docker0 root netem delay 10ms 2ms
```

### Add Packet Loss

```bash
# 0.1% packet loss (simulates unreliable network)
sudo tc qdisc add dev docker0 root netem delay 5ms loss 0.1%
```

### Remove All Latency Rules

```bash
# Remove the qdisc (restores normal network)
sudo tc qdisc del dev docker0 root netem 2>/dev/null
```

## Testing Workflow

```bash
# 1. Start services
docker compose up -d

# 2. Run baseline benchmark (no latency)
./tests/run-benchmarks.sh --tier low -s pk-lookup

# 3. Add 10ms latency
sudo tc qdisc add dev docker0 root netem delay 10ms

# 4. Run same benchmark with latency
./tests/run-benchmarks.sh --tier low -s pk-lookup

# 5. Compare results
./scripts/compare_benchmarks.sh <baseline.json> <latency.json>

# 6. Clean up
sudo tc qdisc del dev docker0 root netem
```

## Expected Impact

| Latency (RTT) | Expected Effect |
|---------------|----------------|
| 0ms (baseline) | Cache HIT: ~1-3ms, Cache MISS: ~50-100ms |
| 2ms | +2ms on all requests (network RTT) |
| 10ms | Cache HIT: ~12ms, Cache MISS: ~60-110ms. Cache more valuable. |
| 50ms | Cache HIT: ~52ms, Cache MISS: ~100-150ms. Cache is critical. |

Higher latency makes caching MORE valuable — the 30s Redis TTL amortizes the network cost across many requests.

## Using in k6 Scenarios

k6 doesn't natively inject network latency. Use `tc` at the OS level as shown above, or use a Docker network with latency:

```bash
# Alternative: use a Docker network with latency using tc in a sidecar
docker run --rm --cap-add NET_ADMIN \
  --network highth-network \
  wazirnd/netem delay 10ms
```

## Troubleshooting

**"RTNETLINK answers: Operation not permitted"**
→ Run with `sudo`. `tc` requires root privileges.

**"Cannot find device 'docker0'"**
→ Use `ip link show` to find the correct interface name. It may be `br-xxxxx` for custom Docker networks.

**Latency not affecting containers**
→ Docker containers on the same network bypass the bridge interface. Use `network_mode: host` or apply tc to the container's veth interface directly.
