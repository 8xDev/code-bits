
# Go CRUD Project with a Full Observability Stack

This project is a simple, minimal yet fully-functional Golang CRUD application designed to demonstrate a comprehensive Observability & Monitoring setup. It covers the three pillars of observability: **Metrics**, **Logs**, and **Traces**.

## Tech Stack

- **Backend**: Go (Golang) + `chi` router
- **Database**: PostgreSQL
- **Cache**: Redis
- **Containerization**: Docker Compose
- **Observability Stack**:
  - **Metrics**: Prometheus
  - **Logs**: Loki
  - **Traces**: OpenTelemetry Collector + Tempo
  - **Visualization**: Grafana
  - **Alerting**: Alertmanager
  - **Exporters**: `postgres_exporter`, `redis_exporter`

## Project Overview

The application provides a simple REST API for managing "Tasks". A task has an ID, title, and a completed status.

### Endpoints

- `POST /tasks`: Create a new task.
  - Body: `{"title": "My new task"}`
- `GET /tasks`: List all tasks.
- `GET /tasks/{id}`: Get a single task by its ID.
- `PUT /tasks/{id}`: Update a task.
  - Body: `{"title": "Updated title", "completed": true}`
- `DELETE /tasks/{id}`: Delete a task.

## How to Run

**Prerequisites:** Docker and Docker Compose.

1.  **Save all files:** Download or copy all the files from this explorer into a local directory with the same structure.

2.  **Run with Docker Compose:**
    Open a terminal in the project's root directory and run:
    ```bash
    docker-compose up --build
    ```
    This command will build the Go application's Docker image and start all services defined in `docker-compose.yml`.

3.  **Run Database Migrations:**
    In a new terminal window, run the migration script:
    ```bash
    docker-compose exec go-app go run ./cmd/migrate
    ```
    This will create the `tasks` table in the PostgreSQL database.

4.  **Interact with the API:**
    The Go application is available at `http://localhost:8080`. You can use `curl` or any API client to interact with it.

    **Example `curl` commands:**
    ```bash
    # Create a task
    curl -X POST -H "Content-Type: application/json" -d '{"title":"Learn Observability"}' http://localhost:8080/tasks

    # List tasks
    curl http://localhost:8080/tasks

    # Get a specific task (replace 1 with a valid ID)
    curl http://localhost:8080/tasks/1

    # Update a task
    curl -X PUT -H "Content-Type: application/json" -d '{"title":"Master Observability", "completed":true}' http://localhost:8080/tasks/1

    # Delete a task
    curl -X DELETE http://localhost:8080/tasks/1
    ```

## Accessing the Observability Stack

- **Grafana**: `http://localhost:3000` (login with user: `admin`, pass: `admin`)
- **Prometheus**: `http://localhost:9090`
- **Alertmanager**: `http://localhost:9093`

Upon logging into Grafana, you will find a pre-provisioned "Go Service Dashboard" that visualizes the metrics, logs, and traces from the application.

## Observability Deep Dive

### 1. Metrics (The "What")

Metrics are numerical representations of data measured over time. They tell you *what* is happening in your system.

- **How it's implemented:**
  - The Go app uses the `prometheus/client_golang` library.
  - Custom metrics (`http_requests_total`, `http_request_duration_seconds`) are registered.
  - A middleware in `main.go` intercepts every HTTP request to update these metrics.
  - The app exposes a `/metrics` endpoint, which Prometheus is configured to scrape.
- **Where to see it:**
  - **Go Service Dashboard** in Grafana: Panels for "Request Rate", "Error Rate (5xx)", and "Request Latency (p95)".
  - **Prometheus UI**: You can run custom PromQL queries.

### 2. Logs (The "Why")

Logs are immutable, timestamped records of discrete events. They provide the context and tell you *why* something happened.

- **How it's implemented:**
  - The Go app uses the standard library's `slog` package for structured (JSON) logging.
  - A logging middleware in `main.go` logs details for every request.
  - The Docker Compose setup uses the Loki logging driver to automatically send container logs to Loki.
- **Where to see it:**
  - **Go Service Dashboard** in Grafana: The "Logs" panel shows logs from the `go-app` service. You can filter by log level (e.g., `error`) and other labels.
  - **Grafana Explore**: You can query logs directly using LogQL.

### 3. Traces (The "Where")

Traces show the end-to-end journey of a request as it travels through different components of your system. They help you find *where* a problem or latency is occurring.

- **How it's implemented:**
  - The Go app is instrumented with the OpenTelemetry Go SDK.
  - A middleware creates a parent span for each incoming HTTP request.
  - Child spans are created for specific operations, like database queries (`db.Query`) and cache operations (`cache.Get`).
  - The app sends these traces to the OpenTelemetry Collector, which then forwards them to Tempo.
- **Where to see it:**
  - **Go Service Dashboard** in Grafana: The "Traces" panel shows recent traces.
  - **Trace to Logs Correlation**: When viewing a trace in Grafana, you can click on a span and see the logs that were generated during that specific span's execution. This is incredibly powerful for debugging.

### 4. Alerting

Alerts proactively notify you when something is wrong.

- **How it's implemented:**
  - Alerting rules are defined in `etc/prometheus/alert.rules.yml`. For example, an alert fires if the 5xx error rate is too high.
  - Prometheus continuously evaluates these rules. If a rule's condition is met, it fires an alert to Alertmanager.
  - Alertmanager can be configured to route these alerts to Slack, email, etc. (the provided config is minimal).
- **Where to see it:**
  - **Alertmanager UI**: Shows currently firing alerts.
  - **Grafana UI**: Can also be configured to display alerts from Prometheus.
