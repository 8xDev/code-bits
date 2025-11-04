package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger creates a new structured JSON logger
func NewLogger() *slog.Logger {
	// Add trace_id and span_id to logs for correlation
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug, // Log debug levels
		AddSource: true,            // Add source file/line to logs
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})

	// Create a custom handler that adds trace context
	traceHandler := &TraceContextHandler{Handler: handler}
	return slog.New(traceHandler)

}

// TraceContextHandler is a slog.Handler middleware that adds trace context
type TraceContextHandler struct {
	slog.Handler
}

// Handle adds trace_id and span_id to the log record
func (h *TraceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

// Enabled checks if the underlying handler is enabled
func (h *TraceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs returns a new handler with the given attributes
func (h *TraceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group
func (h *TraceContextHandler) WithGroup(name string) slog.Handler {
	return &TraceContextHandler{Handler: h.Handler.WithGroup(name)}
}

// LoggingMiddleware is a chi middleware for structured logging
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()

			// Get context for logging (it has trace info)
			ctx := r.Context()

			// Log the start of the request
			logger.InfoContext(ctx, "request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"request_id", middleware.GetReqID(ctx),
			)

			defer func() {
				// Log the end of the request
				logger.InfoContext(ctx, "request finished",
					"status", ww.Status(),
					"bytes_written", ww.BytesWritten(),
					"duration_ms", time.Since(t1).Milliseconds(),
				)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}

}
