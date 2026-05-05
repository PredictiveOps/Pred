# Monitoring & Observability Guide

## Quick Start

1. **Start the stack**:

   ```bash
   docker compose up -d
   ```

2. **Access Prometheus**:
   - UI: http://localhost:9090
   - Metrics: http://localhost:9090/metrics

3. **Verify all services are being scraped**:
   - Go to http://localhost:9090/targets
   - All services should show "UP" in green

## Microservices Metrics Endpoints

All microservices expose Prometheus metrics via a `/metrics` HTTP endpoint:

### Development Environment (docker-compose)

| Service                      | URL                                          | Port |
| ---------------------------- | -------------------------------------------- | ---- |
| **ingestion-service**        | http://ingestion-service:8003/metrics        | 8003 |
| **event-processing-service** | http://event-processing-service:8001/metrics | 8001 |
| **notifications-service**    | http://notifications-service:8080/metrics    | 8080 |
| **keycloak**                 | http://keycloak:8080/metrics                 | 8080 |
| **kong**                     | http://kong:8000/metrics                     | 8000 |

### Local Development (services running on host)

Replace service name with `localhost` and use the local port:

| Service                      | URL                           | Port |
| ---------------------------- | ----------------------------- | ---- |
| **ingestion-service**        | http://localhost:8003/metrics | 8003 |
| **event-processing-service** | http://localhost:8001/metrics | 8001 |
| **notifications-service**    | http://localhost:8080/metrics | 8080 |

## Metrics Available

### Common Metrics (all services)

**HTTP Request Metrics:**

- `http_requests_total{method, path, status}` — Total request count
- `http_request_duration_seconds{method, path, status}` — Request latency histogram

**Process Metrics (Go Runtime):**

- `process_cpu_seconds_total` — Total CPU time
- `process_resident_memory_bytes` — Memory usage
- `go_goroutines` — Active goroutines
- `go_gc_duration_seconds` — Garbage collection duration

### Service-Specific Metrics

Custom application metrics can be added per service using the Prometheus Go client library. See each service's README for details.

## Useful PromQL Queries

### Request Rate (requests/sec)

```promql
rate(http_requests_total[5m])
```

### Request Latency (95th percentile)

```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### Memory Usage

```promql
process_resident_memory_bytes / 1024 / 1024  # in MB
```

### Error Rate

```promql
rate(http_requests_total{status=~"5.."}[5m])
```

### HTTP Requests by Service

```promql
sum(rate(http_requests_total[5m])) by (job)
```

## Configuration

**Location**: `observability/prometheus/prometheus.yml`

**Key settings:**

- `scrape_interval: 15s` — How often metrics are collected
- `scrape_timeout: 10s` — Time limit for scrape request
- `retention: 15d` — How long metrics are stored

To modify:

1. Edit `observability/prometheus/prometheus.yml`
2. Restart Prometheus: `docker compose restart prometheus`

## Troubleshooting

### Service shows as "DOWN" in targets

1. Check if service is running:

   ```bash
   docker compose ps
   ```

2. Verify `/metrics` endpoint is accessible:

   ```bash
   curl http://localhost:8003/metrics  # for ingestion-service
   ```

3. Check Prometheus logs:
   ```bash
   docker compose logs prometheus
   ```

### No data in Prometheus

- Metrics take time to accumulate — wait for at least one scrape interval (15s)
- Check if services are actually being hit (generate some traffic first)
- Verify services are running and metrics endpoint is responding

### High memory usage

- Reduce `retention` in `prometheus.yml` (currently 15 days)
- Lower `scrape_interval` to reduce data volume
- Disable high-cardinality metrics if needed

## Adding Custom Metrics

Each service can expose custom application metrics. Example:

```go
package main

import "github.com/prometheus/client_golang/prometheus"

var (
    eventsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "events_processed_total",
            Help: "Total events processed",
        },
        []string{"tenant_id", "status"},
    )
)

func init() {
    prometheus.MustRegister(eventsProcessed)
}
```

Then increment the metric:

```go
eventsProcessed.WithLabelValues(tenantID, "success").Inc()
```

See `github.com/prometheus/client_golang` for examples.

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [PromQL Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Go Prometheus Client Library](https://github.com/prometheus/client_golang)
- [Service-specific README files](../) for implementation details
