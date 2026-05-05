@AGENTS.md

## Prometheus Metrics

The service exposes Prometheus metrics at `/metrics` using `github.com/prometheus/client_golang`. All standard Go runtime metrics are included (process CPU, memory, goroutines, etc.).

**Note**: The `/metrics` endpoint is already added to the router (see `api/metrics.go` and `api/router.go`). No custom application metrics are currently instrumented — add them as needed using the `prometheus` package.
