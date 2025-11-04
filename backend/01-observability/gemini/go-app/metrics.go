package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all our custom Prometheus metrics
type Metrics struct {
	Registry                  *prometheus.Registry
	HTTPRequestCounter        *prometheus.CounterVec
	HTTPRequestDurationHist   *prometheus.HistogramVec
	DBRequestDurationHist     *prometheus.HistogramVec
	RateLimiterRequestCounter *prometheus.CounterVec
	RateLimiterBlockedCounter *prometheus.CounterVec
}

// NewMetrics initializes and registers all Prometheus metrics
func NewMetrics(namespace string) *Metrics {
	registry := prometheus.NewRegistry()

	// Register standard Go and Process collectors
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: registry,

		// HTTP Metrics
		HTTPRequestCounter: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status"}, // Labels
		),
		HTTPRequestDurationHist: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "Histogram of HTTP request latencies.",
				Buckets:   prometheus.DefBuckets, // Default buckets
			},
			[]string{"method", "path"},
		),

		// Database Metrics
		DBRequestDurationHist: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "db_request_duration_seconds",
				Help:      "Histogram of database query latencies.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation"}, // e.g., "create", "get", "get_all"
		),

		// Rate Limiter Metrics
		RateLimiterRequestCounter: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limiter_requests_total",
				Help:      "Total number of requests processed by the rate limiter.",
			},
			[]string{"client_ip"},
		),
		RateLimiterBlockedCounter: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limiter_blocked_total",
				Help:      "Total number of requests blocked by the rate limiter.",
			},
			[]string{"client_ip"},
		),
	}

	return m

}

// PrometheusHandler returns an http.Handler for the metrics endpoint
func (m *Metrics) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// MetricsMiddleware is a chi middleware to capture HTTP metrics
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to get status code
			ww := &responseWriterInterceptor{w, http.StatusOK}

			// Get the route pattern (e.g., /api/v1/tasks/{id})
			// This provides low-cardinality labels
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = "unknown"
			}

			defer func() {
				duration := time.Since(start)
				status := strconv.Itoa(ww.statusCode)

				// Record metrics
				m.HTTPRequestCounter.WithLabelValues(r.Method, routePattern, status).Inc()
				m.HTTPRequestDurationHist.WithLabelValues(r.Method, routePattern).Observe(duration.Seconds())
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}

}

// responseWriterInterceptor helps capture the status code
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
