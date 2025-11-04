package main

import "time"

// Task represents the model for a task
type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTaskRequest defines the shape of the request body for creating a task
type CreateTaskRequest struct {
	Title string `json:"title"`
}

// UpdateTaskRequest defines the shape of the request body for updating a task
type UpdateTaskRequest struct {
	Title     *string `json:"title,omitempty"`
	Completed *bool   `json:"completed,omitempty"`
}
