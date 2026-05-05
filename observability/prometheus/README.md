# Prometheus Configuration

This directory contains the Prometheus configuration for the Pred system.

## Files

- **prometheus.yml** — Prometheus configuration file defining scrape targets, intervals, and job definitions

## Overview

Prometheus collects time-series metrics from all microservices at regular intervals (default: every 15 seconds).

### Monitored Services

| Service                  | Endpoint                                | Port |
| ------------------------ | --------------------------------------- | ---- |
| ingestion-service        | `ingestion-service:8003/metrics`        | 8003 |
| event-processing-service | `event-processing-service:8001/metrics` | 8001 |
| notifications-service    | `notifications-service:8080/metrics`    | 8080 |
| keycloak                 | `keycloak:8080/metrics`                 | 8080 |
| kong                     | `kong:8000/metrics`                     | 8000 |

## Accessing Prometheus

**UI**: http://localhost:9090

**Metrics endpoint**: http://localhost:9090/metrics

## Configuration Details

### Scrape Interval

- **Global**: 15 seconds
- Per-job overrides are not currently configured

### Storage

- **Retention**: 15 days (default)
- **Path**: `/prometheus` (in container)
- **Mounted on host**: `prometheus_data` volume

### Targets

All targets use the same scrape config:

- **Scheme**: HTTP
- **Path**: `/metrics`
- **Timeout**: 10 seconds (default)

## Modifying Configuration

To add or modify scrape targets:

1. Edit `prometheus.yml`
2. Add a new job under `scrape_configs`:
   ```yaml
   - job_name: "new-service"
     metrics_path: "/metrics"
     static_configs:
       - targets: ["service-name:port"]
   ```
3. Restart Prometheus:
   ```bash
   docker compose restart prometheus
   ```

Prometheus will automatically reload the configuration (no restart needed if HUP signal is configured, otherwise restart is required).

## Troubleshooting

**Prometheus won't start**:

- Check YAML syntax: `docker logs prometheus`
- Verify all referenced targets are resolvable

**Targets showing as "DOWN"**:

- Ensure services are running: `docker ps`
- Verify the `/metrics` endpoint is accessible
- Check network connectivity within the compose network

**No metrics appearing**:

- Services may take time to accumulate metrics (first scrape happens after scrape interval)
- Verify metrics endpoint returns data: `curl http://service:port/metrics`

## Performance Considerations

- **Scrape interval**: Lower values = more frequent updates but higher load
- **Retention**: 15 days on default; reduce if disk space is limited
- **Number of metrics**: High-cardinality labels (e.g., per-request data) can bloat storage

For details, see [../OBSERVABILITY.md](../OBSERVABILITY.md).
