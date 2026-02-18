package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatched = "unmatched"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vulcan_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vulcan_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// metricsMiddleware records request count and duration for every HTTP request.
// Uses the chi route pattern (not the raw path) to avoid unbounded cardinality.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		path := routePattern(r)
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// routePattern extracts the matched chi route pattern, falling back to "unmatched".
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	return unmatched
}

// metricsHandler returns the Prometheus metrics handler.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}
