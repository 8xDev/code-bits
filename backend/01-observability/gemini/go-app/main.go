package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	// === Setup ===
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Init Logger (Structured JSON)
	logger := NewLogger()
	slog.SetDefault(logger) // Set as default for any global logging

	// Init Tracer (OpenTelemetry)
	tracerProvider, shutdownTracer, err := InitTracer()
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			logger.ErrorContext(ctx, "failed to shutdown tracer", "error", err)
		}
	}()
	logger.InfoContext(ctx, "Tracer initialized")

	// Init Metrics (Prometheus)
	metrics := NewMetrics("go-app")

	// Init Database (PostgreSQL)
	db, err := InitDB(ctx, os.Getenv("POSTGRES_DSN"), metrics, tracerProvider)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close(ctx)
	logger.InfoContext(ctx, "Database initialized")

	// Init Cache (Redis)
	rdb, err := InitRedis(ctx, os.Getenv("REDIS_ADDR"))
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	logger.InfoContext(ctx, "Redis initialized")

	// === HTTP Server ===
	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "8080"
	}
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "8081"
	}

	// Handlers
	taskHandler := NewTaskHandler(db, tracerProvider)

	// === App Router (chi) ===
	r := chi.NewRouter()

	// Register middleware
	// OTel middleware must be first to trace the entire request
	r.Use(otelhttp.NewMiddleware(os.Getenv("OTEL_SERVICE_NAME")))
	r.Use(LoggingMiddleware(logger))
	r.Use(MetricsMiddleware(metrics))
	r.Use(RateLimiterMiddleware(rdb, metrics, logger))

	// Register routes
	r.Route("/api/v1/tasks", func(r chi.Router) {
		r.Post("/", taskHandler.CreateTask)
		r.Get("/", taskHandler.GetTasks)
		r.Get("/{id}", taskHandler.GetTask)
		r.Put("/{id}", taskHandler.UpdateTask)
		r.Delete("/{id}", taskHandler.DeleteTask)
	})

	srv := &http.Server{
		Addr:    ":" + appPort,
		Handler: r,
	}

	// === Metrics Server (Prometheus) ===
	metricsRouter := chi.NewRouter()
	metricsRouter.Handle("/metrics", metrics.PrometheusHandler())

	metricsSrv := &http.Server{
		Addr:    ":" + metricsPort,
		Handler: metricsRouter,
	}

	// === Start Servers ===
	go func() {
		logger.InfoContext(ctx, "Starting application server", "port", appPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "application server error", "error", err)
			cancel() // Trigger shutdown
		}
	}()

	go func() {
		logger.InfoContext(ctx, "Starting metrics server", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "metrics server error", "error", err)
			cancel() // Trigger shutdown
		}
	}()

	// === Graceful Shutdown ===
	<-ctx.Done() // Wait for interrupt signal

	logger.InfoContext(ctx, "Shutting down servers...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Shutdown servers
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "application server shutdown error", "error", err)
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "metrics server shutdown error", "error", err)
	}

	logger.InfoContext(shutdownCtx, "Servers shut down gracefully")

}
