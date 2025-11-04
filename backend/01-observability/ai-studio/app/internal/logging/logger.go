
package logging

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"
	
	"github.com/go-chi/chi/v5/middleware"
)

// NewLogger creates a new slog.Logger with JSON output.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// NewStructuredLogger is a middleware that logs request details in a structured format.
func NewStructuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			
			// Use a custom context to pass logger down
			ctx := context.WithValue(r.Context(), "logger", logger)
			
			defer func() {
				logger.Info("request completed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"latency_ms", time.Since(t1).Milliseconds(),
					"bytes_written", ww.BytesWritten(),
					"request_id", middleware.GetReqID(r.Context()),
				)
			}()
			
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
