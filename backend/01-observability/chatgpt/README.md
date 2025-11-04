# Observability & Monitoring: Tasks management app

**Purpose:** educational sample showing a small Golang CRUD app with *metrics*, *logs*, *traces*, and *rate limiting*, tied together with Prometheus, Loki, OpenTelemetry Collector, and Grafana. The stack is orchestrated with `docker compose`.

---

## Table of Contents
- Overview & Architecture
- What's included
- Running the project
- Endpoints & Example requests
- Observability details (metrics, logs, traces)
- Rate limiting design & metrics
- Grafana dashboards & alerts
- Troubleshooting

---

## Overview & Architecture

Services included (via `docker-compose`):
- `app` — Golang tasks REST API (chi router)
- `postgres` — data store with migrations
- `redis` — used for centralized rate limiting
- `prometheus` — metrics scraping and alerting
- `grafana` — dashboards and log/metric visualization
- `loki` — centralized logging backend
- `otel-collector` — collects and forwards traces/metrics
- `postgres-exporter`, `redis-exporter` — DB/Redis metrics for Prometheus

Flow:
- App instruments metrics (Prometheus), logs (structured JSON to stdout), and traces (OpenTelemetry OTLP -> Collector).
- Prometheus scrapes `/metrics` on the app, exporters, and the Collector.
- Loki ingests application logs (via Grafana Loki).
- Grafana uses Prometheus & Loki to visualize metrics and logs; dashboards are pre-provisioned.

---

## What's included
All files required to run:
- `docker-compose.yml`
- `app/` — Golang app source and Dockerfile
- `migrations/` — SQL migrations to create and seed `tasks` table
- `observability/` — Prometheus, Grafana provisioning, Loki, OTEL Collector config
- `README.md` (this file)

---

## Quickstart

Pre-reqs:
- Docker & Docker Compose (v2 recommended)

Run:
```bash
docker compose up --build
````

* App: [http://localhost:8080](http://localhost:8080)
* Prometheus: [http://localhost:9090](http://localhost:9090)
* Grafana: [http://localhost:3000](http://localhost:3000) (admin/admin)
* Loki: [http://localhost:3100](http://localhost:3100)
* OTEL Collector metrics: [http://localhost:8889](http://localhost:8889) (Prometheus exposition by collector)

Migrations are applied by Postgres container initialization (see `migrations/001_create_tasks.sql`).

---

## REST Endpoints (Tasks)

* `GET /tasks` — list tasks
* `POST /tasks` — create task (JSON body: `{ "title": "...", "content":"...", "done": false }`)
* `GET /tasks/{id}` — get a task
* `PUT /tasks/{id}` — update a task
* `DELETE /tasks/{id}` — delete a task

Example `curl`:

```bash
# create
curl -X POST http://localhost:8080/tasks -H "Content-Type: application/json" -d '{"title":"test", "content":"desc", "done": false}'

# list
curl http://localhost:8080/tasks
```

---

## Observability — Concepts & How it's implemented

### Metrics

**Concept:** Quantitative measurements (counters, gauges, histograms).
**What's implemented:**

* `http_requests_total{path,method,code}` — counter per route.
* `http_request_latency_seconds` — histogram of latencies.
* `rate_limiter_requests_total` — counter of requests seen by rate limiter.
* `rate_limiter_blocked_total` — counter of requests blocked.
* Exporters: Postgres exporter and Redis exporter are running and scraped by Prometheus.

View metrics at: `http://localhost:9090/targets` and browse metrics at `http://localhost:9090/graph`.

### Logs

**Concept:** Structured events for debugging. Use JSON logs to correlate with traces and metrics.
**What's implemented:**

* App uses `zerolog` to emit structured logs (to stdout). Loki ingests logs.
* Use Grafana Explore -> Loki datasource to query logs.

### Traces

**Concept:** Distributed tracing yields request flows across services.
**What's implemented:**

* App uses OpenTelemetry middleware (chi) and sends OTLP to the Collector at `otel-collector:4317`.
* Collector can export to various backends (logging/sample). Grafana/Tempo integration can be added.

### Correlation

Use trace IDs (exposed in logs and traces) and metrics labels to link logs, traces, and metrics. The Otel middleware attaches trace context to request spans.

---

## Rate Limiting — Design & Rationale

**Design:** A simple fixed-window counter in Redis keyed by client IP:

* Key: `rl:{client-ip}`
* On each request `INCR` the key. If it's the first hit (value==1), set TTL to window (default 1 minute).
* If counter > limit (default `100` per minute), return HTTP 429.

**Rationale:** Redis centralization ensures limits work across multiple app replicas. Fixed-window is easy to understand and efficient for demo purposes. For production consider sliding window (token bucket) and per-user keys for authenticated flows.

**Configuration:**

* Limit & window are set in code (see `app/internal/middleware/rate_limiter.go`). You can make them configurable via env vars easily.

**Metrics emitted:**

* `rate_limiter_requests_total` — incremented for every incoming request evaluated by the limiter.
* `rate_limiter_blocked_total` — incremented when a request is blocked (HTTP 429).

**Grafana Panels:**

* Pre-provisioned panel shows `rate_limiter_requests_total` vs `rate_limiter_blocked_total`.
* Alerts defined in `observability/alert_rules/alerts.yml` fire when there is a spike in blocked requests.

---

## Alerts (Prometheus rules)

* `HighErrorRate` — high proportion of 5xx over total requests.
* `ExcessiveBlockedRequests` — spike in `rate_limiter_blocked_total`.

These live in `observability/alert_rules/alerts.yml`.

---

## Instrumentation Best Practices (notes)

* Use meaningful label dimensions (avoid high-cardinality labels).
* Keep histograms for latency. Choose buckets appropriate for your latency distribution.
* Fail-open rate limiter behavior: for safety the middleware allows requests if Redis is down — for demo purpose. In production you may fail-closed depending on requirements.
* Keep logs structured (JSON) with context fields (path, method, user-id, trace_id).

---

## Troubleshooting

1. **App cannot connect to Postgres**

   * Check `docker compose logs postgres` for errors.
   * Ensure Postgres container finished startup: `docker compose ps`.
2. **Redis errors**

   * Check `docker compose logs redis`. Ensure host in `DATABASE_URL`/`REDIS_ADDR` matches service name.
3. **Prometheus not scraping**

   * Visit `http://localhost:9090/targets` for target statuses.
4. **Grafana dashboards missing**

   * Check provisioning logs in Grafana container logs.
5. **Traces not visible**

   * Ensure `otel-collector` is running and reachable at `otel-collector:4317`. Collector logs show received traces.

---

## Extending this demo

* Add authentication and per-user rate limits.
* Use a token-bucket algorithm (sliding-window) for smoother limiting.
* Add Tempo for trace storage and link to Grafana.
* Add more detailed dashboards for Postgres slow queries.

---

## Notes

This project is intentionally small and educational. The instrumentation demonstrates how metrics, logs, traces, and rate limiting can be integrated from day one.
