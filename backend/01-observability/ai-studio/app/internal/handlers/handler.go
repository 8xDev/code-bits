package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go-crud-app/internal/models"
	"go-crud-app/internal/storage"
)

var tracer = otel.Tracer("handler")

type Handler struct {
	store *storage.Store
}

func NewHandler(store *storage.Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	// Start an OTel span for this handler operation
	ctx, span := tracer.Start(r.Context(), "handler.CreateTask")
	defer span.End()

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		slog.ErrorContext(ctx, "failed to decode request body", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := h.store.CreateTask(ctx, task.Title)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create task in store", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	task.ID = id
	span.SetAttributes(attribute.Int("task.id", id))

	slog.InfoContext(ctx, "task created successfully", "task_id", id)
	writeJSON(w, http.StatusCreated, task)
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler.GetTask")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("task.id", id))

	task, err := h.store.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "failed to get task from store", "task_id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler.ListTasks")
	defer span.End()

	tasks, err := h.store.ListTasks(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list tasks from store", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int("tasks.count", len(tasks)))
	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler.UpdateTask")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("task.id", id))

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task.ID = id
	if err := h.store.UpdateTask(ctx, task); err != nil {
		slog.ErrorContext(ctx, "failed to update task in store", "task_id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "task updated successfully", "task_id", id)
	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler.DeleteTask")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("task.id", id))

	if err := h.store.DeleteTask(ctx, id); err != nil {
		slog.ErrorContext(ctx, "failed to delete task from store", "task_id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "task deleted successfully", "task_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write json response", "error", err)
	}
}
