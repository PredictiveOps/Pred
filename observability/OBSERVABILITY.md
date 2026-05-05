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

## Configuration

Prometheus configuration is defined in [prometheus.yml](./prometheus/prometheus.yml). All services are configured with:

- **Scrape interval**: 15 seconds
- **Metrics path**: `/metrics`
- **Timeout**: 10 seconds (default)

### Scrape Targets

| Service                  | Host                     | Port | Endpoint   |
| ------------------------ | ------------------------ | ---- | ---------- |
| ingestion-service        | ingestion-service        | 8003 | `/metrics` |
| event-processing-service | event-processing-service | 8001 | `/metrics` |
| notifications-service    | notifications-service    | 8080 | `/metrics` |
| keycloak                 | keycloak                 | 8080 | `/metrics` |
| kong                     | kong                     | 8000 | `/metrics` |

## Running Prometheus

Prometheus is automatically started by `docker-compose`:

```bash
docker compose up -d
```

Access the Prometheus UI at: **http://localhost:9090**

To stop:

```bash
docker compose down
```

To stop and remove volumes:

```bash
docker compose down -v
```

## Prometheus UI Features

### Querying Metrics

1. **Graph Tab** — Execute PromQL queries and view results as time-series graphs
2. **Alerts Tab** — View alert status (if configured)
3. **Status Tab** — Check service configuration, targets, and rules

### Example Queries

View HTTP request rate (requests/sec):

```promql
rate(http_requests_total[5m])
```

View 95th percentile request latency:

```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

View memory usage by service:

```promql
process_resident_memory_bytes
```

View goroutine count:

```promql
go_goroutines
```

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

## Adding Custom Metrics

To add custom application metrics to a service:

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

4. **Verify the metric** is exported at `/metrics`

## Troubleshooting

### Service not appearing in Prometheus targets

1. Check `http://localhost:9090/targets` — ensure all services show "UP"
2. Verify the service is running (`docker ps`)
3. Verify the service exposes `/metrics` (curl to test)
4. Check Prometheus logs: `docker logs prometheus`

### Metrics not updating

1. Verify scrape interval in [prometheus.yml](./prometheus/prometheus.yml) — default is 15 seconds
2. Check if the service is actively serving metrics — curl `/metrics` directly
3. Check network connectivity between Prometheus and the service

### Prometheus consuming too much memory

1. Reduce retention: adjust `--storage.tsdb.retention.time` in `docker-compose.yml` (default: 15d)
2. Reduce scrape frequency: increase `scrape_interval` in [prometheus.yml](./prometheus/prometheus.yml)
3. Filter unnecessary metrics using `metric_relabel_configs` in [prometheus.yml](./prometheus/prometheus.yml)

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [PromQL Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Go Prometheus Client](https://github.com/prometheus/client_golang)
