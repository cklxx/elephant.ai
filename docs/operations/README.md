# ALEX Operations Guide

This guide provides practical operational guidance for deploying, monitoring, and troubleshooting ALEX in production and development environments.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Health Monitoring](#health-monitoring)
3. [Configuration Best Practices](#configuration-best-practices)
4. [Task Management](#task-management)
5. [Troubleshooting](#troubleshooting)
6. [Performance Tuning](#performance-tuning)
7. [Security Considerations](#security-considerations)

## Quick Start

### Minimal Configuration (Testing/Development)

For local development without external dependencies:

```json
{
  "api_key": "",
  "enable_mcp": false,
  "enable_git_tools": false
}
```

Or via environment:
```bash
export ALEX_ENABLE_MCP=false
export ALEX_ENABLE_GIT_TOOLS=false
```

This configuration allows:
- Running tests without API keys
- Offline development
- Fast container startup
- CI/CD pipeline compatibility

### Production Configuration

Full-featured production setup:

```json
{
  "api_key": "sk-your-openai-key",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4",
  "enable_mcp": true,
  "enable_git_tools": true,
  "tavily_api_key": "tvly-your-key",
  "max_tokens": 4096,
  "max_iterations": 10
}
```

Environment variables:
```bash
export OPENAI_API_KEY="sk-your-key"
export TAVILY_API_KEY="tvly-your-key"
export ALEX_ENABLE_MCP=true
export ALEX_ENABLE_GIT_TOOLS=true
```

### Deployment

#### Local Development
```bash
# Start backend + frontend
./deploy.sh start

# Check status
./deploy.sh status

# View logs
./deploy.sh logs

# Stop services
./deploy.sh down
```

#### Docker Compose
```bash
# Set environment
echo "OPENAI_API_KEY=sk-your-key" > .env

# Start services
docker-compose up -d

# View logs
docker-compose logs -f alex-server

# Stop services
docker-compose down
```

## Health Monitoring

### Health Check Endpoint

ALEX provides a comprehensive health check endpoint at `/health`:

```bash
curl http://localhost:8080/health
```

**Response Format**:
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
      "name": "git_tools",
      "status": "ready",
      "message": "Git tools available",
      "details": {
        "tools": ["git_commit", "git_pr", "git_history"]
      }
    },
    {
      "name": "mcp",
      "status": "ready",
      "message": "MCP initialized successfully",
      "details": {
        "servers": 2,
        "tools": 15,
        "attempts": 1
      }
    }
  ]
}
```

### Component Status Values

- **`ready`**: Component is healthy and operational
- **`not_ready`**: Component is initializing or temporarily unavailable
- **`disabled`**: Component is disabled by configuration

### Health Check Integration

**Kubernetes Liveness Probe**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

**Docker Compose**:
```yaml
services:
  alex-server:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Monitoring Component Health

**Check if MCP is ready**:
```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name=="mcp")'
```

**Check if Git tools are enabled**:
```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name=="git_tools") | .status'
```

**Alert on unhealthy components**:
```bash
STATUS=$(curl -s http://localhost:8080/health | jq -r '.status')
if [ "$STATUS" != "healthy" ]; then
  echo "ALERT: ALEX is not healthy"
  exit 1
fi
```

## Configuration Best Practices

### Feature Flag Strategy

**When to disable MCP**:
- Running in restricted environments without external process execution
- CI/CD pipelines where MCP servers aren't available
- Testing scenarios requiring isolated execution
- Reducing startup time in development

**When to disable Git tools**:
- Running without LLM access (offline mode)
- Testing file and search tools only
- Reducing initialization overhead
- Environments without git installed

### Environment-Specific Configs

**Development** (`dev.json`):
```json
{
  "enable_mcp": false,
  "enable_git_tools": false,
  "verbose": true,
  "max_iterations": 5
}
```

**Staging** (`staging.json`):
```json
{
  "enable_mcp": true,
  "enable_git_tools": true,
  "verbose": true,
  "max_iterations": 10
}
```

**Production** (`prod.json`):
```json
{
  "enable_mcp": true,
  "enable_git_tools": true,
  "verbose": false,
  "max_iterations": 10,
  "max_tokens": 4096
}
```

### Storage Configuration

**Session Storage**:
```bash
export ALEX_SESSION_DIR="~/.alex-sessions"
```
- Default: `~/.alex-sessions`
- Persistent session history
- Safe to backup/restore

**Cost Tracking**:
```bash
export ALEX_COST_DIR="~/.alex-costs"
```
- Default: `~/.alex-costs`
- Per-session cost tracking
- Use for billing and usage analytics

## Task Management

### Creating Tasks

**Basic task**:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "Analyze the codebase", "session_id": "sess-123"}'
```

**Task with agent preset**:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review code for security vulnerabilities",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only",
    "session_id": "security-audit"
  }'
```

### Cancelling Tasks

**Cancel a running task**:
```bash
curl -X POST http://localhost:8080/api/tasks/task-abc123/cancel
```

**Response**:
```json
{
  "message": "Task cancelled successfully",
  "task_id": "task-abc123"
}
```

**Task status after cancellation**:
```json
{
  "task_id": "task-abc123",
  "status": "cancelled",
  "message": "task cancelled by user"
}
```

### Monitoring Tasks

**Get task status**:
```bash
curl http://localhost:8080/api/tasks/task-abc123
```

**List all tasks**:
```bash
curl http://localhost:8080/api/tasks
```

**Watch task progress via SSE**:
```bash
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=sess-123"
```

## Troubleshooting

### Common Issues

#### 1. Container fails to start

**Symptom**: `make test` or server startup fails with initialization errors

**Cause**: MCP or Git tools trying to initialize without dependencies

**Solution**:
```bash
export ALEX_ENABLE_MCP=false
export ALEX_ENABLE_GIT_TOOLS=false
```

Or in config:
```json
{
  "enable_mcp": false,
  "enable_git_tools": false
}
```

#### 2. MCP shows "not_ready" in health check

**Symptom**: Health check shows MCP in `not_ready` state

**Diagnosis**:
```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name=="mcp")'
```

**Possible causes**:
- MCP servers not installed or configured
- MCP configuration file missing (`~/.alex/mcp-config.json`)
- MCP servers failing to start

**Solution**:
1. Check MCP configuration: `cat ~/.alex/mcp-config.json`
2. Verify MCP servers are executable
3. Check logs for MCP initialization errors
4. Disable MCP if not needed: `export ALEX_ENABLE_MCP=false`

#### 3. Git tools unavailable

**Symptom**: Health check shows git_tools as `disabled` or `not_ready`

**Diagnosis**:
```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name=="git_tools")'
```

**Possible causes**:
- Git tools disabled in configuration
- LLM client initialization failed
- No API key provided

**Solution**:
1. Enable Git tools: `export ALEX_ENABLE_GIT_TOOLS=true`
2. Ensure API key is set: `export OPENAI_API_KEY=sk-your-key`
3. Check LLM factory initialization in logs

#### 4. Task cancellation not working

**Symptom**: Task continues running after cancellation request

**Diagnosis**:
```bash
curl http://localhost:8080/api/tasks/task-id
# Check status field
```

**Possible causes**:
- Task already completed
- Task not found
- Cancel function not registered (timing issue)

**Solution**:
- Only tasks in `pending` or `running` status can be cancelled
- Check task status before cancelling
- If task is stuck, restart the server

#### 5. High memory usage

**Symptom**: Server memory usage grows over time

**Possible causes**:
- Session accumulation
- MCP process leaks
- Large SSE event buffers

**Solution**:
1. Clean up old sessions:
```bash
rm -rf ~/.alex-sessions/old-*
```
2. Restart MCP if enabled:
```bash
# Check MCP initialization
curl http://localhost:8080/health
```
3. Monitor SSE connections:
```bash
# Check active connections (implementation dependent)
netstat -an | grep :8080
```

### Debug Mode

Enable verbose logging:
```bash
export ALEX_VERBOSE=1
./alex-server
```

Or in config:
```json
{
  "verbose": true
}
```

### Log Analysis

**Server logs location**:
```bash
# Local deployment
./deploy.sh logs

# Docker Compose
docker-compose logs -f alex-server

# Direct run
journalctl -u alex-server -f
```

**Key log patterns**:
- `[DI]` - Dependency injection lifecycle
- `[ServerCoordinator]` - Task execution
- `[HealthChecker]` - Health probe results
- `[MCP]` - MCP initialization and errors
- `[GitTools]` - Git tool registration

## Performance Tuning

### Container Startup

**Fast startup (disable heavy dependencies)**:
```bash
export ALEX_ENABLE_MCP=false
export ALEX_ENABLE_GIT_TOOLS=false
```

**Expected startup time**:
- Minimal config: < 1 second
- With Git tools: 1-2 seconds
- With MCP: 2-5 seconds (depends on MCP servers)

### Task Execution

**Limit iterations for faster responses**:
```json
{
  "max_iterations": 5
}
```

**Reduce token usage**:
```json
{
  "max_tokens": 2048
}
```

### Concurrent Sessions

ALEX supports concurrent task execution with isolated cost tracking:
- Each session maintains independent state
- Cost tracking isolated per session
- Cancellation propagates correctly per task

**Monitor concurrent tasks**:
```bash
curl http://localhost:8080/api/tasks | jq '[.[] | select(.status=="running")] | length'
```

## Security Considerations

### API Key Management

**Never commit API keys**:
- Use environment variables
- Use secret management systems (Vault, AWS Secrets Manager)
- Rotate keys regularly

**Secure configuration**:
```bash
# Set restrictive permissions
chmod 600 ~/.alex-config.json
```

### Network Security

**Expose only necessary ports**:
```yaml
# Docker Compose example
services:
  alex-server:
    ports:
      - "127.0.0.1:8080:8080"  # Bind to localhost only
```

**Use reverse proxy**:
```nginx
# Nginx example
server {
  listen 80;
  server_name alex.example.com;

  location / {
    proxy_pass http://localhost:8080;
    proxy_set_header Host $host;
  }
}
```

### Tool Access Control

**Use restricted presets for untrusted tasks**:
```bash
# Read-only access for code review
curl -X POST http://localhost:8080/api/tasks \
  -d '{"tool_preset": "read-only", "task": "Review this PR"}'

# Safe preset (no code execution)
curl -X POST http://localhost:8080/api/tasks \
  -d '{"tool_preset": "safe", "task": "Analyze architecture"}'
```

### Session Isolation

- Each session maintains isolated context
- Cost tracking per session prevents interference
- Cancel one task without affecting others

## Metrics and Monitoring

### Key Metrics to Track

**Health metrics**:
- `/health` endpoint response time
- Component status (ready/not_ready/disabled)
- MCP initialization attempts

**Task metrics**:
- Task creation rate
- Task completion rate
- Task cancellation rate
- Average task duration

**Resource metrics**:
- Memory usage
- CPU usage
- Session count
- Active SSE connections

### Example Monitoring Script

```bash
#!/bin/bash
# monitor-alex.sh

# Check health
HEALTH=$(curl -s http://localhost:8080/health)
STATUS=$(echo $HEALTH | jq -r '.status')

if [ "$STATUS" != "healthy" ]; then
  echo "ALERT: ALEX unhealthy - $STATUS"
  echo $HEALTH | jq '.components[] | select(.status != "ready")'
fi

# Check running tasks
RUNNING=$(curl -s http://localhost:8080/api/tasks | jq '[.[] | select(.status=="running")] | length')
echo "Running tasks: $RUNNING"

# Check component status
echo "Component Status:"
echo $HEALTH | jq -r '.components[] | "\(.name): \(.status)"'
```

## Additional Resources

- [Observability Guide](../reference/OBSERVABILITY.md) - Detailed observability setup
- [Architecture Review](../analysis/base_flow_architecture_review.md) - Sprint 1-4 improvements
- [Preset System](../reference/PRESET_QUICK_REFERENCE.md) - Agent and tool presets
- [Deployment Guide](DEPLOYMENT.md) - Production deployment patterns

## Support

For issues and questions:
- GitHub Issues: https://github.com/cklxx/Alex-Code/issues
- Documentation: https://github.com/cklxx/Alex-Code/tree/main/docs
