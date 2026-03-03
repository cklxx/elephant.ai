# Changelog

All notable changes to elephant.ai are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Preanalysis emoji reaction listener for Lark messages — visual feedback before the ReAct loop starts
- `RecoverStalePending` — cancels orphaned pending dispatches on restart
- Per-dispatch timeout propagation with context error classification
- `DeleteReaction` + processing reaction lifecycle for Lark
- Claude-only team templates for agent orchestration

### Fixed
- Kernel: guard `MarkDispatchRunning` failure to skip execution instead of warn-and-continue
- Kernel: `sanitizeRuntimeSummary` now preserves valid content (K-05)
- Kernel: persist state after prune to prevent data loss on restart (K-03)
- Lark: use `context.WithoutCancel` for end-emoji reaction goroutine to avoid premature cancellation
- Session: persist execution errors and `stop_reason` to session metadata
- Workspace: harden merge conflict detection for CI environments

### Changed
- Refactored Lark tools to extract shared error helpers into `api_errors.go`
- Removed dead `sandboxPermError` helper and unused syscall import

---

## [0.3.0] — 2026-01

### Added
- Real bridge pipeline E2E tests covering P0–P2 features
- Integration tests for latest model generations
- `alex dev logs-ui` — log analyzer UI accessible from browser
- Lark supervisor with restart storm detection and exponential backoff
- PGID-based process group cleanup on shutdown (no orphan processes)

### Changed
- Replaced legacy `./dev.sh` with typed Go-based `alex dev` command suite
- Race-free port allocation via `net.Listen` reservation before service startup
- Atomic PID and supervisor state files (tmp + rename pattern)

---

## [0.2.0] — 2025-12

### Added
- Multi-provider LLM support: OpenAI, Anthropic, ByteDance ARK, DeepSeek, OpenRouter, Ollama
- ReAct agent loop with typed events and approval gates
- Persistent memory store (Postgres + file backends)
- Lark WebSocket gateway with auto-context injection
- Web Console (Next.js) with SSE streaming and artifact rendering
- Built-in skills: `deep-research`, `meeting-notes`, `email-drafting`, `ppt-deck`, `video-production`
- MCP (Model Context Protocol) client for external tool integrations
- OpenTelemetry traces + Prometheus metrics + per-session cost accounting

### Changed
- Context assembly refactored to layered retrieval (chat history → memory → policy)

---

## [0.1.0] — 2025-10

### Added
- Initial project scaffold: Go backend + Next.js frontend
- CLI entrypoint (`cmd/alex`) with interactive TUI
- Basic LLM chat loop with OpenAI and Anthropic
- File-based memory store
- Lark bot integration (polling mode)

---

[Unreleased]: https://github.com/cklxx/elephant.ai/compare/HEAD...main
