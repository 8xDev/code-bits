
package metrics

import (
	"net/http"
	"strconv"
	"time"
	
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// http_requests_total: Counter for total number of HTTP requests
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "code"},
	)
	
	// http_request_duration_seconds: Histogram for request duration
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "code"},
	)
)

func RegisterMetrics() {
	// This function is just a placeholder to make it explicit that metrics are registered.
	// promauto handles registration automatically.
}

// MetricsMiddleware is a Chi middleware that records Prometheus metrics for each request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		
		defer func() {
			duration := time.Since(start)
			statusCode := ww.Status()
			
			// Record metrics
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(statusCode)).Inc()
			httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(statusCode)).Observe(duration.Seconds())
		}()
		
		next.ServeHTTP(ww, r)
	})
}
