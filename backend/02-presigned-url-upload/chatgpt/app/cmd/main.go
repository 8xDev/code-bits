package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/example/posts-presign/internal/handlers"
	"github.com/example/posts-presign/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// logger
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	// config
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/posts_db?sslmode=disable")
	minioEndpoint := getenv("MINIO_ENDPOINT", "minio:9000")
	minioUser := getenv("MINIO_ROOT_USER", "minioadmin")
	minioPass := getenv("MINIO_ROOT_PASSWORD", "minioadmin")
	minioBucket := getenv("MINIO_BUCKET", "posts-media")
	publicBase := getenv("MINIO_PUBLIC_BASE", "http://localhost:9000")
	apiKey := getenv("API_KEY", "changeme_api_key_please_change")
	maxUploadMB := getenvInt("MAX_UPLOAD_MB", 10)
	allowedOrigins := getenv("CORS_ALLOWED_ORIGINS", "http://localhost:8080")

	// DB
	sql.Register("pgx", stdlib.GetDefaultDriver())
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("open db")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Msg("ping db")
	}

	// MinIO client
	minioClient, err := storage.NewMinioClient(minioEndpoint, minioUser, minioPass, minioBucket, publicBase)
	if err != nil {
		log.Fatal().Err(err).Msg("minio client")
	}

	h := handlers.New(db, minioClient, apiKey, maxUploadMB)

	r := chi.NewRouter()

	// CORS simple (allow origins from env, supports single origin or comma list)
	r.Use(corsMiddleware(strings.Split(allowedOrigins, ",")))
	// API key auth for write endpoints
	r.Use(h.AuthMiddleware)

	// static frontend and swagger
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	r.Get("/app.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/app.js")
	})
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/swagger.html")
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "openapi.yaml")
	})

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Upload initiation (returns presigned POST or PUT info)
		r.Post("/uploads/init", h.InitUpload)             // -> presigned POST or PUT
		// After client uploads directly to MinIO, create the Post record by referencing object key
		r.Post("/posts", h.CreatePostFromKey)             // create post (object_key required)
		r.Get("/posts", h.ListPosts)
		r.Get("/posts/{id}", h.GetPost)
		r.Delete("/posts/{id}", h.DeletePost)
	})

	addr := ":" + getenv("APP_PORT", "8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	log.Info().Str("addr", addr).Msg("starting app")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server stopped")
	}
}

func getenv(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}
func getenvInt(k string, d int) int {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	var x int
	_, err := fmt.Sscan(v, &x)
	if err != nil {
		return d
	}
	return x
}

func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	origins := make(map[string]struct{})
	for _, o := range allowed {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if len(origins) == 0 {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					if _, ok := origins[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}
				}
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-KEY")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			}
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
