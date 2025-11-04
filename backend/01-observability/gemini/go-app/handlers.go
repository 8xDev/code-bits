package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
)

// TaskHandler holds dependencies for task HTTP handlers
type TaskHandler struct {
	db     *DB
	tracer trace.Tracer
}

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(db *DB, tp trace.TracerProvider) *TaskHandler {
	return &TaskHandler{
		db:     db,
		tracer: tp.Tracer("handler"),
	}
}

// Helper to write JSON responses
func (h *TaskHandler) writeJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log the error
		slog.ErrorContext(r.Context(), "failed to write JSON response", "error", err)
		http.Error(w, `{"error": "Failed to write JSON response"}`, http.StatusInternalServerError)
	}
}

// Helper to write JSON errors
func (h *TaskHandler) writeJSONError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.writeJSON(w, r, status, map[string]string{"error": message})
}

// === Handlers ===

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.CreateTask")
	defer span.End()

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		h.writeJSONError(w, r, http.StatusBadRequest, "Title is required")
		return
	}

	task, err := h.db.CreateTask(ctx, req.Title)
	if err != nil {
		h.writeJSONError(w, r, http.StatusInternalServerError, "Failed to create task")
		return
	}

	h.writeJSON(w, r, http.StatusCreated, task)

}

func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.GetTasks")
	defer span.End()

	tasks, err := h.db.GetTasks(ctx)
	if err != nil {
		h.writeJSONError(w, r, http.StatusInternalServerError, "Failed to get tasks")
		return
	}

	h.writeJSON(w, r, http.StatusOK, tasks)

}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.GetTask")
	defer span.End()

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "Invalid task ID")
		return
	}

	task, err := h.db.GetTask(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			h.writeJSONError(w, r, http.StatusNotFound, "Task not found")
		} else {
			h.writeJSONError(w, r, http.StatusInternalServerError, "Failed to get task")
		}
		return
	}

	h.writeJSON(w, r, http.StatusOK, task)

}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.UpdateTask")
	defer span.End()

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == nil && req.Completed == nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "No fields to update")
		return
	}

	task, err := h.db.UpdateTask(ctx, id, req)
	if err != nil {
		if err == pgx.ErrNoRows {
			h.writeJSONError(w, r, http.StatusNotFound, "Task not found")
		} else {
			h.writeJSONError(w, r, http.StatusInternalServerError, "Failed to update task")
		}
		return
	}

	h.writeJSON(w, r, http.StatusOK, task)

}

func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.DeleteTask")
	defer span.End()

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		h.writeJSONError(w, r, http.StatusBadRequest, "Invalid task ID")
		return
	}

	err = h.db.DeleteTask(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			h.writeJSONError(w, r, http.StatusNotFound, "Task not found")
		} else {
			h.writeJSONError(w, r, http.StatusInternalServerError, "Failed to delete task")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)

}
