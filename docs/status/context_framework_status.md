# Context Framework Implementation Status

_Last updated: 2024-05-21._

This document tracks how much of the three-layer context architecture (see `docs/design/agent_context_framework.md`) is live in the current codebase. It is referenced by the CLI when snapshot data is missing so operators can quickly see why certain fields are empty.

## ✅ Landed slices
- **Static registry cache** – `internal/context/manager.go` now loads personas, goals, policies, knowledge packs, and worlds from `configs/context/*` with TTL-based caching and world selection fallbacks.
- **Session snapshot store** – `internal/session/state_store` provides in-memory/file-backed implementations, while `internal/context/manager.go` persists every turn via `RecordTurn`.
- **Server/API + CLI surfaces** – `internal/server/http/api_handler.go` exposes `/api/sessions/:id/snapshots` and `/turns/:turn_id` for pagination, and `cmd/alex/cli.go` ships the `alex sessions pull` command (with JSON export) for local operators.
- **Task-analysis derived plans/beliefs** – `internal/agent/app/execution_preparation_service.go` converts task-analysis steps and criteria into `PlanNode`/`Belief` slices on `TaskState`, and `internal/agent/domain/react_engine.go` persists them with every `RecordTurn` call.
- **Knowledge reference seeding** – retrieval guidance from task analysis is now turned into `KnowledgeReference` entries so CLI/API callers can inspect which local queries, search terms, and gaps were suggested per turn.
- **World state + diff ingestion** – the preparation service seeds each session with the selected `WorldProfile`, while the React engine records structured tool-result observations so snapshots expose `world_state` and `diff` data on every turn.
- **Feedback capture** – tool execution now emits lightweight `FeedbackSignal` entries (including reward values from metadata), enabling APIs/CLI to inspect loop-level reward traces alongside plan and knowledge metadata.

## ⚠️ Outstanding gaps
- **Turn journals + replay hooks** – the event-log/`turn_journal` pipeline described in the design doc (Section 4) has not been implemented; `internal/session/api/snapshots.go` is still a placeholder.
- **Meta-context steward** – memory selection, knowledge governance, and persona evolution jobs (Section 5) remain unstarted; no `cmd/context-steward` binary exists yet.

## Tracking guidance
- When a snapshot omits the fields above, the CLI prints a reminder pointing back to this document.
- Update this status doc whenever a new slice ships so the remaining gaps stay visible to engineers, PMs, and operators.
