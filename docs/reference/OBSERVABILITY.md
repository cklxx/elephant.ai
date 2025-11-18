# ALEX Observability Guide
> Last updated: 2025-11-18


This guide explains how to use ALEX's comprehensive observability features including structured logging, metrics, and distributed tracing.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Configuration](#configuration)
4. [Structured Logging](#structured-logging)
5. [Metrics](#metrics)
6. [Distributed Tracing](#distributed-tracing)
7. [Integration Guide](#integration-guide)
8. [Best Practices](#best-practices)

## Overview

ALEX uses OpenTelemetry for vendor-neutral observability with three pillars:

- **Logs**: Structured JSON logs with contextual information
- **Metrics**: Prometheus-compatible metrics for monitoring
- **Traces**: Distributed traces showing request flow

## Quick Start

### 1. Enable Observability

Create `~/.alex/config.yaml`:

```yaml
observability:
  logging:
    level: info
    format: json

  metrics:
    enabled: true
    prometheus_port: 9090

  tracing:
    enabled: true
    exporter: jaeger
    jaeger_endpoint: http://localhost:14268/api/traces
    sample_rate: 1.0
```

### 2. Connect to Observability Backends

Provision or connect to your preferred observability infrastructure (for example Prometheus, Grafana, Jaeger, or any OTLP-compatible platform).
Ensure the endpoints referenced in your configuration are reachable from ALEX.

### 3. Run ALEX

```bash
alex
```

### 4. View Observability Data

Use the dashboards and interfaces provided by your observability tooling (for example Grafana, Jaeger, or Prometheus) to inspect metrics, traces, and logs exposed by ALEX.

## Configuration

### Logging Configuration

```yaml
observability:
  logging:
    level: debug  # debug, info, warn, error
    format: json  # json, text
```

**Log Levels**:
- `debug`: Verbose debugging information
- `info`: General informational messages (default)
- `warn`: Warning messages for concerning but non-critical issues
- `error`: Error messages for failures

**Log Formats**:
- `json`: Machine-readable JSON format (recommended for production)
- `text`: Human-readable text format (good for development)

### Metrics Configuration

```yaml
observability:
  metrics:
    enabled: true
    prometheus_port: 9090  # Port for Prometheus scraping
```

When enabled, ALEX exposes metrics at `http://localhost:9090/metrics`.

### Tracing Configuration

```yaml
observability:
  tracing:
    enabled: true
    exporter: jaeger           # jaeger, otlp, zipkin
    jaeger_endpoint: http://localhost:14268/api/traces
    otlp_endpoint: localhost:4318   # For OTLP exporter
    zipkin_endpoint: http://localhost:9411/api/v2/spans
    sample_rate: 1.0           # 0.0 to 1.0 (1.0 = 100%)
    service_name: alex
    service_version: 1.0.0
```

**Exporters**:
- `jaeger`: Direct Jaeger collector (recommended)
- `otlp`: OpenTelemetry Protocol (flexible, supports multiple backends)
- `zipkin`: Zipkin compatible (legacy support)

**Sample Rate**: Controls what percentage of traces to collect
- `1.0`: Trace every request (development)
- `0.1`: Trace 10% of requests (production with high traffic)
- `0.01`: Trace 1% of requests (production with very high traffic)

## Structured Logging

### Log Format

JSON logs include:

```json
{
  "time": "2025-10-01T10:30:45.123Z",
  "level": "INFO",
  "msg": "LLM request completed",
  "trace_id": "abc123",
  "session_id": "session-456",
  "model": "gpt-4",
  "latency": 1.234,
  "input_tokens": 100,
  "output_tokens": 50,
  "cost": 0.002
}
```

### Using Logger in Code

```go
// Get logger from observability
obs, _ := observability.New("")
logger := obs.Logger

// Simple logging
logger.Info("Task started")
logger.Debug("Processing step 1")
logger.Warn("Approaching rate limit")
logger.Error("Failed to execute", "error", err)

// Logging with context (includes trace_id, session_id)
ctx := context.WithValue(ctx, "trace_id", "abc123")
logger.InfoContext(ctx, "Request completed",
    "duration", duration,
    "status", "success",
)

// Add persistent fields
logger = logger.With("component", "coordinator", "version", "1.0")
logger.Info("Component initialized")
```

### Sensitive Data Protection

API keys and secrets are automatically sanitized:

```go
// Before: sk-1234567890abcdefghijklmnop
// After:  sk-12345...mnop
logger.Info("API key", "key", observability.SanitizeAPIKey(apiKey))
```

## Metrics

### Available Metrics

#### LLM Metrics

```
alex.llm.requests.total{model="gpt-4",status="success"} 100
alex.llm.tokens.input{model="gpt-4"} 10000
alex.llm.tokens.output{model="gpt-4"} 5000
alex.llm.latency{model="gpt-4",le="1.0"} 95
alex.cost.total{model="gpt-4"} 0.15
```

#### Tool Metrics

```
alex.tool.executions.total{tool_name="file_read",status="success"} 50
alex.tool.duration{tool_name="file_read",le="0.1"} 45
```

#### Session Metrics

```
alex.sessions.active 3
```

### Recording Metrics

```go
// Record LLM request
obs.Metrics.RecordLLMRequest(
    ctx,
    "gpt-4",           // model
    "success",         // status
    1*time.Second,     // latency
    100,               // input tokens
    50,                // output tokens
    0.002,             // cost
)

// Record tool execution
obs.Metrics.RecordToolExecution(
    ctx,
    "file_read",       // tool name
    "success",         // status
    100*time.Millisecond, // duration
)

// Track sessions
obs.Metrics.IncrementActiveSessions(ctx)
// ... later ...
obs.Metrics.DecrementActiveSessions(ctx)
```

### Querying Metrics

In Prometheus (http://localhost:9091):

```promql
# Request rate per second
rate(alex_llm_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(alex_llm_latency_bucket[5m]))

# Total cost in last hour
increase(alex_cost_total[1h])

# Error rate
rate(alex_llm_requests_total{status="error"}[5m])
```

## Distributed Tracing

### Trace Structure

Each ALEX request creates a trace with nested spans:

```
alex.session.solve_task (root)
├── alex.react.iteration (iteration 1)
│   ├── alex.llm.generate
│   ├── alex.tool.execute (file_read)
│   └── alex.tool.execute (bash)
├── alex.react.iteration (iteration 2)
│   ├── alex.llm.generate
│   └── alex.tool.execute (file_write)
└── alex.llm.generate (final answer)
```

### Span Attributes

Each span includes contextual attributes:

```
alex.session_id: "session-123"
alex.tool_name: "file_read"
alex.llm.model: "gpt-4"
alex.llm.input_tokens: 100
alex.llm.output_tokens: 50
alex.llm.token_count: 150
alex.cost: 0.002
alex.iteration: 1
alex.status: "success"
```

### Creating Custom Spans

```go
// Start a span
ctx, span := obs.Tracer.StartSpan(
    ctx,
    "my.custom.operation",
    attribute.String("key", "value"),
)
defer span.End()

// Add attributes
span.SetAttributes(
    attribute.Int("count", 10),
    attribute.String("status", "processing"),
)

// Record error
if err != nil {
    span.SetStatus(codes.Error, err.Error())
    span.RecordError(err)
}

// Use helper functions
span.SetAttributes(observability.LLMAttrs(
    "gpt-4",  // model
    100,      // input tokens
    50,       // output tokens
    0.002,    // cost
)...)
```

### Viewing Traces

In Jaeger UI (http://localhost:16686):

1. Select service: `alex`
2. Select operation (e.g., `alex.session.solve_task`)
3. Click "Find Traces"
4. Click on a trace to see the flamegraph and span details

## Integration Guide

### Integrating Observability in Components

#### 1. Initialize Observability

```go
// In main.go or initialization code
obs, err := observability.New("")  // Uses default config path
if err != nil {
    log.Fatal(err)
}
defer obs.Shutdown(context.Background())
```

#### 2. Instrument LLM Clients

```go
// Wrap LLM client with instrumentation
llmClient := llm.NewOpenAIClient(model, config)
instrumentedClient := observability.NewInstrumentedLLMClient(llmClient, obs)
```

#### 3. Instrument Tool Registry

```go
// Wrap tool registry
registry := tools.NewRegistry()
instrumentedRegistry := observability.NewInstrumentedToolRegistry(registry, obs)
```

#### 4. Add Tracing to Coordinator

```go
func (c *Coordinator) ExecuteTask(ctx context.Context, task string, sessionID string) error {
    // Start root span
    ctx, span := c.obs.Tracer.StartSpan(
        ctx,
        observability.SpanSessionSolveTask,
        observability.SessionAttrs(sessionID)...,
    )
    defer span.End()

    // Add session ID to context
    ctx = observability.ContextWithSessionID(ctx, sessionID)

    // Track active session
    c.obs.Metrics.IncrementActiveSessions(ctx)
    defer c.obs.Metrics.DecrementActiveSessions(ctx)

    // Log execution
    c.obs.Logger.InfoContext(ctx, "Task execution started", "task", task)

    // ... rest of execution ...
}
```

#### 5. Add Tracing to ReAct Engine

```go
func (e *ReactEngine) think(ctx context.Context, state *TaskState) error {
    // Create iteration span
    ctx, span := e.obs.Tracer.StartSpan(
        ctx,
        observability.SpanReactIteration,
        observability.IterationAttrs(state.Iterations)...,
    )
    defer span.End()

    // ... thinking logic ...
}
```

## Best Practices

### 1. Log Levels

- Use `Debug` for detailed debugging information
- Use `Info` for important state changes
- Use `Warn` for recoverable issues
- Use `Error` for failures that need attention

### 2. Contextual Logging

Always include relevant context:

```go
logger.InfoContext(ctx, "Operation completed",
    "operation", "file_read",
    "path", filepath,
    "size", fileSize,
    "duration", duration,
)
```

### 3. Metric Labels

Keep metric labels low-cardinality:

```go
// Good: Limited set of models
RecordLLMRequest(ctx, "gpt-4", "success", ...)

// Bad: Unbounded user IDs
RecordLLMRequest(ctx, model, status, userId, ...) // DON'T DO THIS
```

### 4. Trace Sampling

- Development: `sample_rate: 1.0` (trace everything)
- Production (low traffic): `sample_rate: 0.5` (50%)
- Production (high traffic): `sample_rate: 0.1` (10%)

### 5. Sensitive Data

Never log sensitive information:

```go
// Bad
logger.Info("User authenticated", "password", password)

// Good
logger.Info("User authenticated", "user_id", userID)
```

### 6. Performance

- Use async exports (automatic with OpenTelemetry)
- Don't create too many custom metrics (use labels instead)
- Sample traces in high-traffic scenarios

### 7. Error Handling

Always log and trace errors:

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    logger.ErrorContext(ctx, "Operation failed", "error", err)
    return err
}
```

## Troubleshooting

### No Metrics in Prometheus

1. Check metrics are enabled: `curl http://localhost:9090/metrics`
2. Verify Prometheus can reach ALEX: Check targets at http://localhost:9091/targets
3. Review ALEX logs for observability initialization errors

### No Traces in Jaeger

1. Verify tracing is enabled in config
2. Check Jaeger collector is accessible
3. Ensure sample_rate > 0
4. Look for trace export errors in logs

### High Memory Usage

1. Reduce trace sample rate
2. Decrease Prometheus retention period
3. Reduce log level from debug to info

### Logs Not Structured

Ensure format is set to `json` in config:

```yaml
observability:
  logging:
    format: json
```

## Examples

See `internal/observability/` for complete examples of:

- Logger usage (`logger.go`, `logger_test.go`)
- Metrics collection (`metrics.go`, `metrics_test.go`)
- Distributed tracing (`tracing.go`)
- Instrumentation wrappers (`instrumentation.go`)

## References

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Structured Logging with slog](https://go.dev/blog/slog)
