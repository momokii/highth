# Nginx Reverse Proxy

## Overview

This document describes how to add an Nginx reverse proxy in front of the Higth API for production deployment. Nginx provides SSL termination, rate limiting, HTTP/2 support, and connection pooling.

## Why Add Nginx?

| Feature | Benefit |
|---------|---------|
| **SSL Termination** | Offloads TLS/HTTPS from Go application, reducing CPU overhead |
| **Rate Limiting** | Protects against abusive clients and DDoS attacks |
| **HTTP/2 Support** | Better multiplexing for concurrent requests |
| **Connection Pooling** | Fewer connections to Go application |
| **Static File Serving** | Efficient for API documentation and assets |
| **Caching** | Can cache API responses at edge level |

## Prerequisites

- Nginx 1.18+ installed
- SSL certificate (for production HTTPS)
- Domain name configured (optional, for production)

## Configuration

### Nginx Configuration File

Create `nginx.conf`:

```nginx
upstream api_backend {
    least_conn;
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    # SSL Configuration
    ssl_certificate /etc/ssl/certs/api.crt;
    ssl_certificate_key /etc/ssl/private/api.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Rate Limiting
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
    limit_req zone=api_limit burst=20 nodelay;

    # Logging
    access_log /var/log/nginx/api_access.log;
    error_log /var/log/nginx/api_error.log;

    # Proxy Settings
    location /api/ {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health Check Endpoint (no rate limiting)
    location /health {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        access_log off;
    }

    # Prometheus Metrics (internal access only)
    location /metrics {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        allow 127.0.0.1;
        allow 10.0.0.0/8;
        deny all;
    }
}

# HTTP to HTTPS Redirect
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$server_name$request_uri;
}
```

### Docker Compose Integration

Add Nginx service to `docker-compose.yml`:

```yaml
services:
  # ... existing services (postgres, redis, api) ...

  nginx:
    image: nginx:alpine
    container_name: highth-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/ssl:ro
    depends_on:
      - api
    networks:
      - highth-network
    restart: unless-stopped
```

## Installation Steps

### Option 1: Docker Deployment (Recommended)

1. **Create nginx folder structure:**
   ```bash
   mkdir -p nginx/ssl
   ```

2. **Generate self-signed certificate for development:**
   ```bash
   openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
     -keyout nginx/ssl/api.key \
     -out nginx/ssl/api.crt \
     -subj "/CN=localhost"
   ```

3. **For production, use Let's Encrypt:**
   ```bash
   certbot certonly --webroot -w /var/www/html \
     -d api.example.com
   ```

4. **Update docker-compose.yml** with nginx service (see above)

5. **Start services:**
   ```bash
   docker-compose up -d
   ```

### Option 2: Native Nginx Installation

1. **Install Nginx:**
   ```bash
   sudo apt update
   sudo apt install nginx
   ```

2. **Copy configuration:**
   ```bash
   sudo cp nginx.conf /etc/nginx/sites-available/higth-api
   sudo ln -s /etc/nginx/sites-available/higth-api /etc/nginx/sites-enabled/
   ```

3. **Test configuration:**
   ```bash
   sudo nginx -t
   ```

4. **Reload Nginx:**
   ```bash
   sudo systemctl reload nginx
   ```

## Verification

### Test HTTP to HTTPS Redirect

```bash
curl -I http://localhost
# Expected: HTTP/1.1 301 Moved Permanently
# Location: https://localhost/
```

### Test API Through Nginx

```bash
curl https://localhost/health
# Expected: {"status":"healthy",...}
```

### Test Rate Limiting

```bash
# Send requests rapidly (should hit rate limit after ~30 requests)
for i in {1..50}; do
  curl https://localhost/api/v1/sensor-readings?device_id=sensor-001&limit=10
done
# Expected: HTTP 429 Too Many Requests after rate limit exceeded
```

## Performance Tuning

### Worker Processes

Edit `/etc/nginx/nginx.conf`:

```nginx
worker_processes auto;
worker_rlimit_nofile 65535;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}
```

### Buffer Sizes

```nginx
http {
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;

    client_body_buffer_size 128k;
    client_max_body_size 1m;
    client_header_buffer_size 1k;
    large_client_header_buffers 4 4k;
    output_buffers 1 32k;
    postpone_output 1460;
}
```

## Monitoring

### Nginx Status Module

Add to `nginx.conf`:

```nginx
server {
    listen 127.0.0.1:8080;
    location /nginx_status {
        stub_status on;
        access_log off;
        allow 127.0.0.1;
        deny all;
    }
}
```

Check status:
```bash
curl http://127.0.0.1:8080/nginx_status
```

### Log Analysis

```bash
# Top request paths
awk '{print $7}' /var/log/nginx/api_access.log | sort | uniq -c | sort -rn | head -10

# Response code distribution
awk '{print $9}' /var/log/nginx/api_access.log | sort | uniq -c | sort -rn

# Average response time
awk '{print $NF}' /var/log/nginx/api_access.log | awk '{sum+=$1; count++} END {print sum/count}'
```

## Security Considerations

### Security Headers

Add to server block:

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
add_header Content-Security-Policy "default-src 'self'" always;
```

### IP Whitelisting (for /metrics)

```nginx
location /metrics {
    proxy_pass http://api_backend;
    allow 127.0.0.1;
    allow 10.0.0.0/8;
    deny all;
}
```

## Troubleshooting

### Issue: 502 Bad Gateway

**Cause:** API service not running or not accessible

**Solution:**
```bash
# Check if API is running
docker ps | grep highth-api

# Check API health
curl http://localhost:8080/health

# Check Nginx error log
tail -f /var/log/nginx/error.log
```

### Issue: SSL Certificate Errors

**Cause:** Invalid or expired certificate

**Solution:**
```bash
# Check certificate expiry
openssl x509 -in /etc/ssl/certs/api.crt -noout -dates

# Regenerate certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/api.key \
  -out /etc/ssl/certs/api.crt
```

### Issue: Rate Limiting Too Aggressive

**Cause:** Rate limit too low for legitimate traffic

**Solution:** Adjust in `nginx.conf`:
```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=50r/s;
limit_req zone=api_limit burst=100 nodelay;
```

## Related Documentation

- **[../architecture.md](../architecture.md)** - Infrastructure components section
- **[../api-spec.md](../api-spec.md)** - API endpoint specifications
- **[../implementation/validation-checklist.md](../implementation/validation-checklist.md)** - Validation checklist
