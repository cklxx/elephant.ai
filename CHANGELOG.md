# Changelog

All notable changes to ALEX will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2025-Q1 Architecture Improvements

### Added - Sprint 1-4 Features

**Sprint 1: Cost Isolation & Task Context**
- Context-aware task cancellation with `context.WithCancelCause`
- Task cancellation API endpoint (`POST /api/tasks/:id/cancel`)
- Cancel function tracking in ServerCoordinator for graceful task termination
- Enhanced task status tracking with cancellation reasons
- Cost tracking isolation per session to prevent concurrent session interference

**Sprint 2: Dependency Injection Decoupling & Configuration Flags**
- Feature flag: `EnableMCP` for selective dependency initialization
- Lazy initialization of the MCP registry (deferred to first use or explicit Start())
- Container lifecycle management with `Start()` and `Shutdown()` methods
- Health check system with pluggable probes (`/health` endpoint)
- Health probes for MCP registry and LLM factory
- Health status types: Ready, NotReady, Disabled
- Offline/testing mode support - tests run without API keys
- MCP initialization status tracking with retry logic and exponential backoff

**Sprint 3: Coordinator Dependency Refactoring**
- `PresetResolver` component for agent preset resolution
- Agent preset system with 5 specialized personas (default, code-expert, researcher, devops, security-analyst)
- Tool preset system with 5 access levels (full, read-only, code-only, web-only, safe)
- Filtered tool registry for preset-based access control
- Context-based preset passing via PresetContextKey
- Task metadata now includes agent_preset and tool_preset

**Sprint 4: Observability Enhancements**
- Comprehensive observability guide (`docs/reference/OBSERVABILITY.md`)
- Structured logging with context propagation
- Health check integration tests
- Enhanced error context with cancellation cause tracking
- Improved task lifecycle visibility in SSE events

**Previous Features**
- Environment-based configuration for web frontend (`.env.development`, `.env.production`)
- Research console-style terminal UI layout with persistent input
- User task display in event stream
- Terminal-style event output component with color-coded events
- Research plan approval UI integration
- Agent runtime ports for logger/clock abstraction and a `ReactiveExecutor` contract to enable typed mocking
- Session history pinning and renaming controls with localized copy

### Changed

**Sprint 1-4 Core Changes**
- Refactored `ServerCoordinator.ExecuteTaskAsync` to use derived contexts instead of `context.Background()`
- Task execution now properly propagates cancellation signals to agent coordinator
- Container construction separated from lifecycle initialization (Build vs Start)
- LLM factory and tool registry initialization deferred until needed
- Health check system integrated into server startup
- Task store now tracks cancellation metadata and reasons
- DI container supports minimal configuration for offline testing

**Previous Changes**
- **BREAKING**: Completely refactored deployment script (`deploy.sh`)
  - Simplified to focus on local development only
  - Added port conflict detection and cleanup
  - Implemented PID-based process management
  - Added log rotation and health checks
  - Removed Docker and Kubernetes logic
- Refactored web frontend layout following the research console design pattern
  - Three-section flexbox: header (fixed) → output (scrollable) → input (fixed)
  - Persistent task input always visible at bottom
  - Auto-scroll to latest events
  - Horizontal input layout with auto-resize textarea
- Reimagined console home to match the new research workspace reference
  - Hero card greets the user, surfaces quick actions, and embeds the task input
  - Left rail consolidates connection status, timeline progress, and pinned/recent sessions
  - Right guidance rail highlights quick starts and timeline messaging with reduced copy
  - Updated translations, Playwright layout spec, and documentation to reflect the lighter style
- Split the marketing homepage from the research console so the hero CTA links directly to the dedicated conversation view
- Added live "Doing something…" badges to tool start events so ongoing agent actions read naturally in the chat stream
- Quickstart panel buttons now prefill the chat input and focus the composer for faster task setup
- Tool call timelines highlight active steps with animated markers and elapsed timing metadata inside the transcript
- Fixed event display to use correct `event_type` field
- Updated all event formatting with proper type narrowing
- Migrated to Zustand v5 API in `useAgentStreamStore`
- React engine now constructed via `ReactEngineConfig`, receiving injected logger/clock dependencies and emitting timestamped events
- Agent coordinator delegates preparation to new execution/task-analysis services, reducing orchestration surface area and stabilising cost tracking

### Fixed

**Sprint 1-4 Fixes**
- Fixed cost tracking interference between concurrent sessions
- Fixed task context loss in background execution
- Fixed health check failures when MCP is disabled
- Fixed container initialization failures in offline/test environments
- Fixed cancel function memory leaks after task completion

**Previous Fixes**
- Input box disappearing after task submission
- Event content not displaying (wrong field access)
- User messages not shown in event stream
- TypeScript compilation errors in event handling
- Infinite re-render loops in SSE hook
- Port conflicts on frontend startup
- Next.js webpack cache corruption

### Removed
- `.env.local` file (replaced with environment-specific files)
- Complex Docker/Kubernetes deployment logic from `deploy.sh`
- Unnecessary summary documentation files

## [0.5.3] - 2025-10-05

### Fixed
- Infinite re-render loop in `useSSE` hook
- TypeScript inference error in `useMemoryStats` hook
- Critical security and code quality issues

## [0.5.2] - Earlier

See git history for earlier changes.

---

## Migration Notes

### Sprint 1-4 Changes (v0.6.0)

#### Configuration Flags (Sprint 2)
New feature flags in `internal/di/container.go`:

```go
Config{
    EnableMCP:      true,  // Enable Model Context Protocol integration
}
```

**Breaking Change**: If you're constructing `di.Container` directly, you must now call `Start()` after `BuildContainer()`:

```go
container, err := di.BuildContainer(config)
if err != nil {
    return err
}
defer container.Shutdown()

// Initialize heavy dependencies
if err := container.Start(); err != nil {
    log.Warn("Some features may be unavailable: %v", err)
}
```

**Minimal Configuration for Testing/Offline**:
```go
config := di.Config{
    EnableMCP:      false,  // Disable MCP to avoid external dependencies
}
container, _ := di.BuildContainer(config)
// No API keys needed, tests will pass
```

#### Health Check Endpoint (Sprint 2)
New endpoint available at `/health`:

```bash
curl http://localhost:8080/health
```

Returns component health status:
```json
{
  "status": "healthy",
  "components": [
    {"name": "llm_factory", "status": "ready"},
    {"name": "mcp", "status": "not_ready", "message": "..."}
  ]
}
```

#### Task Cancellation API (Sprint 1)
New cancellation endpoint:

```bash
curl -X POST http://localhost:8080/api/tasks/:id/cancel
```

Tasks now include cancellation status and reasons in responses.

#### Agent Presets (Sprint 3)
New optional fields in task creation:

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review code for security issues",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

See `docs/reference/PRESET_QUICK_REFERENCE.md` for available presets.

### Deployment Script Changes
The deployment script has been completely rewritten. New commands:

```bash
./deploy.sh start    # Start backend + frontend
./deploy.sh status   # Check service status
./deploy.sh logs     # Tail logs
./deploy.sh down     # Stop all services
```

Old Docker/Kubernetes commands are no longer supported. For production deployment, use container orchestration directly with the provided Dockerfile.

### Environment Configuration
Frontend now uses environment-specific files:

- **Development**: `web/.env.development` (used by `npm run dev`)
- **Production**: `web/.env.production` (used by `npm run build`)

The `.env.local` file is no longer used. Update your environment configuration in the appropriate file.

### Frontend Layout
The web UI has been redesigned with a terminal-style layout:
- Input is always visible at the bottom
- Events stream above with auto-scroll
- Minimalist design inspired by the research console reference experience
