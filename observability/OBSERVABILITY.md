# Observability — Prometheus & Metrics

## Overview

Prometheus is configured to collect metrics from all microservices. This document describes the metrics setup, how to access the Prometheus UI, and how to add custom metrics.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Microservices (all expose /metrics endpoint)                 │
├──────────────┬──────────────────┬──────────────────┬─────────┤
│ Ingestion    │ Event Processing │ Notifications    │ Keycloak│
│ :8003/metrics│ :8001/metrics    │ :8080/metrics    │ :8080   │
└──────────┬───┴────────┬─────────┴────────┬─────────┴────┬────┘
           │            │                  │              │
           └────────────┴──────────────────┴──────────────┘
                        │
                        ▼
           ┌──────────────────────────┐
           │     Prometheus           │
           │  (localhost:9090)        │
           │                          │
           │ - Scrape every 15s       │
           │ - 15d retention          │
           │ - TSDB storage           │
           └──────────────────────────┘
```

## Quick Start

1. **Start the stack**:

   ```bash
   docker compose up -d
   ```

2. **Access Prometheus**:
   - UI: http://localhost:9090
   - Targets: http://localhost:9090/targets — all services should show "UP"

3. **Stop**:

   ```bash
   docker compose down        # stop
   docker compose down -v     # stop and remove volumes
   ```

## Common Commands

```bash
# Start monitoring stack
docker compose up -d

# Check scrape targets
open http://localhost:9090/targets

# Query metrics interactively
open http://localhost:9090/graph

# Curl a service metrics endpoint directly
curl http://localhost:8003/metrics  # ingestion-service
curl http://localhost:8001/metrics  # event-processing-service
curl http://localhost:8080/metrics  # notifications-service

# Restart Prometheus after config changes
docker compose restart prometheus

# View Prometheus logs
docker compose logs prometheus
```

## Configuration

Prometheus configuration is defined in [prometheus/prometheus.yml](./prometheus/prometheus.yml). All services are configured with:

- **Scrape interval**: 15 seconds
- **Metrics path**: `/metrics`
- **Timeout**: 10 seconds (default)
- **Retention**: 15 days

To apply changes: edit `prometheus/prometheus.yml`, then `docker compose restart prometheus`.

### Scrape Targets

| Service                  | Host                     | Port | Endpoint   |
| ------------------------ | ------------------------ | ---- | ---------- |
| ingestion-service        | ingestion-service        | 8003 | `/metrics` |
| event-processing-service | event-processing-service | 8001 | `/metrics` |
| notifications-service    | notifications-service    | 8080 | `/metrics` |
| keycloak                 | keycloak                 | 8080 | `/metrics` |
| kong                     | kong                     | 8000 | `/metrics` |

## Metrics Endpoints

**Docker Compose (internal)**:

- `http://ingestion-service:8003/metrics`
- `http://event-processing-service:8001/metrics`
- `http://notifications-service:8080/metrics`
- `http://keycloak:8080/metrics`
- `http://kong:8000/metrics`

**Local development (host)**:

- `http://localhost:8003/metrics`
- `http://localhost:8001/metrics`
- `http://localhost:8080/metrics`

## Standard Metrics Exposed

All services that have integrated Prometheus expose the following metrics:

### HTTP Metrics

- `http_requests_total{method, path, status}` — Counter of total HTTP requests
- `http_request_duration_seconds{method, path, status}` — Histogram of request latencies

### Process Metrics (Go Runtime)

- `process_cpu_seconds_total` — Total CPU time consumed
- `process_resident_memory_bytes` — Resident memory size in bytes
- `go_goroutines` — Number of active goroutines
- `go_gc_duration_seconds` — GC stop-the-world duration

## Useful PromQL Queries

```promql
# Request rate (requests/sec)
rate(http_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Memory usage in MB
process_resident_memory_bytes / 1024 / 1024

# Error rate (5xx)
rate(http_requests_total{status=~"5.."}[5m])

# Requests by service
sum(rate(http_requests_total[5m])) by (job)

# Active goroutines
go_goroutines
```

## Adding Custom Metrics

1. **Import the Prometheus client library**:

   ```go
   import "github.com/prometheus/client_golang/prometheus"
   ```

2. **Define a metric** (e.g., in a `metrics.go` file):

   ```go
   var eventsProcessed = prometheus.NewCounterVec(
       prometheus.CounterOpts{
           Name: "events_processed_total",
           Help: "Total number of events processed",
       },
       []string{"tenant_id", "status"},
   )

   func init() {
       prometheus.MustRegister(eventsProcessed)
   }
   ```

3. **Instrument your code**:

   ```go
   eventsProcessed.WithLabelValues(tenantID, "success").Inc()
   ```

4. **Verify** the metric is exported at `/metrics`

## Implementation Status

| Item                             | Status         |
| -------------------------------- | -------------- |
| Prometheus configuration         | ✅ Complete    |
| ingestion-service metrics        | ✅ Active      |
| event-processing-service metrics | ✅ Implemented |
| notifications-service metrics    | ✅ Implemented |
| keycloak metrics                 | ✅ Configured  |
| kong metrics                     | ✅ Configured  |
| Prometheus UI                    | ✅ Working     |

## Troubleshooting

| Issue                         | Solution                                                                        |
| ----------------------------- | ------------------------------------------------------------------------------- |
| Service shows "DOWN"          | Verify it's running: `docker compose ps`; curl its `/metrics` endpoint directly |
| No metrics data               | Wait 15s+ for first scrape; generate traffic first                              |
| Can't access `/metrics`       | Check service is up; verify port binding                                        |
| Config changes not applied    | `docker compose restart prometheus`                                             |
| Prometheus high memory usage  | Reduce retention in `prometheus.yml` or increase `scrape_interval`              |

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [PromQL Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Go Prometheus Client](https://github.com/prometheus/client_golang)
