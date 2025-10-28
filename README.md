# ALEX

Terminal-native AI programming agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Philosophy

**Keep it simple and explicit—no new entities unless the problem demands it.**

ALEX embraces a hexagonal architecture with clear boundaries between the core domain, ports, and adapters. Every subsystem is designed to minimize accidental complexity so that contributors can focus on the essential behaviour of an AI coding agent.

## System Overview

| Interface | Description |
|-----------|-------------|
| CLI (TUI + legacy line mode) | Native terminal experience with streaming output, pane navigation, and session management. |
| Web UI | Real-time interface backed by Server-Sent Events (SSE) for managing tasks, tools, and session history. |
| HTTP API | Public endpoints for task submission, session control, and health checks. |

Core services are written in Go with a modular toolkit, while the web interface is powered by a separate Node/React application.

## Highlights

- **Rich Interaction Modes**
  - Terminal UI with panes for transcript, tool output, and subagents
  - One-shot command execution from the shell
  - Web dashboard that mirrors streaming events

- **Tooling Ecosystem (15+ tools)**
  - File operations, shell execution, search, git, reasoning, and web utilities
  - "Think" and "subagent" helpers for complex multi-step tasks

- **Model Flexibility**
  - Providers: OpenAI, DeepSeek, OpenRouter, Ollama, and custom endpoints via configuration
  - Configurable per-session presets for agent persona and tool access level (v0.6.0+)

- **Operational Resilience**
  - Persisted sessions with fork/resume support
  - Task cancellation API with end-to-end propagation
  - Structured logging, health checks, and cost tracking

- **Observability (v0.6.0+)**
  - Health endpoints for readiness/liveness
  - Session cost isolation and live spend tracking
  - Metrics around SSE broadcasting, context compression, and tool filtering

## Architecture (2025 Q1 review)

ALEX follows a hexagonal architecture that keeps domain logic independent from infrastructure concerns.

```
Domain (pure business logic)
  ↓ depends on
Ports (interfaces/contracts)
  ↑ implemented by
Adapters (infrastructure and delivery)
```

```
internal/
├── agent/
│   ├── domain/       # core reasoning engine, tool formatter
│   ├── app/          # coordinators and orchestrators
│   └── ports/        # interfaces consumed by adapters
├── llm/              # integrations for OpenAI, DeepSeek, Ollama, OpenRouter, etc.
├── session/          # persistence, repositories, serialization
├── tools/
│   ├── builtin/      # file, shell, git, search, think, subagent tools
│   └── registry.go   # tool registration and discovery
└── server/           # HTTP/SSE server, handlers, routing
```

The latest architecture assessment prioritises work on cost isolation, cancellation propagation, lazy dependency injection, and deeper observability. See [docs/analysis/base_flow_architecture_review.md](docs/analysis/base_flow_architecture_review.md) for recommendations and roadmap notes.

## Installation

### CLI via npm

```bash
npm install -g alex-code
```

### Build from source

```bash
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code
make build
```

Releases are published at [github.com/cklxx/Alex-Code/releases](https://github.com/cklxx/Alex-Code/releases).

### Web Server + UI (Local Script)

```bash
# Clone repository
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code

# Provide model credentials
echo "OPENAI_API_KEY=sk-your-key" > .env

# Start services (backend + web UI)
./deploy.sh

# Web UI available at http://localhost:3000
# API available at http://localhost:8080

# Stop services
./deploy.sh down
```

Refer to [QUICKSTART_SSE.md](QUICKSTART_SSE.md) for detailed instructions on the streaming stack.

## Usage

### CLI (TUI)

```bash
alex
```

Key bindings:

- `Tab` / `Shift+Tab` — cycle focus between panes (transcript, stream, tools, subagents, MCP, input)
- `PgUp` / `PgDn`, arrow keys — scroll focused pane; `End` jumps to live output
- `?` — toggle the in-terminal help overlay
- `/` — search within the focused pane; `n`/`N` navigates results

Slash commands:

- `/new` — start a fresh session
- `/sessions` — list saved sessions (current session highlighted)
- `/load <id>` — load a previous session
- `/mcp [list|refresh|restart <name>]` — inspect or restart MCP servers
- `/cost [session_id]` — print the latest spend totals
- `/export [path]` — write the transcript to Markdown (default naming included)
- `/follow <transcript|stream|both> <on|off|toggle>` — control auto-follow behaviour
- `/verbose [on|off|toggle]` — adjust CLI verbosity
- `/quit` — exit (equivalent to `Ctrl+C`)

The Subagents pane highlights agent level, start/finish timestamps, and duration for each worker. The status bar displays live token totals and spend per model so you can monitor usage during long sessions.

Enable the legacy line UI by launching:

```bash
alex --no-tui
```

or setting `ALEX_NO_TUI=1` before running `alex`.

### CLI Command Mode

```bash
alex "analyze this codebase"
```

Resume or list sessions:

```bash
alex -r <session_id> -i   # resume
alex session list         # list all sessions
```

Starting the TUI with no arguments automatically restores the most recent session.

### Web Server Mode

Start the server:

```bash
# Scripted (launches backend + web UI)
./deploy.sh

# Manual fallback
make server-run
cd web && npm install
npm run dev
```

Endpoints:

- Web UI: http://localhost:3000
- API: http://localhost:8080
- SSE stream: http://localhost:8080/api/sse?session_id=<id>

Sample API calls:

```bash
# Submit a task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "What is 2+2?", "session_id": "demo"}'

# Submit with presets (v0.6.0+)
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
        "task": "Review code for security issues",
        "agent_preset": "security-analyst",
        "tool_preset": "read-only",
        "session_id": "demo"
      }'

# Cancel a running task
curl -X POST http://localhost:8080/api/tasks/task-abc123/cancel

# Health check
curl http://localhost:8080/health

# Watch SSE events
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=demo"
```

## Configuration

The first CLI launch creates `~/.alex-config.json`:

```json
{
  "api_key": "sk-or-xxx",
  "base_url": "https://openrouter.ai/api/v1",
  "model": "deepseek/deepseek-chat-v3-0324:free"
}
```

Override values with environment variables:

```bash
# LLM configuration
export OPENAI_API_KEY="your-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4"

# CLI behaviour
export ALEX_VERBOSE="1"
export ALEX_TUI_FOLLOW_TRANSCRIPT="0"
export ALEX_TUI_FOLLOW_STREAM="0"

# Feature flags (v0.6.0+)
export ALEX_ENABLE_MCP="true"
```

Persist defaults by setting `"follow_transcript"` and `"follow_stream"` inside `~/.alex-config.json`.

### Feature Flags (v0.6.0+)

| Variable | Default | Description |
|----------|---------|-------------|
| `ALEX_ENABLE_MCP` | `true` | Toggle Model Context Protocol integration |

Minimal configuration for offline/testing environments:

```json
{
  "enable_mcp": false
}
```

This allows development without external API dependencies.

## Health & Observability

### Health Checks

```bash
curl http://localhost:8080/health
```

Example response:

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
      "status": "ready",
      "message": "MCP initialized with 3 servers, 12 tools registered"
    }
  ]
}
```

Component status values:

- `ready` — component is operational
- `not_ready` — initializing or temporarily unavailable
- `disabled` — turned off via configuration

Use this endpoint for Kubernetes probes, load balancer checks, and monitoring pipelines.

### Cost Tracking

Each session tracks LLM spend separately and accumulates totals in real time.

```bash
alex session cost <session_id>
```

Or query via API:

```bash
curl http://localhost:8080/api/sessions/<session_id>/cost
```

### Task Cancellation

Cancel running tasks gracefully:

```bash
curl -X POST http://localhost:8080/api/tasks/<task_id>/cancel
```

Cancellation signals propagate through the orchestrator, tooling layer, and LLM requests to ensure proper cleanup.

### Metrics & Logging

- Event broadcaster metrics track SSE connections and delivery rates
- Context compression reports token reduction ratios
- Tool filtering metrics surface preset-based access control decisions
- Session lifecycle metrics reveal active sessions and completion rates

Structured logging propagates session IDs and trace IDs with API key sanitisation. Enable verbose logging during development:

```bash
export ALEX_VERBOSE=1
alex --verbose
```

## Development Workflow

```bash
make dev     # format, vet, build
go test ./...  # run all Go tests
```

Focused test examples:

```bash
go test ./internal/agent/domain/ -v
go test ./internal/tools/builtin/ -v
```

Release automation:

```bash
node scripts/update-version.js 0.x.x
make release-npm
```

## Documentation Map

### User Guides
- [QUICKSTART_SSE.md](QUICKSTART_SSE.md) — Web UI quick start (≈3 minutes)
- [DEPLOYMENT.md](DEPLOYMENT.md) — Production deployment
- [CLAUDE.md](CLAUDE.md) — Claude Code integration
- [docs/operations/README.md](docs/operations/README.md) — Operations & troubleshooting (v0.6.0)

### Reference
- [docs/reference/OBSERVABILITY.md](docs/reference/OBSERVABILITY.md) — Observability guide (v0.6.0)
- [docs/reference/PRESET_QUICK_REFERENCE.md](docs/reference/PRESET_QUICK_REFERENCE.md) — Agent preset quick reference (v0.6.0)
- [docs/reference/PRESET_SYSTEM_SUMMARY.md](docs/reference/PRESET_SYSTEM_SUMMARY.md) — Preset system internals (v0.6.0)
- [docs/reference/MCP_GUIDE.md](docs/reference/MCP_GUIDE.md) — Model Context Protocol usage

### Architecture & Design
- [docs/analysis/base_flow_architecture_review.md](docs/analysis/base_flow_architecture_review.md) — 2025 Q1 architecture review
- [docs/design/SSE_WEB_ARCHITECTURE.md](docs/design/SSE_WEB_ARCHITECTURE.md) — SSE web architecture
- [SSE_IMPLEMENTATION_SUMMARY.md](SSE_IMPLEMENTATION_SUMMARY.md) — Implementation summary
- [docs/architecture/](docs/architecture/) — System diagrams and notes

### Development References
- [internal/server/README.md](internal/server/README.md) — Server development guide
- [web/README.md](web/README.md) — Frontend development guide
- [evaluation/swe_bench/](evaluation/swe_bench/) — SWE-Bench evaluation suite

## License

MIT

---

Built with Go · [github.com/cklxx/Alex-Code](https://github.com/cklxx/Alex-Code)
