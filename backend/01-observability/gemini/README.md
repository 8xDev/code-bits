# Go Observability & Monitoring Project

This project is a minimal, fully functional Go CRUD application designed to demonstrate a modern observability stack. It covers metrics, logs, traces, and rate limiting using a practical, easy-to-understand example.

The entire stack is containerized and orchestrated with Docker Compose, making it runnable out-of-the-box.

---

## Architecture Overview

The project consists of two main parts: the **Application Stack** and the **Observability Stack**.

### Application Stack

- **go-app**: A Go REST API built with the Chi router.  
- **postgres**: PostgreSQL database for persistent storage.  
- **redis**: Redis for caching and rate limiting.  

### Observability Stack

- **prometheus**: Collects and stores all metrics.  
- **pg_exporter**: Exports PostgreSQL metrics to Prometheus.  
- **redis_exporter**: Exports Redis metrics to Prometheus.  
- **loki**: Collects and stores structured logs from the go-app.  
- **otel-collector**: The OpenTelemetry Collector receives traces from the go-app, processes them, and exports them to Tempo and Loki.  
- **tempo**: A high-volume, minimal-dependency distributed tracing backend.  
- **grafana**: The visualization layer. It connects to Prometheus (metrics), Loki (logs), and Tempo (traces) to provide a single-pane-of-glass dashboard.  

---

## Core Observability Concepts

This project demonstrates the "three pillars" of observability.

### 1. Metrics (The "What")

Metrics are quantitative, numeric measurements of the system's health over time. They are aggregated and efficient.

**How:** The project uses `prometheus/client_golang` to create custom metrics (counters, histograms).

**Examples in this project:**

- `app_http_requests_total`: A counter for HTTP requests (labeled by method, path, status code).  
- `app_http_request_duration_seconds`: A histogram of HTTP request latency.  
- `app_db_request_duration_seconds`: A histogram of database query latency (labeled by operation).  
- `app_rate_limiter_requests_total`: Counter for all requests hitting the rate limiter.  
- `app_rate_limiter_blocked_total`: Counter for requests blocked by the rate limiter (labeled by IP).  

**View:** In Grafana (Prometheus data source) or directly at [http://localhost:9090](http://localhost:9090).

---

### 2. Logs (The "Why")

Logs are structured, timestamped text records of specific events. They provide context for why something happened.

**How:** Uses Go's standard `slog` library with a `slog.NewJSONHandler` to write structured JSON logs to stdout.

**Collection:** The `docker-compose.yml` file configures the Loki logging driver for the go-app service. This driver automatically captures stdout and ships it directly to Loki, tagging it with `job=go-app`.

**View:** In Grafana (Loki data source). You can query logs using LogQL, for example:
```

{job="go-app"} | json | level="ERROR"

````

---

### 3. Traces (The "Where")

Traces show the end-to-end flow of a single request as it moves through different services and components. They are excellent for pinpointing latency and errors in a distributed system.

**How:** Uses the OpenTelemetry Go SDK.

- `otelhttp` middleware automatically traces all incoming HTTP requests.  
- Manually created child spans for database operations (e.g., `db.CreateTask`) show how much time is spent in the database vs. the application logic.  

**Collection:** The app exports traces via gRPC to the `otel-collector` (`otel-collector:4317`), which then batches and forwards them to Tempo.

**View:** In Grafana (Tempo data source).

---

## Correlation: Tying it all together

The real power comes from correlating these signals. The Grafana setup is pre-configured to:

- **Traces to Logs:** From a Tempo trace, you can one-click pivot to see all Loki logs associated with that specific `trace_id`.  
- **Logs to Traces:** JSON logs automatically include the `trace_id`, so you can copy it from a log line and search for it in Tempo.  

---

## Rate Limiting Implementation

This project includes a practical, Redis-based rate limiter to demonstrate how to protect your API and observe the impact.

**Strategy:** Fixed Window Counter. Allows 10 requests per 10-second window per client IP address. Simple to implement and demonstrate.

**Implementation:** The `rate_limiter.go` middleware:

1. Gets the client's IP address from the request.  
2. Uses `redis.Incr()` on a key like `rate_limit:<ip>`.  
3. If it's a new key, sets a 10-second expiry (`redis.Expire()`).  
4. If the count > 10, increments the `app_rate_limiter_blocked_total` metric and returns HTTP 429 Too Many Requests.  
5. If the count ≤ 10, increments `app_rate_limiter_requests_total` and passes the request to the next handler.  

**Observability:** The “Go App Observability” dashboard has a dedicated “Rate Limiting” section showing:

- A graph of allowed vs. blocked requests over time.  
- A table of the “Top 5 Blocked IPs” to identify noisy clients.  

**Alerting:** A Prometheus alert (`HighRateLimitBlocking`) fires if the rate of blocked requests is sustained, notifying you of potential DDoS activity or a misbehaving client.  

---

## Setup & Running

### Prerequisites

- Docker  
- Docker Compose  

### Instructions

1. Clone this repository (or save all files from this block).  
2. Rename `.env.example` to `.env`. The default values will work.
   ```bash
   mv .env.example .env
   ````

3. Build and start all services in detached mode:
   ```bash
   docker compose up -d --build
   ```

4. Wait a minute for all services to start and health checks to pass.

---

## How to Use & Observe

### Grafana (The Main Dashboard)

* **URL:** [http://localhost:3000](http://localhost:3000)
* **Login:** `admin / admin` (from `.env`)

Navigate to **Dashboards → Go App Observability**.

You will see panels for Metrics, Rate Limiting, Logs, and Traces all in one place.

---

### Prometheus

* **URL:** [http://localhost:9090](http://localhost:9090)
* Explore metrics (e.g., `app_http_requests_total`) or check Alerts for pending/firing alerts.

---

### Tempo (via Grafana)

In Grafana, go to the **Explore** tab and select the **Tempo** data source to search for traces.

---

## Example API Usage

Use `curl` or Postman to interact with the API.

### 1. Create a Task

```bash
curl -v -X POST http://localhost:8080/api/v1/tasks \
-d '{"title": "Learn Observability"}'
```

### 2. Get All Tasks

```bash
curl -v http://localhost:8080/api/v1/tasks
```

**Observe:** In the Grafana dashboard, you will see the request rate, latency, and logs for this request.

### 3. Get a Specific Task

```bash
curl -v http://localhost:8080/api/v1/tasks/1
```

### 4. Update a Task

```bash
curl -v -X PUT http://localhost:8080/api/v1/tasks/1 \
-d '{"title": "Master Observability", "completed": true}'
```

### 5. Delete a Task

```bash
curl -v -X DELETE http://localhost:8080/api/v1/tasks/1
```

---

### Triggering the Rate Limiter

This command will send 15 requests in a loop. The first 10 should return HTTP 200, and the last 5 should return HTTP 429.

```bash
for i in $(seq 1 15); do
  curl -s -o /dev/null -w "Request $i: %{http_code}\n" \
  -X GET http://localhost:8080/api/v1/tasks
done
```

**Observe:**
In the "Rate Limiting" panel in Grafana, you will see the `app_rate_limiter_blocked_total` metric spike for your IP.
The "Top 5 Blocked IPs" table will update.
The `HighRateLimitBlocking` alert in Prometheus will eventually move to a "Pending" or "Firing" state.

---

## Troubleshooting

* **"Service 'x' failed to start":**
  Check logs using:

  ```bash
  docker compose logs <service_name>
  ```

* **Grafana Dashboards Missing:**
  Wait 30–60 seconds on first boot for Grafana to provision datasources and dashboards.

* **No Data in Grafana:**
  Ensure Prometheus can scrape its targets.
  Visit [http://localhost:9090/targets](http://localhost:9090/targets) and check that all services (go-app, pg_exporter, etc.) are "UP".
