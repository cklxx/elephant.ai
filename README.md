# ALEX

Terminal-native AI programming agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Philosophy

**保持简洁清晰，如无需求勿增实体**

Built with hexagonal architecture, clean separation of concerns, and focus on essential complexity only.

## Features

- **Interactive Modes**
  - Terminal CLI with streaming output
  - Web UI with real-time SSE events
  - Command-line one-shot execution

- **15+ Built-in Tools**
  - File ops, shell, search, git, web
  - Think, subagent for complex reasoning

- **Multi-Model Support**
  - OpenAI, DeepSeek, OpenRouter, Ollama

- **Session Management**
  - Persistence and resumption
  - Session forking and branching

- **Web Interface** 🆕
  - Real-time event streaming (SSE)
  - Visual tool execution display
  - Interactive task management
  - Markdown rendering with syntax highlighting

## Installation

### CLI Installation

**NPM**
```bash
npm install -g alex-code
```

**From Source**
```bash
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code && make build
```

**Releases**
[github.com/cklxx/Alex-Code/releases](https://github.com/cklxx/Alex-Code/releases)

### Web Server + UI (Docker Compose) 🆕

```bash
# Clone repository
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code

# Set API key
echo "OPENAI_API_KEY=sk-your-key" > .env

# Start all services
docker-compose up -d

# Access Web UI at http://localhost:3000
```

See [QUICKSTART_SSE.md](QUICKSTART_SSE.md) for details.

## Usage

### CLI Mode

**Interactive Mode**
```bash
alex
```

Key shortcuts:

- `Tab` / `Shift+Tab` – move focus between transcript, stream, tools, subagents, MCP, and input panes.
- `PgUp` / `PgDn`, arrow keys – scroll the focused pane; `End` jumps back to live updates.
- `?` – toggle an in-terminal help overlay with all shortcuts.
- `/` – search within the focused pane; `n`/`N` moves to the next/previous match.
- Slash commands:
  - `/new` – start a fresh session and clear the transcript.
  - `/sessions` – list saved sessions with the active one highlighted.
  - `/load <id>` – load a prior session into the UI.
  - `/mcp [list|refresh|restart <name>]` – inspect or restart MCP servers without leaving the chat.
  - `/cost [session_id]` – display the latest cost totals for the active (or specified) session directly in the transcript.
  - `/export [path]` – write the visible transcript to a Markdown file (defaults to `session-…-transcript-<timestamp>.md`).
  - `/follow <transcript|stream|both> <on|off|toggle>` – inspect or persist the default auto-follow behaviour from inside the TUI.
- `/verbose [on|off|toggle]` – inspect or change the CLI verbose flag that controls tool logging detail.
- `/quit` – exit the session (or press `Ctrl+C`).
- The Subagents pane now highlights each worker's agent level, start/finish times, and run duration so you can spot slow or failing subtasks at a glance.
- The status bar shows live token totals alongside per-model LLM spend so you can keep an eye on usage costs during long sessions.
- Configure default auto-follow behaviour either via the `/follow` command in the TUI, by editing `follow_transcript` / `follow_stream` in `~/.alex-config.json`, or by setting the `ALEX_TUI_FOLLOW_TRANSCRIPT` / `ALEX_TUI_FOLLOW_STREAM` environment variables if you prefer panes to stay pinned after session resets.

Need the legacy line UI? Launch with:

```bash
alex --no-tui
```

or set `ALEX_NO_TUI=1` before running `alex`.

**Command Mode**
```bash
alex "analyze this codebase"
```

**Session Management**
```bash
alex -r session_id -i    # resume
alex session list        # list all
```

Launching the TUI without arguments automatically restores your most recent session so you can continue the previous conversation immediately.

### Web Server Mode 🆕

**Start Server**
```bash
# Option 1: Docker Compose
docker-compose up -d

# Option 2: From source
make server-run
cd web && npm run dev
```

**Access**
- Web UI: http://localhost:3000
- API: http://localhost:8080
- SSE Events: http://localhost:8080/api/sse?session_id=xxx

**API Examples**
```bash
# Submit task
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "What is 2+2?", "session_id": "demo"}'

# Watch SSE events
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=demo"
```

## Configuration

Alex creates `~/.alex-config.json` on first run:

```json
{
  "api_key": "sk-or-xxx",
  "base_url": "https://openrouter.ai/api/v1",
  "model": "deepseek/deepseek-chat-v3-0324:free"
}
```

Override with environment:
```bash
export OPENAI_API_KEY="your-key"
export ALEX_VERBOSE="1"
export ALEX_TUI_FOLLOW_TRANSCRIPT="0"   # disable transcript auto-follow
export ALEX_TUI_FOLLOW_STREAM="0"       # disable live stream auto-follow
```

Set the same defaults persistently by adding `"follow_transcript"` / `"follow_stream"` entries to `~/.alex-config.json`.

## Tools

File: `read` `write` `edit` `replace` `list`
Shell: `bash` `code_execute`
Search: `grep` `ripgrep` `find` `code_search`
Task: `todo_read` `todo_update`
Web: `web_search` `web_fetch`
Git: `commit` `pr` `history`
Reasoning: `think` `subagent`

## Architecture

```
Domain (Pure Logic)
  ↓ depends on
Ports (Interfaces)
  ↑ implemented by
Adapters (Infrastructure)
```

**Structure**
```
internal/
├── agent/
│   ├── domain/     # react_engine, tool_formatter
│   ├── app/        # coordinator
│   └── ports/      # interfaces
├── llm/            # openai, deepseek, ollama
├── tools/
│   ├── builtin/    # 15+ tools
│   └── registry.go
└── session/        # persistence
```

## Development

**Workflow**
```bash
make dev     # format, vet, build
make test    # all tests
```

**Testing**
```bash
go test ./internal/agent/domain/ -v
go test ./internal/tools/builtin/ -v
```

**Release**
```bash
node scripts/update-version.js 0.x.x
make release-npm
```

## Documentation

### User Guides
- [QUICKSTART_SSE.md](QUICKSTART_SSE.md) - 🆕 Web UI Quick Start (3 minutes)
- [DEPLOYMENT.md](DEPLOYMENT.md) - 🆕 Production Deployment Guide
- [CLAUDE.md](CLAUDE.md) - Claude Code Integration Guide

### Architecture & Design
- [docs/design/SSE_WEB_ARCHITECTURE.md](docs/design/SSE_WEB_ARCHITECTURE.md) - 🆕 SSE Architecture Design
- [SSE_IMPLEMENTATION_SUMMARY.md](SSE_IMPLEMENTATION_SUMMARY.md) - 🆕 Implementation Summary
- [docs/architecture/](docs/architecture/) - System Architecture

### Development
- [internal/server/README.md](internal/server/README.md) - 🆕 Server Development
- [web/README.md](web/README.md) - 🆕 Web Frontend Development
- [evaluation/swe_bench/](evaluation/swe_bench/) - SWE-Bench Evaluation

## License

MIT

---

Built with Go · [github.com/cklxx/Alex-Code](https://github.com/cklxx/Alex-Code)
