# Go-native DevOps System — Replace All Shell Scripts

**Status:** Completed (Phase 1-5)
**Branch:** `feat/go-devops` (merged to `main`)
**Date:** 2026-02-08

## Summary

Replaced ~47 shell scripts (~5,000+ lines bash) with a Go-native DevOps system integrated as `alex dev` subcommands. 4,735 lines of Go across 25 new files.

## Phases

### Phase 1: Core Infrastructure [DONE]
- `internal/devops/service.go` — Service interface + ServiceState enum
- `internal/devops/config.go` — Unified config (struct tags → defaults/env/YAML)
- `internal/devops/docker/client.go` — DockerClient interface + CLI impl
- `internal/devops/process/manager.go` — PGID-based ProcessManager
- `internal/devops/port/allocator.go` — Race-free port allocation via net.Listen
- `internal/devops/health/checker.go` — HTTP/TCP/Process health probes
- `internal/devops/log/section.go` — Colored terminal SectionWriter
- `internal/devops/log/manager.go` — Log tail/rotation

### Phase 2: Service Implementations [DONE]
- `internal/devops/services/sandbox.go` — Docker container lifecycle
- `internal/devops/services/authdb.go` — PostgreSQL auth DB setup
- `internal/devops/services/backend.go` — Go build + server management
- `internal/devops/services/web.go` — Next.js dev server
- `internal/devops/services/acp.go` — ACP daemon (host mode)

### Phase 3: Orchestrator + CLI [DONE]
- `internal/devops/orchestrator.go` — Multi-service orchestration
- `cmd/alex/dev.go` — CLI entry (up/down/status/logs/restart/sandbox/test/lint)
- `cmd/alex/main.go` — Added `case "dev":` routing

### Phase 4: Lark Supervisor [DONE]
- `internal/devops/supervisor/restart_policy.go` — Time-windowed restart storm detection
- `internal/devops/supervisor/status.go` — Atomic JSON state file
- `internal/devops/supervisor/autofix.go` — Codex autofix triggering
- `internal/devops/supervisor/supervisor.go` — Tick-loop supervisor with backoff
- `cmd/alex/dev_lark.go` — Lark supervisor CLI

### Phase 5: Tests + Validation [DONE]
- `internal/devops/config_test.go` — Config defaults, env, YAML, state strings
- `internal/devops/port/allocator_test.go` — Reserve, release, concurrent
- `internal/devops/supervisor/restart_policy_test.go` — Policy, cooldown, reset
- `internal/devops/supervisor/status_test.go` — Atomic write/read roundtrip

## Validation
- `go test -race ./internal/devops/...` — All pass
- `go vet ./internal/devops/...` — Clean
- `go build ./cmd/alex/` — Success

## Key Design Decisions
- Import cycle fix: `health.Result` lives in health package (not devops)
- Docker interaction via CLI exec (no heavy client library)
- PGID-based process groups for clean shutdown
- Atomic file writes (tmp + rename) for status/PID files
- Exponential backoff for supervisor restarts (1<<(failCount-1), max 60s)

## Commits
1. `a13a030d` — Phase 1+2: core + services
2. `0f9d044c` — Phase 3: orchestrator + CLI
3. `b5519aac` — Phase 4: supervisor
4. `9faee255` — Phase 5: tests
