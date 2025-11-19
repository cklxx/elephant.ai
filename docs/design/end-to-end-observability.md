# End-to-End Observability & Performance Instrumentation
> Last updated: 2025-02-14

## Executive Summary
The Alex server now exposes an end-to-end observability surface that spans the HTTP ingress, Server-Sent Events (SSE) streaming loop, long-running background tasks, and the browser runtime. The design concentrates on two pillars:

1. **Canonicalized telemetry for every hop** – requests, streams, and tasks share the same route- and session-aware labels so traces and metrics can be correlated with low-cardinality tags.
2. **Actionable feedback loops** – Prometheus-friendly metrics, OTLP-compatible traces, and Web Vitals Real User Monitoring (RUM) signals flow into the same collector so regressions are easy to spot and triage.

## TODO
- [x] **Finalize configuration samples** – `configs/observability.example.yaml` is now tracked in git and mirrors every field consumed by `observability.LoadConfig`, so new environments can copy it verbatim and tweak endpoints without spelunking source code.
- [x] **Define dashboard/alert baselines** – the “HTTP + SSE Operations” and “Browser Web Vitals” Grafana boards (documented in the new *Observability Dashboards & Alerts* section) outline required panels plus starter alert rules for latency, disconnect storms, and Web Vitals regressions.
- [x] **Backfill integration tests** – new regression tests exercise canonical route propagation inside `ObservabilityMiddleware`, verify SSE write failures emit `write_error` metrics, and assert background task errors flip the `status=error` histogram label so instrumentation does not silently regress.
- [x] **RUM privacy review** – the Privacy Review section captures approved attributes, storage duration, and mitigation steps for `/api/metrics/web-vitals`, providing an auditable record for future changes.

## Goals & Non-Goals
| Goal | Description |
| --- | --- |
| Unified provider bootstrap | Instantiate logging, metrics, and tracing once in the server binary and inject the handle everywhere (HTTP router, SSE handler, background coordinator). |
| Canonical HTTP + SSE instrumentation | Capture latency, response sizes, connection lifetimes, and per-event delivery results using normalized route labels to prevent metric cardinality blow-ups. |
| Task lifecycle spans | Every asynchronous task execution should emit spans/metrics so slow or failing jobs surface immediately. |
| Frontend RUM ingestion | Accept browser `reportWebVitals` payloads without authentication and convert them into labeled metrics to close the loop between client UX and backend health. |

| Non-Goal | Rationale |
| --- | --- |
| Replace existing logging | Structured logs remain unchanged; observability augments them instead of adding a new log pipeline. |
| Provide alerting rules | Alerting is left to the infrastructure team once the new Prometheus series land. |

## Architecture Overview
```
Browser reportWebVitals ───────────────▶ /api/metrics/web-vitals
                                         │              │
HTTP clients ─▶ Router ─▶ Obs middleware ├─▶ Tracer spans┤
SSE clients  ─▶ SSE handler──────────────┘              ▼
Background tasks ─▶ ServerCoordinator instrumentation ─▶ MetricsCollector (Prometheus) + TracerProvider (OTLP)
```

### Reference configuration
*Sample artifact:* `configs/observability.example.yaml`

The tracked example mirrors every field consumed by `observability.LoadConfig`—logging verbosity/format, Prometheus enablement, and tracing exporter knobs—so ops teams can drop it into Kubernetes secrets or docker-compose overrides without reverse engineering Go structs.【F:configs/observability.example.yaml†L1-L23】 The sample defaults to OTLP tracing with a moderate sampling rate (20%) and exposes Prometheus metrics on port `9464`, matching the rollout guidance in this document.

### Observability dashboards & alert baselines
*Detailed runbook:* see the new "Observability Dashboards & Alerts" section inside `docs/operations/monitoring_and_metrics.md` for the canonical Grafana JSON snippets and recommended Prometheus alert expressions.【F:docs/operations/monitoring_and_metrics.md†L18-L120】 Highlights:

1. **HTTP + SSE Operations dashboard** – overlays `alex.http.latency`, `alex.http.requests.total`, and `alex.sse.connections.active` per canonical route, including a disconnect-spike alert that fires when `alex.sse.connection.duration` p95 drops below 10s for five minutes.
2. **Background Task Health dashboard** – tracks `alex.sessions.active`, `alex.tasks.execution.duration`, and failure ratios with alerts when `status="error"` exceeds 5% of executions in five minutes.
3. **Browser Web Vitals dashboard** – plots `alex.frontend.web_vital{name="LCP"}` vs. backend route latency and contains alerts for sustained `LCP` > 4s or `CLS` > 0.25.

### Integration test coverage
To keep the instrumentation honest, two new regression tests land with this update:

* `internal/server/http/middleware_test.go` simulates a request that overwrites its canonical route inside a handler and asserts `RecordHTTPServerRequest` receives the annotated template instead of the raw path, preventing silent route label regressions.
* `internal/server/http/sse_handler_test.go` injects a writer that fails during the initial `connected` event to verify we emit an `event_type=connected status=write_error` metric sample when handshakes break.
* `internal/server/app/server_coordinator_test.go` exercises the asynchronous failure path by forcing the agent coordinator to return an error and asserting the task execution metric records `status=error` alongside the span lifecycle.

### Web Vitals privacy review
Security and privacy teams approved collecting the following attributes: `name`, `value`, `delta`, `label`, `page`, `navigationType`, and `timestamp`. Personally identifiable data (user IDs, session tokens) remains banned, and the ingestion endpoint still rejects unknown JSON fields to prevent drift. Metrics flow into Prometheus with a 30-day retention cap enforced at the scraper, and only aggregated histograms are exported off-cluster. Any schema changes must undergo the same review before deployment.

* `cmd/alex-server/main.go` instantiates `observability.New` exactly once (based on the `ALEX_OBSERVABILITY_CONFIG` path) and wires it into the coordinator and router so every downstream component receives the same provider instance for spans + metrics + shutdown hooks.【F:cmd/alex-server/main.go†L79-L296】
* `internal/observability/observability.go` loads the YAML config, builds the logger, metrics collector, and tracer provider, and exposes a shared shutdown method so the main process can flush exporters before exiting.【F:internal/observability/observability.go†L8-L80】

## Component Design
### 1. Observability Provider Lifecycle
* Boot is controlled by `observability.New`, which reads the config path, configures log level/format, initializes Prometheus metrics, and sets the OpenTelemetry tracer provider. Failures degrade gracefully by swapping in noop collectors, keeping startup resilient in constrained environments.【F:internal/observability/observability.go†L16-L58】
* The server binary keeps the provider pointer (`obs`) and passes it into the `ServerCoordinator`, HTTP router, and SSE handler so all downstream call sites share the same meter/tracer state.【F:cmd/alex-server/main.go†L247-L296】
* Graceful shutdown is centralized: the `main` package defers a 5-second timeout context that calls `obs.Shutdown`, which drains the Prometheus HTTP server and tracer exporter cleanly before terminating.【F:cmd/alex-server/main.go†L83-L102】

### 2. Metrics Collector & Schema
* `internal/observability/metrics.go` defines typed instruments for LLM throughput, tool invocations, sessions, HTTP request latency/response size, SSE connection lifetimes and message bytes, background task executions, and Web Vitals values. Instruments use meters scoped to the `alex` namespace and register histograms/counters with human-readable descriptions and units for Prometheus scraping.【F:internal/observability/metrics.go†L64-L266】
* Helper methods such as `RecordHTTPServerRequest`, `RecordSSEMessage`, `RecordTaskExecution`, and `RecordWebVital` encapsulate attribute assignment so callers only provide semantic values (method, canonical route, status, etc.). This keeps label usage consistent and prevents duplication.【F:internal/observability/metrics.go†L357-L449】
* The collector optionally starts an embedded `/metrics` server when `prometheus_port` is configured, making it trivial to plug the binary into Kubernetes or docker-compose scrapers.【F:internal/observability/metrics.go†L268-L305】

### 3. HTTP Ingress Instrumentation
* `internal/server/http/router.go` wraps every mux handler with a `routeHandler` helper that stamps the canonical route template into the request context before the middleware executes. Nested handlers (tasks, sessions, auth) call `annotateRequestRoute` again when branching to sub-resources (e.g., `/api/sessions/:session_id/turns/:turn_id`).【F:internal/server/http/router.go†L14-L185】
* `ObservabilityMiddleware` records start time, wraps the `ResponseWriter` with a byte-counting recorder, starts an `alex.http.request` span, and, upon completion, emits a histogram/counter sample with `http.method`, resolved canonical route, status, latency, and response bytes. Canonicalization collapses numeric IDs and UUID-like slugs into `:id` placeholders to avoid unbounded label values.【F:internal/server/http/middleware.go†L234-L330】
* The middleware also propagates any canonical route updates that downstream handlers set (via `annotateRequestRoute`), ensuring SSE, session snapshots, and other sub-resources share the right labels without duplicating instrumentation logic.【F:internal/server/http/middleware.go†L234-L278】

### 4. SSE Streaming Visibility
* The SSE handler accepts an `WithSSEObservability` option so it can increment/decrement active connections, record connection duration histograms, and wrap each stream in an `alex.sse.connection` span annotated with the session ID and close reason. Span closing is deferred so errors propagate cleanly.【F:internal/server/http/sse_handler.go†L24-L200】
* Every outbound SSE payload (initial handshake, history replay, live events) goes through a helper that tracks serialization errors, write failures, and successful deliveries via `RecordSSEMessage`, tagging each sample with the event type and status (`ok`, `write_error`, etc.).【F:internal/server/http/sse_handler.go†L122-L200】【F:internal/observability/metrics.go†L381-L418】
* Active connection gauges empower dashboards to alert on sudden disconnect storms, while message histograms help spot payload growth that could stall browsers.

### 5. Background Task Tracing
* The `ServerCoordinator` receives the observability provider through `WithObservability`. When executing tasks asynchronously, it starts a span for each `alex.session.solve_task` invocation, attaches session/task IDs as attributes, and ensures errors/cancellations set span status to `Error`.【F:internal/server/app/server_coordinator.go†L117-L339】
* Metrics hooks increment the active-session gauge when background work begins and automatically decrement it plus record a `task_duration` histogram sample with a `status` label (`success`, `error`, `cancelled`). This lets operators correlate queue depth to task runtimes without manual bookkeeping.【F:internal/server/app/server_coordinator.go†L207-L339】【F:internal/observability/metrics.go†L341-L430】
* Task analytics remain intact: spans are additive and leverage the same broadcaster context, so there is no double counting or interference with SSE replay.

### 6. Browser Web Vitals Pipeline
* The HTTP API exposes `/api/metrics/web-vitals` as a lightweight, unauthenticated POST endpoint. Requests are bounded with `MaxBytesReader`, strict JSON decoding (unknown fields rejected), and canonical page normalization before writing the observation into the `alex.frontend.web_vital` histogram. Any missing `name` field surfaces as a `400` error to protect the metrics store.【F:internal/server/http/api_handler.go†L244-L308】
* The Next.js root layout exports `reportWebVitals` and uses either `navigator.sendBeacon` or a `keepalive` `fetch` to forward metrics with `credentials: 'omit'`, ensuring it cannot hang the browser unload event. Payloads include name, delta, page, navigation type, and timestamp for richer filtering downstream.【F:web/app/layout.tsx†L43-L77】
* Because the backend path bypasses auth middleware, it can receive metrics from anonymous browsers (staging/prod) without session tokens while still applying canonical route logic for aggregation.【F:internal/server/http/router.go†L47-L175】

## Metrics & Trace Catalogue
| Surface | Metric/Span | Key Attributes | Source |
| --- | --- | --- | --- |
| HTTP ingress | `alex.http.requests.total`, `alex.http.latency`, `alex.http.response.size`, `alex.http.request` span | `http.method`, canonical `http.route`, `http.status_code` | Middleware wrapping mux handlers.【F:internal/server/http/middleware.go†L234-L330】【F:internal/observability/metrics.go†L357-L379】 |
| SSE | `alex.sse.connections.active`, `alex.sse.connection.duration`, `alex.sse.messages.total`, `alex.sse.message.size`, `alex.sse.connection` span | `event_type`, `status`, `alex.session_id`, `alex.sse.close_reason` | SSE handler instrumentation.【F:internal/server/http/sse_handler.go†L24-L200】【F:internal/observability/metrics.go†L381-L418】 |
| Tasks | `alex.sessions.active`, `alex.tasks.executions.total`, `alex.tasks.execution.duration`, `alex.session.solve_task` span | `status`, `alex.session_id`, `alex.task_id` | Background coordinator.【F:internal/server/app/server_coordinator.go†L207-L339】【F:internal/observability/metrics.go†L341-L430】 |
| Web Vitals | `alex.frontend.web_vital` histogram | `name`, `label`, `page`, `delta` | `/api/metrics/web-vitals` ingestion + Next.js reporter.【F:internal/server/http/api_handler.go†L244-L308】【F:web/app/layout.tsx†L43-L77】 |

## Operational Considerations
1. **Cardinality control** – canonicalization (`/api/tasks/:task_id`) is enforced centrally; future routes must either use `routeHandler` or call `annotateRequestRoute` manually to avoid accidental blow-ups.【F:internal/server/http/router.go†L92-L185】【F:internal/server/http/middleware.go†L234-L307】
2. **Back-pressure** – SSE instrumentation exposes send failures, which combined with connection gauges can highlight when downstream clients fall behind. If consistent `write_error` spikes appear, consider buffering policies or heartbeat throttling.【F:internal/server/http/sse_handler.go†L122-L200】
3. **Privacy** – Web Vitals payloads omit user identifiers and rely on canonical page paths; if future metrics include PII, extend the handler to validate allowed pages or require signed tokens.
4. **Exporter pluggability** – tracing exporters currently support OTLP and Zipkin, selectable via config; metrics rely on the OpenTelemetry Prometheus exporter, so hooking Grafana Agent/OTLP metrics later only requires swapping the SDK reader.【F:internal/observability/tracing.go†L16-L112】【F:internal/observability/metrics.go†L64-L118】

## Rollout Plan
1. **Config rollout** – ship the new `ALEX_OBSERVABILITY_CONFIG` file with metrics/tracing toggles per environment; start with metrics-only in staging to validate cardinality, then enable tracing exporters.
2. **Dashboarding** – create Grafana dashboards for HTTP, SSE, and task panels using the catalogued metrics; add a Web Vitals panel to overlay backend latency vs. user-perceived metrics.
3. **Alerting** – once baselines exist, add alerts for high HTTP latency per route, low SSE connection counts, or Web Vitals regressions.
4. **Instrumentation debt tracking** – new routes/handlers must call `routeHandler` or `annotateRequestRoute`; add a codeowners checklist to ensure coverage before merging changes touching `internal/server/http`.

This design document should serve as the reference for future observability enhancements and onboarding guides for operators building dashboards/alerts atop the emitted telemetry.
