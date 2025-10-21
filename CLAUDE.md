# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**ALEX - Agile Light Easy Xpert Code Agent** is a terminal-native AI programming agent built in Go with hexagonal architecture, ReAct agent pattern, and comprehensive tooling.

## Quick Reference

### Essential Commands
```bash
make dev                     # Format, vet, build (main workflow)
make test                    # Run all tests
make build                   # Build ./alex binary
```

### Deployment (Local Development)
```bash
./deploy.sh start            # Start backend + frontend
./deploy.sh status           # Check service status
./deploy.sh logs             # Tail logs
./deploy.sh down             # Stop all services
```

**Note**: Deployment script refactored to focus on local development only. Removed Docker/K8s logic.

### Testing
```bash
go test ./internal/agent/domain/ -v      # Domain layer
go test ./internal/tools/builtin/ -v     # Builtin tools
```

### NPM Publishing
```bash
make npm-copy-binaries       # Copy binaries to npm packages
make npm-publish             # Publish to npm
```

## Architecture Index

**See detailed docs in:** `docs/architecture/SPRINT_1-4_ARCHITECTURE.md`

### Key Directories
- `internal/agent/domain/` - Pure business logic (ReactEngine, ToolFormatter)
- `internal/agent/app/` - Application services (Coordinator)
- `internal/agent/ports/` - Interfaces for adapters
- `internal/tools/builtin/` - 15+ built-in tools
- `internal/llm/` - LLM client adapters (OpenAI, DeepSeek, Ollama)
- `cmd/alex/` - CLI entry points (main.go, tui_modern.go, cli.go)
- `cmd/alex-server/` - Web server for SSE-based agent API
- `web/` - Next.js web frontend (research console UI)

### Hexagonal Architecture Layers
```
Domain (Pure Logic)
  ↓ depends on
Ports (Interfaces)
  ↑ implemented by
Adapters (Infrastructure: LLM, Tools, Session)
```

## Built-in Tools Index

**Location:** `internal/tools/builtin/`

**Tool Implementations:**
- File: `file_read.go`, `file_write.go`, `file_edit.go`, `list_files.go`
- Shell: `bash.go`, `code_execute.go`
- Search: `grep.go`, `ripgrep.go`, `find.go`
- Task: `todo_read.go`, `todo_update.go`
- Web: `web_search.go` (Tavily), `web_fetch.go` (15-min cache)
- Reasoning: `think.go`

**Tool Registration:** `internal/tools/registry.go` - Dynamic registration system

## Development Principles

**保持简洁清晰，如无需求勿增实体**

1. **Simplicity First** - Minimal necessary complexity
2. **Clear Naming** - Self-documenting code
3. **Minimal Configuration** - Avoid options unless essential
4. **Testing Required** - All new code needs tests

## Key Implementation Files

### ReAct Agent (Domain Layer)
- `internal/agent/domain/react_engine.go:33-181` - Main ReAct loop (SolveTask)
- `internal/agent/domain/tool_formatter.go` - Tool output formatting

### Tool System
- `internal/tools/registry.go` - Tool registration
- `internal/tools/builtin/validation.go` - Path security

### LLM Integration
- `internal/llm/factory.go` - Multi-model factory
- `internal/llm/openai_client.go` - OpenAI/OpenRouter/DeepSeek adapter

### TUI (Modern Streaming)
- `cmd/alex/tui_modern.go` - Clean streaming interface (no chat format)
- `cmd/alex/tui.go` - Original chat-style TUI (deprecated)

### Web Frontend (Next.js)
- `web/app/page.tsx` - Main page with the research console layout (header → output → input)
- `web/components/agent/TerminalOutput.tsx` - Terminal-style event display
- `web/components/agent/TaskInput.tsx` - Persistent input (always visible)
- `web/hooks/useSSE.ts` - SSE connection with auto-reconnect
- `web/hooks/useTaskExecution.ts` - Task submission API
- `web/lib/api.ts` - API client (uses `NEXT_PUBLIC_API_URL`)
- `web/lib/types.ts` - TypeScript event types (15+ event types)

**Environment Config:**
- `web/.env.development` - Dev mode (npm run dev)
- `web/.env.production` - Production mode (npm run build)
- No `.env.local` - use environment-specific files only

## Testing Requirements

**All new code MUST include comprehensive tests**

### Test Patterns
```go
// Domain layer - dependency injection
func TestReactEngine_SolveTask(t *testing.T) {
    engine := NewReactEngine(5)
    mockLLM := &MockLLM{}
    mockTools := &MockToolRegistry{}
    // ... test logic
}

// Builtin tools - with test helpers
func TestFileRead_Execute(t *testing.T) {
    tool := NewFileRead()
    result, err := tool.Execute(ctx, call)
    // ... assertions
}
```

## Commit Standards

### Commit Message Format
```
<type>: <concise description>

<optional detailed explanation>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
Co-Authored-By: Claude <noreply@anthropic.com>
```

## Important Reminders

**From project instructions:**
- Do what has been asked; nothing more, nothing less
- NEVER create files unless absolutely necessary
- ALWAYS prefer editing existing files to creating new ones
- NEVER proactively create documentation files unless explicitly requested
- All new functionality MUST include comprehensive test coverage

## Additional Documentation

- **Detailed Architecture:** `docs/architecture/ALEX_DETAILED_ARCHITECTURE.md`
- **SWE-Bench Evaluation:** `evaluation/swe_bench/README.md`
- **Project README:** `README.md` - User-facing documentation
- **Changelog:** `CHANGELOG.md` - Version history and migration notes