# ALEX SSE Server - Quick Start Guide

## 30-Second Quick Start

```bash
# 1. Build and start server
export OPENAI_API_KEY="sk-..."
make server-run

# 2. In another terminal, connect SSE client
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=demo"

# 3. In a third terminal, submit a task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "What is 2+2?", "session_id": "demo"}'
```

## What You'll See

**Terminal 2 (SSE Stream):**
```
event: connected
data: {"session_id":"demo"}

event: task_analysis
data: {"event_type":"task_analysis","action_name":"Solving math problem",...}

event: thinking
data: {"event_type":"thinking","iteration":1,...}

event: think_complete
data: {"event_type":"think_complete","content":"The answer is 4",...}

event: task_complete
data: {"event_type":"task_complete","final_answer":"2+2 equals 4",...}
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/sse?session_id=X` | GET | SSE event stream |
| `/api/tasks` | POST | Create and execute task |
| `/api/sessions` | GET | List all sessions |
| `/api/sessions/:id` | GET | Get session details |
| `/api/sessions/:id` | DELETE | Delete session |
| `/health` | GET | Health check |

## Event Types

Events you'll receive via SSE:

- `connected` - Connection established
- `task_analysis` - Task pre-analysis complete
- `thinking` - LLM generating response
- `tool_call_start` - Tool execution starting
- `tool_call_complete` - Tool execution finished
- `task_complete` - Task completed

## Configuration

```bash
# Required
export OPENAI_API_KEY="sk-..."

# Optional
export ALEX_LLM_PROVIDER="openai"    # Default: openai
export ALEX_LLM_MODEL="gpt-4o"       # Default: gpt-4o
export PORT="8080"                   # Default: 8080
```

## Common Tasks

### Build Server
```bash
make server-build
```

### Run Tests
```bash
make server-test
```

### Integration Test
```bash
make server-test-integration
```

### Health Check
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### List Sessions
```bash
curl http://localhost:8080/api/sessions
# {"sessions":["session-1","session-2"]}
```

### Complex Task Example
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "List all Go files and count lines of code",
    "session_id": "code-analysis"
  }'
```

## Troubleshooting

### Server won't start
- Check `OPENAI_API_KEY` is set
- Check port 8080 is not in use: `lsof -i :8080`

### No events received
- Verify session_id matches between SSE and task
- Check server logs for errors
- Ensure task was submitted successfully

### Connection drops
- Network proxy might be buffering SSE
- Try increasing heartbeat interval in `sse_handler.go`

## Next Steps

1. **Web Frontend:** Build Next.js UI (see `docs/design/SSE_WEB_ARCHITECTURE.md`)
2. **Authentication:** Add JWT middleware
3. **Rate Limiting:** Implement per-client limits
4. **Deploy:** Use Docker or Kubernetes

## Full Documentation

- Complete Guide: `docs/SSE_SERVER_GUIDE.md`
- Implementation Details: `docs/SSE_SERVER_IMPLEMENTATION.md`
- Architecture Design: `docs/design/SSE_WEB_ARCHITECTURE.md`
