# ALEX SSE Server Guide
> Last updated: 2025-11-18


## Overview

The ALEX SSE (Server-Sent Events) Server provides a real-time streaming API for executing AI agent tasks over HTTP. It follows the hexagonal architecture pattern and reuses the existing domain event system.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   ALEX SSE Server (Go)                       │
│  ┌─────────────────────────────────────────────────────────┤
│  │              HTTP/SSE API Layer                          │
│  │  ┌──────────┐  ┌──────────┐                             │
│  │  │   SSE    │  │   REST   │                             │
│  │  │ Handler  │  │  Handler │                             │
│  │  └──────────┘  └──────────┘                             │
│  └─────────────────────────────────────────────────────────┤
│  │            Service Layer (Application)                   │
│  │  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │  │ EventBroadcaster│  │  ServerCoordinator          │  │
│  │  └─────────────────┘  └─────────────────────────────┘  │
│  └─────────────────────────────────────────────────────────┤
│  │             Domain Layer (Reused)                        │
│  │  ┌─────────────────────────────────────────────────────┤
│  │  │   ReactEngine   │   ToolRegistry  │  EventSystem   │ │
│  │  └─────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
internal/server/
├── ports/                    # Interfaces
│   ├── broadcaster.go        # SSEBroadcaster interface
│   └── session.go            # ServerSessionManager interface
├── app/                      # Application layer
│   ├── event_broadcaster.go  # Implements domain.EventListener
│   └── server_coordinator.go # Orchestrates task execution
└── http/                     # HTTP handlers
    ├── sse_handler.go        # SSE endpoint handler
    ├── api_handler.go        # REST API handlers
    ├── middleware.go         # CORS, logging
    └── router.go             # HTTP router setup

cmd/alex-server/
└── main.go                   # Server entry point
```

## API Endpoints

### SSE Endpoint

**GET /api/sse?session_id={sessionID}**

Opens a Server-Sent Events connection to receive real-time task execution events.

**Response Format:**
```
Content-Type: text/event-stream

event: connected
data: {"session_id":"abc123"}

event: task_analysis
data: {"event_type":"task_analysis","timestamp":"2025-10-02T10:00:00Z","action_name":"Analyzing task","goal":"Complete the task"}

event: tool_call_start
data: {"event_type":"tool_call_start","tool_name":"bash","arguments":{"command":"ls -la"}}

event: tool_call_complete
data: {"event_type":"tool_call_complete","tool_name":"bash","result":"file1.txt\nfile2.txt","duration":150}

event: task_complete
data: {"event_type":"task_complete","final_answer":"Task completed successfully","total_iterations":3}
```

### REST Endpoints

**POST /api/tasks**

Creates and executes a new task asynchronously.

Request:
```json
{
  "task": "List all files in the current directory",
  "session_id": "abc123"
}
```

Response:
```json
{
  "status": "accepted",
  "session_id": "abc123",
  "message": "Task is being executed. Connect to SSE endpoint to receive events."
}
```

**GET /api/sessions**

Lists all available sessions.

Response:
```json
{
  "sessions": ["session-1", "session-2", "session-3"]
}
```

**GET /api/sessions/:id**

Retrieves session details.

Response:
```json
{
  "id": "session-1",
  "messages": [...],
  "todos": [...],
  "created_at": "2025-10-02T10:00:00Z",
  "updated_at": "2025-10-02T10:05:00Z"
}
```

**DELETE /api/sessions/:id**

Deletes a session.

**GET /health**

Health check endpoint.

Response:
```json
{
  "status": "ok"
}
```

## Configuration

Configure the server using environment variables:

```bash
export OPENAI_API_KEY="sk-..."          # Required
export ALEX_LLM_PROVIDER="openai"       # Default: openai
export ALEX_LLM_MODEL="gpt-4o"          # Default: gpt-4o
export ALEX_BASE_URL=""                 # Optional custom base URL
export PORT="8080"                      # Default: 8080
```

## Running the Server

### Build and Run

```bash
# Build the server
go build -o alex-server ./cmd/alex-server

# Run the server
./alex-server
```

### Using Make

```bash
# Add to Makefile
make server-build  # Build server
make server-run    # Run server
```

## Testing with curl

### 1. Start SSE Connection

In one terminal:
```bash
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=test-session"
```

### 2. Submit a Task

In another terminal:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "List all .go files in the current directory",
    "session_id": "test-session"
  }'
```

### 3. List Sessions

```bash
curl http://localhost:8080/api/sessions
```

### 4. Get Session Details

```bash
curl http://localhost:8080/api/sessions/test-session
```

### 5. Health Check

```bash
curl http://localhost:8080/health
```

## Event Types

The server emits the following event types (from `internal/agent/domain/events.go`):

| Event Type | Description |
|------------|-------------|
| `connected` | Initial connection established |
| `task_analysis` | Task pre-analysis complete |
| `iteration_start` | New ReAct iteration starting |
| `thinking` | LLM is generating response |
| `think_complete` | LLM response received |
| `tool_call_start` | Tool execution beginning |
| `tool_call_stream` | Streaming tool output |
| `tool_call_complete` | Tool execution finished |
| `iteration_complete` | Iteration finished |
| `task_complete` | Entire task completed |
| `error` | Error occurred |

## Key Implementation Details

### EventBroadcaster

- Implements `domain.EventListener` interface
- Manages client connections per session
- Broadcasts events to all subscribed clients
- Handles buffer overflow gracefully (drops events if client buffer full)
- Thread-safe with RWMutex

### SSE Handler

- Sets proper SSE headers (`text/event-stream`)
- Implements heartbeat every 30 seconds
- Gracefully handles client disconnection
- Serializes domain events to JSON format

### ServerCoordinator

- Wraps existing `AgentCoordinator`
- Integrates `EventBroadcaster` as event listener
- Provides session management operations
- Executes tasks asynchronously

## CORS Configuration

The server includes CORS middleware that allows:
- Origins: `localhost:3000`, `localhost:3001`
- Methods: `GET, POST, PUT, DELETE, OPTIONS`
- Headers: `Content-Type, Authorization`

For production, update `internal/server/http/middleware.go` to restrict allowed origins.

## Deployment

### Docker

Create `Dockerfile.server`:
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o alex-server ./cmd/alex-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/alex-server /usr/local/bin/
EXPOSE 8080
CMD ["alex-server"]
```

Build and run:
```bash
docker build -f Dockerfile.server -t alex-server .
docker run -p 8080:8080 -e OPENAI_API_KEY=sk-... alex-server
```

### Docker Compose

See `docs/design/SSE_WEB_ARCHITECTURE.md` for full docker-compose configuration.

## Performance Considerations

### Heartbeat Interval

The server sends heartbeat comments every 30 seconds to keep connections alive. Adjust in `sse_handler.go` if needed.

### Event Buffer Size

Each client has a 100-event buffer. Events are dropped if the buffer is full. Increase the buffer size in `sse_handler.go` if clients are slow to consume events.

### Concurrent Clients

The broadcaster uses `sync.RWMutex` for thread safety. It can handle hundreds of concurrent clients efficiently.

## Future Enhancements

1. **Session-based Event Filtering**: Currently broadcasts to all sessions. Implement proper session ID extraction from event context.

2. **Authentication**: Add JWT or API key authentication middleware.

3. **Rate Limiting**: Implement per-client rate limiting for task submissions.

4. **Event Persistence**: Store events in database for replay and historical analysis.

5. **WebSocket Support**: Add WebSocket alternative for bidirectional communication.

6. **Metrics & Monitoring**: Add Prometheus metrics for connection count, event throughput, etc.

## Troubleshooting

### Connection Drops

- Check firewall rules
- Verify proxy settings (disable buffering for SSE)
- Increase heartbeat interval if network is unstable

### Missing Events

- Check client buffer size
- Verify session ID matches between SSE connection and task submission
- Check server logs for errors

### High Memory Usage

- Monitor client connection count
- Check for zombie connections (clients not properly disconnecting)
- Reduce event buffer size if needed

## References

- Design Document: `docs/design/SSE_WEB_ARCHITECTURE.md`
- Domain Events: `internal/agent/domain/events.go`
- Agent Coordinator: `internal/agent/app/coordinator.go`
