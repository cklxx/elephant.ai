# ALEX Monitoring and Metrics Guide

**Version**: 0.6.0
**Last Updated**: 2025-Q1

This guide provides operational guidance for monitoring ALEX in production environments, including health checks, cost tracking, metrics collection, and troubleshooting.

## Table of Contents

1. [Overview](#overview)
2. [Health Check System](#health-check-system)
3. [Session Cost Tracking](#session-cost-tracking)
4. [Event Broadcaster Metrics](#event-broadcaster-metrics)
5. [Context Compression Metrics](#context-compression-metrics)
6. [Tool Filtering Metrics](#tool-filtering-metrics)
7. [Example Queries](#example-queries)
8. [Troubleshooting](#troubleshooting)

---

## Overview

ALEX's observability stack (v0.6.0+) provides:

- **Health Checks**: Component-level status monitoring
- **Cost Tracking**: Per-session LLM cost isolation
- **Event Metrics**: SSE broadcaster performance
- **Context Metrics**: Token compression statistics
- **Tool Metrics**: Access control and execution stats
- **Structured Logging**: Context-aware logging with sanitization

## Health Check System

### Endpoint

```bash
GET /health
```

### Response Format

```json
{
  "status": "healthy",
  "components": [
    {
      "name": "llm_factory",
      "status": "ready",
      "message": "LLM factory initialized"
    },
    {
      "name": "mcp",
      "status": "not_ready",
      "message": "MCP initialization in progress",
      "details": {
        "attempts": 2,
        "last_attempt": "2025-01-11T10:30:00Z",
        "last_error": "connection timeout"
      }
    }
  ]
}
```

### Component Status Types

| Status | Meaning | Action Required |
|--------|---------|-----------------|
| `ready` | Component operational | None |
| `not_ready` | Component initializing or temporarily unavailable | Monitor; may resolve automatically |
| `disabled` | Component disabled by configuration | Verify intended configuration |

### Health Check Components

#### 1. LLM Factory Probe

**Purpose**: Verify LLM client factory is initialized

**Status Conditions**:
- `ready`: Factory created successfully
- `not_ready`: Factory initialization failed

**Configuration**:
```bash
# LLM factory always initializes unless critical error
# No specific configuration needed
```

#### 2. MCP Probe

**Purpose**: Monitor Model Context Protocol integration

**Status Conditions**:
- `ready`: MCP servers connected, tools registered
- `not_ready`: MCP initialization in progress or failed
- `disabled`: `ALEX_ENABLE_MCP=false`

**Configuration**:
```bash
export ALEX_ENABLE_MCP=true  # Enable MCP
```

**Details Include**:
- Number of initialization attempts
- Last attempt timestamp
- Error messages from failed attempts
- Server count and tool count when ready

### Using Health Checks

#### Kubernetes Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

#### Kubernetes Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  successThreshold: 1
  failureThreshold: 2
```

#### Load Balancer Health Check

**AWS ALB**:
- Health check path: `/health`
- Success codes: `200`
- Interval: 30 seconds
- Timeout: 5 seconds
- Healthy threshold: 2
- Unhealthy threshold: 3

#### Monitoring/Alerting

**Prometheus Alert**:
```yaml
- alert: ALEXComponentNotReady
  expr: alex_health_component_status{status="not_ready"} == 1
  for: 5m
  annotations:
    summary: "ALEX component {{ $labels.component }} not ready"
    description: "Component has been not_ready for 5 minutes"
```

---

## Session Cost Tracking

### Overview

ALEX isolates cost tracking per session (Sprint 1) to prevent interference between concurrent sessions.

### Architecture

**Before (v0.5.x)**: Shared callback state caused cost mixing
```
Session A → LLMClient (shared) → CostTracker (shared) ❌
Session B → LLMClient (shared) → CostTracker (shared) ❌
```

**After (v0.6.0)**: Context-based isolation
```
Session A → Context A → LLMClient → CostTracker A ✅
Session B → Context B → LLMClient → CostTracker B ✅
```

### Querying Session Costs

#### CLI Command

```bash
# View cost for specific session
alex session cost <session_id>

# View cost for current session (in TUI)
/cost
```

#### API Endpoint

```bash
# Get session cost details
curl http://localhost:8080/api/sessions/<session_id>/cost

# Response
{
  "session_id": "session-123",
  "total_cost": 0.0245,
  "breakdown": [
    {
      "model": "gpt-4",
      "requests": 5,
      "input_tokens": 1200,
      "output_tokens": 800,
      "cost": 0.024
    },
    {
      "model": "gpt-3.5-turbo",
      "requests": 2,
      "input_tokens": 300,
      "output_tokens": 100,
      "cost": 0.0005
    }
  ]
}
```

#### Cost Event Stream (SSE)

Cost updates are broadcast via SSE:

```javascript
// Subscribe to cost events
const eventSource = new EventSource('/api/sse?session_id=demo');

eventSource.addEventListener('cost_update', (event) => {
  const data = JSON.parse(event.data);
  console.log('Current cost:', data.total_cost);
  console.log('Last operation:', data.last_operation_cost);
});
```

### Cost Calculation

**Pricing Formula**:
```
Cost = (InputTokens × InputPrice + OutputTokens × OutputPrice) / 1000
```

**Model Pricing** (as of 2025-Q1):
| Model | Input ($/1K) | Output ($/1K) |
|-------|--------------|---------------|
| GPT-4 | $0.03 | $0.06 |
| GPT-3.5 Turbo | $0.0015 | $0.002 |
| DeepSeek Chat | $0.0002 | $0.0006 |

### Cost Tracking Implementation

**Location**: `internal/llm/cost_tracking_decorator.go`

**Key Changes (v0.6.0)**:
- Removed shared callback state
- Cost tracking via context propagation
- Session ID extracted from context
- Thread-safe cost accumulation per session

**Code Reference**:
```go
// Context-aware cost tracking
sessionID := session.ExtractSessionID(ctx)
costTracker := c.getCostTracker(sessionID)
costTracker.RecordUsage(model, inputTokens, outputTokens, cost)
```

---

## Event Broadcaster Metrics

### Overview

The SSE event broadcaster manages real-time event delivery to web clients.

### Available Metrics

#### Connection Metrics

```
alex_sse_connections_active{session_id="demo"} 3
alex_sse_connections_total{session_id="demo"} 45
alex_sse_disconnections_total{session_id="demo"} 42
```

#### Event Delivery Metrics

```
alex_sse_events_broadcasted{event_type="tool_start"} 234
alex_sse_events_broadcasted{event_type="tool_end"} 230
alex_sse_events_dropped{session_id="demo"} 4
alex_sse_broadcast_latency{le="0.1"} 0.95
```

### Monitoring Broadcasting

#### Query Active Connections

```promql
# Active SSE connections per session
sum by (session_id) (alex_sse_connections_active)

# Total connections over time
rate(alex_sse_connections_total[5m])
```

#### Query Event Delivery

```promql
# Event broadcast rate
rate(alex_sse_events_broadcasted[5m])

# Event drop rate (indicates backpressure)
rate(alex_sse_events_dropped[5m])

# 95th percentile broadcast latency
histogram_quantile(0.95, rate(alex_sse_broadcast_latency_bucket[5m]))
```

### Troubleshooting Event Delivery

**Symptom**: Events not appearing in web UI

**Diagnosis**:
1. Check active connections:
   ```bash
   curl http://localhost:8080/api/sse/stats
   ```

2. Verify session ID matches:
   ```bash
   # Client side
   console.log(eventSource.url);

   # Server logs
   grep "SSE connection established" logs/web.log
   ```

3. Check for dropped events:
   ```promql
   alex_sse_events_dropped{session_id="demo"}
   ```

**Resolution**:
- If no active connections: Client connection failed
- If events dropped: Increase channel buffer size
- If latency high: Check event processing pipeline

---

## Context Compression Metrics

### Overview

Context compression reduces token usage by summarizing conversation history.

### Available Metrics

#### Compression Statistics

```
alex_context_compression_ratio{session_id="demo"} 0.65
alex_context_tokens_before{session_id="demo"} 4000
alex_context_tokens_after{session_id="demo"} 2600
alex_context_compressions_total{session_id="demo"} 3
```

#### Token Savings

```
alex_context_tokens_saved_total{session_id="demo"} 4200
alex_context_cost_saved_total{session_id="demo"} 0.126
```

### Monitoring Compression

#### Query Compression Efficiency

```promql
# Average compression ratio (lower is better)
avg(alex_context_compression_ratio)

# Tokens saved over time
rate(alex_context_tokens_saved_total[1h])

# Cost saved (dollars)
sum(alex_context_cost_saved_total)
```

#### Compression Triggers

Context compression triggers when:
1. Message count exceeds threshold (default: 20)
2. Token count exceeds threshold (default: 4000)
3. Manual trigger via `/compress` command

### Optimizing Compression

**Configuration** (`~/.alex-config.json`):
```json
{
  "compression": {
    "enabled": true,
    "message_threshold": 20,
    "token_threshold": 4000,
    "target_ratio": 0.5
  }
}
```

**Tuning Guidelines**:
- **High compression ratio (0.8+)**: Increase summarization aggressiveness
- **Low compression ratio (0.3-)**: Risk losing context; increase threshold
- **Frequent compressions**: Lower message/token thresholds
- **Rare compressions**: Increase thresholds for better context retention

---

## Tool Filtering Metrics

### Overview

Tool filtering enforces preset-based access control (Sprint 3).

### Available Metrics

#### Access Control

```
alex_tool_filter_allowed{preset="read-only",tool="file_read"} 45
alex_tool_filter_blocked{preset="read-only",tool="bash"} 12
alex_tool_executions{tool="file_read",preset="read-only"} 45
```

#### Preset Usage

```
alex_preset_usage{agent_preset="security-analyst",tool_preset="read-only"} 8
alex_preset_usage{agent_preset="code-expert",tool_preset="full"} 23
```

### Monitoring Tool Access

#### Query Access Patterns

```promql
# Most used tools by preset
topk(5, sum by (tool, preset) (alex_tool_executions))

# Blocked tool attempts (security events)
sum by (tool, preset) (alex_tool_filter_blocked)

# Preset popularity
sum by (agent_preset, tool_preset) (alex_preset_usage)
```

#### Security Monitoring

**Alert on Unexpected Blocks**:
```yaml
- alert: UnexpectedToolBlocked
  expr: rate(alex_tool_filter_blocked{preset="full"}[5m]) > 0
  annotations:
    summary: "Tool blocked despite 'full' preset"
    description: "Unexpected tool blocking may indicate misconfiguration"
```

### Tool Preset Access Matrix

| Tool | full | read-only | code-only | web-only | safe |
|------|------|-----------|-----------|----------|------|
| file_read | ✅ | ✅ | ✅ | ❌ | ✅ |
| file_write | ✅ | ❌ | ✅ | ❌ | ✅ |
| file_edit | ✅ | ❌ | ✅ | ❌ | ✅ |
| bash | ✅ | ❌ | ❌ | ❌ | ❌ |
| code_execute | ✅ | ❌ | ✅ | ❌ | ❌ |
| grep | ✅ | ✅ | ✅ | ❌ | ✅ |
| web_search | ✅ | ✅ | ❌ | ✅ | ✅ |
| web_fetch | ✅ | ✅ | ❌ | ✅ | ✅ |

Reference: `internal/agent/presets/tools.go`

---

## Example Queries

### System Health Dashboard

```promql
# Overall system health
up{job="alex"}

# Component health
alex_health_component_status{component="llm_factory"}
alex_health_component_status{component="mcp"}

# Active sessions
alex_sessions_active

# Request rate
rate(alex_llm_requests_total[5m])

# Error rate
rate(alex_llm_requests_total{status="error"}[5m])
```

### Cost Analysis Dashboard

```promql
# Total cost by model
sum by (model) (alex_cost_total)

# Cost per session
sum by (session_id) (alex_cost_total)

# Cost trend (hourly)
increase(alex_cost_total[1h])

# Most expensive sessions
topk(10, sum by (session_id) (alex_cost_total))
```

### Performance Dashboard

```promql
# 95th percentile LLM latency
histogram_quantile(0.95, rate(alex_llm_latency_bucket[5m]))

# Tool execution duration
histogram_quantile(0.95, rate(alex_tool_duration_bucket[5m]))

# SSE broadcast latency
histogram_quantile(0.95, rate(alex_sse_broadcast_latency_bucket[5m]))

# Context compression efficiency
avg(alex_context_compression_ratio)
```

### Security Dashboard

```promql
# Blocked tool attempts
sum by (tool, preset) (alex_tool_filter_blocked)

# Preset usage patterns
sum by (agent_preset, tool_preset) (alex_preset_usage)

# Cancelled tasks
sum(alex_tasks_cancelled_total)

# Failed authentications (if auth enabled)
rate(alex_auth_failures_total[5m])
```

---

## Troubleshooting

### Issue: Health Check Fails

**Symptoms**:
- `/health` returns 500 or non-200 status
- Components show `not_ready` status

**Diagnosis**:
1. Check server logs:
   ```bash
   tail -f logs/server.log | grep health
   ```

2. Test individual components:
   ```bash
   # Test LLM factory
   curl http://localhost:8080/api/models

   # Test MCP
   curl http://localhost:8080/api/mcp/servers
   ```

3. Check configuration:
   ```bash
   # Verify feature flags
   env | grep ALEX_ENABLE
   ```

**Resolution**:
- **LLM factory not ready**: Verify API key is set
- **MCP not ready**: Check MCP server configuration and network connectivity

### Issue: Cost Tracking Incorrect

**Symptoms**:
- Costs mixed between sessions
- Cost showing zero despite API calls
- Cost accumulating after session ends

**Diagnosis**:
1. Check session isolation:
   ```bash
   # Verify session IDs are unique
   curl http://localhost:8080/api/sessions | jq '.[] | .session_id'
   ```

2. Review cost events:
   ```bash
   # Check SSE stream for cost_update events
   curl -N http://localhost:8080/api/sse?session_id=demo | grep cost_update
   ```

3. Verify context propagation:
   ```bash
   # Check logs for session context
   grep "session_id" logs/server.log
   ```

**Resolution**:
- **Costs mixed**: Verify using v0.6.0+ (context-based isolation)
- **Zero cost**: Check LLM client is wrapped with cost tracking decorator
- **Cost after session end**: Verify session cleanup is called

### Issue: Events Not Broadcasting

**Symptoms**:
- Web UI not updating
- SSE events not received
- Event delivery delayed

**Diagnosis**:
1. Check SSE connections:
   ```bash
   curl http://localhost:8080/api/sse/stats
   ```

2. Verify event generation:
   ```bash
   # Check logs for event emission
   grep "Broadcasting event" logs/server.log
   ```

3. Test with curl:
   ```bash
   curl -N -H "Accept: text/event-stream" \
     "http://localhost:8080/api/sse?session_id=demo"
   ```

**Resolution**:
- **No connection**: Check CORS and network configuration
- **Events not generated**: Verify task is running and emitting events
- **Delayed delivery**: Check broadcaster channel buffer size

### Issue: High Memory Usage

**Symptoms**:
- Memory usage growing over time
- OOM kills in production
- Slow performance after long runtime

**Diagnosis**:
1. Check session count:
   ```promql
   alex_sessions_active
   ```

2. Review context compression:
   ```promql
   alex_context_tokens_before
   ```

3. Analyze goroutine count:
   ```bash
   curl http://localhost:8080/debug/pprof/goroutine?debug=1
   ```

**Resolution**:
- **Too many sessions**: Implement session TTL and cleanup
- **Large contexts**: Enable compression or reduce message threshold
- **Goroutine leak**: Check cancel functions are called for all tasks

### Issue: Tool Access Denied

**Symptoms**:
- Tool execution blocked unexpectedly
- "Tool not found" errors for existing tools
- Preset restrictions too strict

**Diagnosis**:
1. Verify preset configuration:
   ```bash
   # Check task metadata
   curl http://localhost:8080/api/tasks/<task_id> | jq '.agent_preset, .tool_preset'
   ```

2. Review tool filtering:
   ```promql
   alex_tool_filter_blocked{tool="bash"}
   ```

3. Check available tools:
   ```bash
   curl http://localhost:8080/api/tools?preset=read-only
   ```

**Resolution**:
- **Wrong preset**: Update task to use appropriate preset
- **Missing tool**: Verify tool is registered in registry
- **Incorrect filtering**: Check preset definition in `internal/agent/presets/tools.go`

---

## Best Practices

### 1. Health Check Configuration

- Set health check interval to 30s for production
- Use longer timeout (5s) to avoid false negatives
- Configure 2-3 failure threshold before marking unhealthy

### 2. Cost Monitoring

- Set up alerts for unusual cost spikes
- Review cost breakdown weekly
- Implement cost budgets per session/user

### 3. Event Broadcasting

- Monitor SSE connection count
- Alert on high event drop rate (>1%)
- Increase buffer size for high-traffic scenarios

### 4. Context Compression

- Enable compression for sessions >10 messages
- Monitor compression ratio (target: 0.5-0.7)
- Review compression quality periodically

### 5. Tool Access Control

- Use `read-only` preset for audits and reviews
- Use `safe` preset when execution risk is concern
- Monitor blocked tool attempts for security

### 6. Log Management

- Use structured JSON logging in production
- Sanitize API keys and sensitive data
- Centralize logs with ELK/Loki stack

---

## References

- [Observability Guide](../reference/OBSERVABILITY.md) - Comprehensive observability setup
- [Architecture Documentation](../architecture/SPRINT_1-4_ARCHITECTURE.md) - System architecture details
- [Health Check Implementation](../../internal/server/app/health.go) - Source code reference
- [Cost Tracking Implementation](../../internal/llm/cost_tracking_decorator.go) - Source code reference
- [Preset System](../reference/PRESET_QUICK_REFERENCE.md) - Agent and tool presets

---

**Last Updated**: 2025-Q1
**Version**: 0.6.0
