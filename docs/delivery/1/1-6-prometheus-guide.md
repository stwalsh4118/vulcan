# Prometheus Client Guide

**Date**: 2026-02-18
**Package**: `github.com/prometheus/client_golang`
**Docs**: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus

## API Usage

### Counter with Labels

```go
counter := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total HTTP requests.",
    },
    []string{"method", "path", "status"},
)
prometheus.MustRegister(counter)
counter.WithLabelValues("GET", "/healthz", "200").Inc()
```

### Histogram with Labels

```go
histogram := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "Request duration in seconds.",
        Buckets: prometheus.DefBuckets,
    },
    []string{"method", "path"},
)
prometheus.MustRegister(histogram)
histogram.WithLabelValues("GET", "/healthz").Observe(0.005)
```

### Exposition Handler

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```
