package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go-crud-app/internal/models"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	tracer      = otel.Tracer("storage")
	ErrNotFound = errors.New("not found")
)

type Store struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewStore(db *sql.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
}

func (s *Store) CreateTask(ctx context.Context, title string) (int, error) {
	// Create a new span for this database operation
	ctx, span := tracer.Start(ctx, "db.CreateTask")
	defer span.End()

	var id int
	query := "INSERT INTO tasks (title) VALUES ($1) RETURNING id"
	err := s.db.QueryRowContext(ctx, query, title).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("could not create task: %w", err)
	}

	// Invalidate the list cache
	s.rdb.Del(ctx, "tasks:all")

	return id, nil
}

func (s *Store) GetTask(ctx context.Context, id int) (*models.Task, error) {
	// --- Cache Interaction Span ---
	ctx, cacheSpan := tracer.Start(ctx, "cache.GetTask")
	cacheKey := fmt.Sprintf("task:%d", id)
	cacheSpan.SetAttributes(attribute.String("cache.key", cacheKey))

	// 1. Try to get from Redis cache first
	val, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var task models.Task
		if err := json.Unmarshal([]byte(val), &task); err == nil {
			slog.DebugContext(ctx, "cache hit", "key", cacheKey)
			cacheSpan.SetAttributes(attribute.Bool("cache.hit", true))
			cacheSpan.End()
			return &task, nil
		}
	}
	slog.DebugContext(ctx, "cache miss", "key", cacheKey)
	cacheSpan.SetAttributes(attribute.Bool("cache.hit", false))
	cacheSpan.End()

	// --- Database Interaction Span ---
	ctx, dbSpan := tracer.Start(ctx, "db.GetTask")
	defer dbSpan.End()
	dbSpan.SetAttributes(attribute.Int("task.id", id))

	// 2. If cache miss, get from PostgreSQL
	var task models.Task
	query := "SELECT id, title, completed FROM tasks WHERE id = $1"
	err = s.db.QueryRowContext(ctx, query, id).Scan(&task.ID, &task.Title, &task.Completed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("could not get task: %w", err)
	}

	// 3. Set the result back into Redis cache for next time
	jsonData, _ := json.Marshal(task)
	s.rdb.Set(ctx, cacheKey, jsonData, 1*time.Minute)

	return &task, nil
}

func (s *Store) ListTasks(ctx context.Context) ([]*models.Task, error) {
	ctx, span := tracer.Start(ctx, "db.ListTasks")
	defer span.End()

	query := "SELECT id, title, completed FROM tasks ORDER BY id"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Completed); err != nil {
			return nil, fmt.Errorf("could not scan task row: %w", err)
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (s *Store) UpdateTask(ctx context.Context, task models.Task) error {
	ctx, span := tracer.Start(ctx, "db.UpdateTask")
	defer span.End()
	span.SetAttributes(attribute.Int("task.id", task.ID))

	query := "UPDATE tasks SET title = $1, completed = $2 WHERE id = $3"
	_, err := s.db.ExecContext(ctx, query, task.Title, task.Completed, task.ID)
	if err != nil {
		return fmt.Errorf("could not update task: %w", err)
	}

	// Invalidate caches
	s.rdb.Del(ctx, fmt.Sprintf("task:%d", task.ID))
	s.rdb.Del(ctx, "tasks:all")

	return nil
}

func (s *Store) DeleteTask(ctx context.Context, id int) error {
	ctx, span := tracer.Start(ctx, "db.DeleteTask")
	defer span.End()
	span.SetAttributes(attribute.Int("task.id", id))

	query := "DELETE FROM tasks WHERE id = $1"
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("could not delete task: %w", err)
	}

	// Invalidate caches
	s.rdb.Del(ctx, fmt.Sprintf("task:%d", id))
	s.rdb.Del(ctx, "tasks:all")

	return nil
}
