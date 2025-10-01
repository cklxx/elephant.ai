# ALEX Observability Stack

This directory contains the complete observability stack for ALEX, including:

- **Prometheus** - Metrics collection and storage
- **Jaeger** - Distributed tracing
- **Grafana** - Visualization and dashboards

## Quick Start

### 1. Start the Observability Stack

```bash
# From the project root
cd deployments/observability
docker-compose up -d
```

This will start:
- Prometheus on http://localhost:9091
- Jaeger UI on http://localhost:16686
- Grafana on http://localhost:3000

### 2. Configure ALEX

Create or edit `~/.alex/config.yaml`:

```yaml
observability:
  logging:
    level: info      # debug, info, warn, error
    format: json     # json, text

  metrics:
    enabled: true
    prometheus_port: 9090  # ALEX metrics endpoint

  tracing:
    enabled: true
    exporter: jaeger       # jaeger, otlp, zipkin
    jaeger_endpoint: http://localhost:14268/api/traces
    sample_rate: 1.0       # 0.0 to 1.0
```

### 3. Run ALEX

```bash
# Run ALEX normally
alex

# Or with verbose logging
ALEX_LOG_LEVEL=debug alex
```

### 4. Access Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
  - Pre-loaded ALEX dashboard shows all key metrics
- **Prometheus**: http://localhost:9091
  - View raw metrics and query data
- **Jaeger**: http://localhost:16686
  - View distributed traces of requests

## Architecture

### Metrics Flow

```
ALEX App (port 9090)
    ↓
Prometheus Scraper (15s interval)
    ↓
Prometheus TSDB
    ↓
Grafana Dashboard
```

### Traces Flow

```
ALEX App
    ↓
Jaeger Collector (HTTP: 14268)
    ↓
Jaeger Storage
    ↓
Jaeger UI
```

### Logs

ALEX uses structured JSON logging to stdout. Logs include:
- Timestamp
- Log level
- Message
- Context fields (trace_id, session_id, etc.)

## Key Metrics

### LLM Metrics
- `alex.llm.requests.total` - Total LLM API calls
- `alex.llm.tokens.input` - Input tokens sent
- `alex.llm.tokens.output` - Output tokens received
- `alex.llm.latency` - Request latency (histogram)
- `alex.cost.total` - Estimated cost in USD

### Tool Metrics
- `alex.tool.executions.total` - Tool executions count
- `alex.tool.duration` - Tool execution duration (histogram)

### Session Metrics
- `alex.sessions.active` - Currently active sessions

## Traces

Each ALEX request creates a trace with the following spans:

1. `alex.session.solve_task` - Root span for entire request
2. `alex.react.iteration` - Each ReAct loop iteration
3. `alex.tool.execute` - Each tool execution
4. `alex.llm.generate` - Each LLM API call

Spans include attributes:
- `alex.session_id` - Session identifier
- `alex.tool_name` - Tool being executed
- `alex.llm.model` - LLM model name
- `alex.llm.token_count` - Total tokens used
- `alex.cost` - Estimated cost

## Grafana Dashboard

The pre-loaded dashboard (`alex-dashboard.json`) includes:

1. **LLM Request Rate** - Requests per second by model and status
2. **LLM Latency Percentiles** - p50, p95, p99 latencies
3. **Token Usage Rate** - Input/output tokens per second
4. **Cost Over Time** - Estimated costs by model
5. **Active Sessions** - Current active sessions gauge
6. **Tool Usage Distribution** - Pie chart of tool usage
7. **Tool Execution Duration** - p95 duration by tool
8. **Model Usage Distribution** - Pie chart of model usage

## Configuration

### Prometheus

Edit `prometheus.yml` to adjust:
- Scrape interval (default: 15s)
- Scrape targets
- Retention period

### Jaeger

Environment variables in `docker-compose.yml`:
- `COLLECTOR_OTLP_ENABLED=true` - Enable OTLP protocol
- Adjust ports if needed

### Grafana

- Default credentials: `admin/admin`
- Datasources are auto-provisioned
- Dashboards are auto-loaded from `grafana/dashboards/`

## Troubleshooting

### Metrics not showing in Prometheus

1. Check ALEX is running with metrics enabled
2. Verify ALEX metrics endpoint: `curl http://localhost:9090/metrics`
3. Check Prometheus targets: http://localhost:9091/targets
4. Ensure `host.docker.internal` resolves (Docker Desktop required)

### Traces not appearing in Jaeger

1. Verify tracing is enabled in ALEX config
2. Check Jaeger collector is accessible: `curl http://localhost:14268/api/traces`
3. Review ALEX logs for tracing errors
4. Verify sample rate is not 0.0

### Grafana dashboard not loading

1. Check datasources: Grafana → Configuration → Data Sources
2. Test Prometheus connection
3. Verify dashboard JSON is valid
4. Check Grafana logs: `docker logs alex-grafana`

## Advanced Usage

### Custom Metrics

Add custom metrics in your code:

```go
// Record custom metric
obs.Metrics.RecordToolExecution(
    ctx,
    "my_custom_tool",
    "success",
    duration,
)
```

### Custom Traces

Add custom spans:

```go
ctx, span := obs.Tracer.StartSpan(
    ctx,
    "my.custom.operation",
    attribute.String("key", "value"),
)
defer span.End()

// Your code here
```

### Custom Logs

Log with context:

```go
obs.Logger.InfoContext(ctx, "Operation completed",
    "operation", "my_op",
    "duration", duration,
    "status", "success",
)
```

## Production Recommendations

1. **Reduce Sample Rate**: Set `sample_rate: 0.1` to trace 10% of requests
2. **Set Log Level**: Use `level: info` or `level: warn` in production
3. **Configure Retention**: Adjust Prometheus retention in docker-compose
4. **Add Alerting**: Configure Grafana alerts for critical metrics
5. **Secure Access**: Add authentication and HTTPS in production
6. **External Storage**: Use remote storage for Prometheus (Thanos, Cortex)

## Stopping the Stack

```bash
# Stop containers
docker-compose down

# Stop and remove volumes (deletes all data)
docker-compose down -v
```

## References

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
