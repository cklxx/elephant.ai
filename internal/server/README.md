# ALEX Server Module

Server-Sent Events (SSE) backend for ALEX, enabling real-time streaming of agent execution events over HTTP.

## Architecture

Follows hexagonal architecture pattern:

```
ports/              # Interfaces (SSEBroadcaster, ServerSessionManager)
  ↑
app/                # Application logic (EventBroadcaster, ServerCoordinator)
  ↑
http/               # HTTP handlers (SSE, REST API, middleware, router)
```

## Key Components

### EventBroadcaster (`app/event_broadcaster.go`)

- Implements `domain.EventListener` interface
- Manages SSE client connections
- Broadcasts domain events to subscribed clients
- Thread-safe with buffered channels

### ServerCoordinator (`app/server_coordinator.go`)

- Orchestrates task execution
- Integrates AgentCoordinator with EventBroadcaster
- Provides session management operations

### SSEHandler (`http/sse_handler.go`)

- Handles SSE connections
- Serializes domain events to JSON
- Implements heartbeat (30s interval)
- Graceful disconnection handling

### APIHandler (`http/api_handler.go`)

REST API endpoints:
- `POST /api/tasks` - Create and execute task
- `POST /api/sessions` - Create empty session
- `GET /api/sessions` - List sessions
- `GET /api/sessions/:id` - Get session details
- `DELETE /api/sessions/:id` - Delete session
- `GET /health` - Health check

## Usage

See `docs/SSE_SERVER_GUIDE.md` for complete documentation.

### Quick Start

```bash
# Build server
make server-build

# Run server
make server-run

# Run tests
make server-test
```

### Testing SSE

```bash
# Terminal 1: Start SSE connection
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=test&replay=session"

# Terminal 2: Submit task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "List files", "session_id": "test"}'
```

## Design Principles

1. **Reuse existing domain layer** - No duplication of business logic
2. **Event-driven** - Leverages existing `domain.AgentEvent` system
3. **Thread-safe** - Safe for concurrent clients
4. **Graceful degradation** - Drops events if client buffer full
5. **Simple and maintainable** - Minimal abstraction, clear separation

## Event Flow

```
Client POST /api/tasks
  → ServerCoordinator.ExecuteTaskAsync()
    → AgentCoordinator.ExecuteTask(with EventBroadcaster)
      → ReactEngine.SolveTask() [emits domain events]
        → EventBroadcaster.OnEvent()
          → Broadcast to SSE clients
            → SSE stream to browser
```

## Testing

```bash
# Unit tests
go test ./internal/server/app/ -v
go test ./internal/server/http/ -v

# Integration test
./scripts/test-sse-server.sh
```

## Configuration

Environment variables:
- `OPENAI_API_KEY` - Required
- `LLM_PROVIDER` - Default: openrouter
- `LLM_MODEL` - Default: deepseek/deepseek-chat
- `LLM_VISION_MODEL` - Optional (used when images are attached)
- `LLM_BASE_URL` - Default: https://openrouter.ai/api/v1
- `PORT` - Default: 8080

## Future Enhancements

- [ ] Session-based event filtering
- [ ] Authentication middleware
- [ ] Rate limiting
- [ ] WebSocket support
- [ ] Event persistence
- [ ] Metrics & monitoring
