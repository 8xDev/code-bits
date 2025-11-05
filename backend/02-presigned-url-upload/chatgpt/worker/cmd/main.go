package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	port := getEnv("WORKER_PORT", "9002")
	r := mux.NewRouter()
	r.HandleFunc("/minio/events", minioEventsHandler).Methods("POST", "OPTIONS")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	log.Info().Str("addr", port).Msg("worker listening for minio events")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("worker stopped")
	}
}

func minioEventsHandler(w http.ResponseWriter, r *http.Request) {
	// MinIO webhook posts JSON arrays with event info; we accept anything and log
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	log.Info().Msgf("Received MinIO event payload: %s", string(body))

	// For demo: parse and log object info, in production trigger thumbnail generation, virus scan, etc.
	var events interface{}
	if err := json.Unmarshal(body, &events); err == nil {
		log.Info().Interface("event", events).Msg("parsed event")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func getEnv(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}
