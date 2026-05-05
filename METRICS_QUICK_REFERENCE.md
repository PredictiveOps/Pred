# Metrics & Observability Quick Reference

## Commands

```bash
# Start monitoring stack
docker compose up -d

# View Prometheus targets
http://localhost:9090/targets

# Query metrics
http://localhost:9090/graph

# Check service metrics endpoint
curl http://localhost:8003/metrics  # ingestion-service
curl http://localhost:8001/metrics  # event-processing-service
curl http://localhost:8080/metrics  # notifications-service

# Restart Prometheus after config changes
docker compose restart prometheus

# View logs
docker compose logs prometheus
```

## Metrics Endpoints (Docker Compose)

| Service          | Endpoint                                       |
| ---------------- | ---------------------------------------------- |
| Ingestion        | `http://ingestion-service:8003/metrics`        |
| Event Processing | `http://event-processing-service:8001/metrics` |
| Notifications    | `http://notifications-service:8080/metrics`    |
| Keycloak         | `http://keycloak:8080/metrics`                 |
| Kong             | `http://kong:8000/metrics`                     |

## Metrics Endpoints (Local)

| Service          | Endpoint                        |
| ---------------- | ------------------------------- |
| Ingestion        | `http://localhost:8003/metrics` |
| Event Processing | `http://localhost:8001/metrics` |
| Notifications    | `http://localhost:8080/metrics` |

## Common PromQL Queries

| Query                                                                      | Purpose                 |
| -------------------------------------------------------------------------- | ----------------------- |
| `rate(http_requests_total[5m])`                                            | Request rate (req/sec)  |
| `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))` | 95th percentile latency |
| `process_resident_memory_bytes / 1024 / 1024`                              | Memory usage (MB)       |
| `go_goroutines`                                                            | Active goroutines       |
| `http_requests_total{status=~"5.."}`                                       | 5xx errors              |
| `sum(rate(http_requests_total[5m])) by (job)`                              | Requests by service     |

## Key Files

| File                                      | Purpose                                        |
| ----------------------------------------- | ---------------------------------------------- |
| `observability/prometheus/prometheus.yml` | Prometheus config & scrape targets             |
| `observability/prometheus/README.md`      | Prometheus setup guide                         |
| `observability/OBSERVABILITY.md`          | Full observability architecture                |
| `MONITORING.md`                           | Quick start & examples                         |
| `PROMETHEUS_IMPLEMENTATION.md`            | Implementation summary                         |
| `*/README.md`                             | Each service documents its `/metrics` endpoint |

## Troubleshooting

| Issue                      | Solution                                                       |
| -------------------------- | -------------------------------------------------------------- |
| Service shows "DOWN"       | Verify service running: `docker compose ps`                    |
| No metrics data            | Wait 15s+ for scrape; generate traffic first                   |
| Can't access `/metrics`    | Check service is running; curl endpoint directly               |
| Prometheus high memory     | Reduce retention in `prometheus.yml` or lower scrape frequency |
| Config changes not applied | Restart: `docker compose restart prometheus`                   |

## Adding Custom Metrics

```go
package main

import "github.com/prometheus/client_golang/prometheus"

var myMetric = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "my_metric_total",
        Help: "Description of metric",
    },
    []string{"label1", "label2"},
)

func init() {
    prometheus.MustRegister(myMetric)
}

// In code:
myMetric.WithLabelValues("value1", "value2").Inc()
```

## Standards

- **All services** expose `/metrics` on their HTTP port
- **Scrape interval**: 15 seconds
- **Retention**: 15 days
- **Metrics path**: `/metrics`
- **Go runtime metrics**: Automatically collected

See full documentation in [MONITORING.md](MONITORING.md) and [PROMETHEUS_IMPLEMENTATION.md](PROMETHEUS_IMPLEMENTATION.md).
