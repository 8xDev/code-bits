package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DB holds the database connection pool and dependencies
type DB struct {
	pool    *pgxpool.Pool
	metrics *Metrics
	tracer  trace.Tracer
}

// NewDB creates a new DB instance with metrics and tracing
func NewDB(pool *pgxpool.Pool, m *Metrics, tp trace.TracerProvider) *DB {
	return &DB{
		pool:    pool,
		metrics: m,
		tracer:  tp.Tracer("db"),
	}
}

// InitDB initializes the PostgreSQL connection pool
func InitDB(ctx context.Context, dsn string, m *Metrics, tp trace.TracerProvider) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// You could instrument pgx with otelpgx here if desired
	// config.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Ping the database
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return NewDB(pool, m, tp), nil

}

// Close closes the database connection pool
func (db *DB) Close(ctx context.Context) {
	db.pool.Close()
}

// Helper function to observe DB operations
func (db *DB) observe(ctx context.Context, operation string, fn func(context.Context) error) error {
	// Start OTel span
	ctx, span := db.tracer.Start(ctx, "db."+operation)
	defer span.End()

	// Start Prometheus timer
	timer := prometheus.NewTimer(db.metrics.DBRequestDurationHist.WithLabelValues(operation))
	defer timer.ObserveDuration()

	// Execute the database operation
	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "database operation failed", "operation", operation, "error", err)
	}

	return err

}

// === CRUD Operations ===
func (db *DB) CreateTask(ctx context.Context, title string) (Task, error) {
	var task Task
	query := `INSERT INTO tasks (title) VALUES ($1)  RETURNING id, title, completed, created_at;`

	err := db.observe(ctx, "create_task", func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, query, title).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	})

	return task, err

}

func (db *DB) GetTasks(ctx context.Context) ([]Task, error) {
	var tasks []Task
	query := `SELECT id, title, completed, created_at FROM tasks ORDER BY created_at DESC;`

	err := db.observe(ctx, "get_all_tasks", func(ctx context.Context) error {
		rows, err := db.pool.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var task Task
			if err := rows.Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt); err != nil {
				return err
			}
			tasks = append(tasks, task)
		}
		return rows.Err()
	})

	return tasks, err

}

func (db *DB) GetTask(ctx context.Context, id int) (Task, error) {
	var task Task
	query := `SELECT id, title, completed, created_at FROM tasks WHERE id = $1;`

	err := db.observe(ctx, "get_task", func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, query, id).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	})

	return task, err

}

func (db *DB) UpdateTask(ctx context.Context, id int, req UpdateTaskRequest) (Task, error) {
	var task Task
	// Build query dynamically - this is simple, for more complex
	// updates, a query builder or squirrel would be better.

	// For this demo, we'll just do a full update
	// A real app would use COALESCE or separate queries

	// We need to fetch the current task first to handle partial updates
	// This is not efficient, but simple for a demo.
	// A better way is dynamic SQL (e.g., SET title = COALESCE($1, title), completed = COALESCE($2, completed))

	currentTask, err := db.GetTask(ctx, id)
	if err != nil {
		return task, err
	}

	if req.Title != nil {
		currentTask.Title = *req.Title
	}
	if req.Completed != nil {
		currentTask.Completed = *req.Completed
	}

	query := `UPDATE tasks SET title = $1, completed = $2 WHERE id = $3
          RETURNING id, title, completed, created_at`

	err = db.observe(ctx, "update_task", func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, query, currentTask.Title, currentTask.Completed, id).Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt)
	})

	return task, err

}

func (db *DB) DeleteTask(ctx context.Context, id int) error {
	return db.observe(ctx, "delete_task", func(ctx context.Context) error {
		cmdTag, err := db.pool.Exec(ctx, "DELETE FROM tasks WHERE id = $1", id)
		if err != nil {
			return err
		}
		if cmdTag.RowsAffected() == 0 {
			// This should be a specific error, e.g., "not found"
			return pgx.ErrNoRows
		}
		return nil
	})
}
