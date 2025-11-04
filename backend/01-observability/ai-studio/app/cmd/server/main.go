
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"go-crud-app/internal/handlers"
	"go-crud-app/internal/logging"
	"go-crud-app/internal/metrics"
	"go-crud-app/internal/storage"
	"go-crud-app/internal/tracing"
)

func main() {
	// === Setup Logger ===
	logger := logging.NewLogger()
	slog.SetDefault(logger)

	// === Setup Tracing ===
	tp, err := tracing.NewTracerProvider("go-crud-app")
	if err != nil {
		slog.Error("failed to create tracer provider", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			slog.Error("failed to shutdown tracer provider", "error", err)
		}
	}()
	slog.Info("Tracer provider created")

	// === Setup Database and Redis ===
	db := initDB()
	rdb := initRedis()
	store := storage.NewStore(db, rdb)
	handler := handlers.NewHandler(store)

	// === Setup Metrics ===
	metrics.RegisterMetrics()
	go func() {
		metricsRouter := chi.NewRouter()
		metricsRouter.Handle("/metrics", promhttp.Handler())
		slog.Info("Metrics server starting on :9091")
		if err := http.ListenAndServe(":9091", metricsRouter); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	// === Setup Chi Router and Middleware ===
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logging.NewStructuredLogger(logger)) // Custom logging middleware
	r.Use(middleware.Recoverer)
	r.Use(metrics.MetricsMiddleware) // Custom metrics middleware

	// CORS for frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// OTel HTTP middleware
	otelHandler := otelhttp.NewHandler(r, "http-server")

	// === Define Routes ===
	r.Route("/tasks", func(r chi.Router) {
		r.Post("/", handler.CreateTask)
		r.Get("/", handler.ListTasks)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", handler.GetTask)
			r.Put("/", handler.UpdateTask)
			r.Delete("/", handler.DeleteTask)
		})
	})
	
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to the Go CRUD App!"))
	})


	// === Start Server ===
	srv := &http.Server{
		Addr:    ":8080",
		Handler: otelHandler,
	}

	go func() {
		slog.Info("Server starting on port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// === Graceful Shutdown ===
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited properly")
}

func initDB() *sql.DB {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "user"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "appdb"),
	)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	
	// Ping to verify connection
	if err = db.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("Successfully connected to the database")
	return db
}

func initRedis() *redis.Client {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	
	// Ping to verify connection
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	
	slog.Info("Successfully connected to Redis")
	return rdb
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
