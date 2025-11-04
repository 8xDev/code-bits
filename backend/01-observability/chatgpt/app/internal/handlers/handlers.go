package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/trace"
)

// Simple Task entity
type Task struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

type Handler struct {
	db     *sql.DB
	redis  *redis.Client
	tracer trace.Tracer
}

var (
	reqs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by path and method",
	}, []string{"path", "method", "code"})

	reqLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_latency_seconds",
		Help:    "Latency in seconds for HTTP requests",
		Buckets: prometheus.DefBuckets,
	})
)

func New(db *sql.DB, redisAddr string, tracer trace.Tracer) *Handler {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &Handler{
		db:     db,
		redis:  rdb,
		tracer: tracer,
	}
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// metrics and timing
	timer := prometheus.NewTimer(reqLatency)
	defer timer.ObserveDuration()
	// read from DB
	rows, err := h.db.QueryContext(r.Context(), "SELECT id, title, content, done FROM tasks ORDER BY id")
	if err != nil {
		observeRequest("/tasks", r.Method, http.StatusInternalServerError)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.Done); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}
	observeRequest("/tasks", r.Method, http.StatusOK)
	writeJSON(w, tasks)
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(reqLatency)
	defer timer.ObserveDuration()

	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		observeRequest("/tasks", r.Method, http.StatusBadRequest)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	err := h.db.QueryRowContext(r.Context(), "INSERT INTO tasks (title, content, done) VALUES ($1, $2, $3) RETURNING id", t.Title, t.Content, t.Done).Scan(&t.ID)
	if err != nil {
		observeRequest("/tasks", r.Method, http.StatusInternalServerError)
		http.Error(w, "db insert error", http.StatusInternalServerError)
		return
	}
	observeRequest("/tasks", r.Method, http.StatusCreated)
	writeJSONWithCode(w, t, http.StatusCreated)
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(reqLatency)
	defer timer.ObserveDuration()
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var t Task
	err := h.db.QueryRowContext(r.Context(), "SELECT id, title, content, done FROM tasks WHERE id=$1", id).Scan(&t.ID, &t.Title, &t.Content, &t.Done)
	if errors.Is(err, sql.ErrNoRows) {
		observeRequest("/tasks/{id}", r.Method, http.StatusNotFound)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		observeRequest("/tasks/{id}", r.Method, http.StatusInternalServerError)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	observeRequest("/tasks/{id}", r.Method, http.StatusOK)
	writeJSON(w, t)
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(reqLatency)
	defer timer.ObserveDuration()
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		observeRequest("/tasks/{id}", r.Method, http.StatusBadRequest)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	res, err := h.db.ExecContext(r.Context(), "UPDATE tasks SET title=$1, content=$2, done=$3 WHERE id=$4", t.Title, t.Content, t.Done, id)
	if err != nil {
		observeRequest("/tasks/{id}", r.Method, http.StatusInternalServerError)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		observeRequest("/tasks/{id}", r.Method, http.StatusNotFound)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	observeRequest("/tasks/{id}", r.Method, http.StatusOK)
	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(reqLatency)
	defer timer.ObserveDuration()
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	res, err := h.db.ExecContext(r.Context(), "DELETE FROM tasks WHERE id=$1", id)
	if err != nil {
		observeRequest("/tasks/{id}", r.Method, http.StatusInternalServerError)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		observeRequest("/tasks/{id}", r.Method, http.StatusNotFound)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	observeRequest("/tasks/{id}", r.Method, http.StatusOK)
	writeJSON(w, map[string]string{"status": "deleted"})
}

func observeRequest(path, method string, code int) {
	reqs.WithLabelValues(path, method, strconv.Itoa(code)).Inc()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	writeJSONWithCode(w, v, http.StatusOK)
}

func writeJSONWithCode(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
