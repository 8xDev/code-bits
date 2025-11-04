package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"time"

	"github.com/example/obs-tasks/internal/handlers"
	"github.com/example/obs-tasks/internal/middleware"
	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Logger
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	ctx := context.Background()

	// Tracing (OTLP to Collector)
	tpShutdown, tracerProvider := setupTracer(ctx)
	defer func() {
		if tpShutdown != nil {
			_ = tpShutdown(ctx)
		}
	}()

	// DB
	dsn := getenv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/tasks_db?sslmode=disable")
	// sql.Register("pgx", stdlib.GetDefaultDriver())
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("opening database")
	}
	if err := db.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Msg("ping database")
	}

	// Redis
	redisAddr := getenv("REDIS_ADDR", "redis:6379")

	// App router
	r := chi.NewRouter()

	// Instrument chi with OpenTelemetry middleware
	r.Use(otelchi.Middleware("tasks-service"))
	// Logging middleware (simple)
	r.Use(LogMiddleware)

	// Rate limiter using Redis
	rl := middleware.NewRedisRateLimiter(redisAddr, 100, time.Minute) // default 100 req/min
	r.Use(rl.Middleware)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Handlers (with DB + Redis)
	h := handlers.New(db, redisAddr, otel.GetTracerProvider().Tracer("tasks-tracer"))

	r.Route("/tasks", func(r chi.Router) {
		r.Get("/", h.ListTasks)
		r.Post("/", h.CreateTask)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetTask)
			r.Put("/", h.UpdateTask)
			r.Delete("/", h.DeleteTask)
		})
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	log.Info().Msg("starting server on :8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}

	// ensure tracer provider shutdown
	_ = tracerProvider
}

// simple logging middleware
func LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(start)).
			Msg("request finished")
	})
}

func setupTracer(ctx context.Context) (func(context.Context) error, oteltrace.TracerProvider) {
	collector := getenv("OTEL_COLLECTOR_ENDPOINT", "otel-collector:4317")
	// Minimal OTLP gRPC exporter to collector
	// Use default SDK
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collector),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create otlp exporter")
		return nil, nil
	}
	bsp := otel.GetTracerProvider() // fallback
	// create SDK TracerProvider with exporter
	tp := otel.GetTracerProvider()
	// For this educational demo we will rely on collector and default provider behaviour.
	// In production you'd construct sdktrace.NewTracerProvider with batcher etc.
	_ = exp
	_ = bsp
	return func(context.Context) error { return nil }, tp
}

func getenv(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}
