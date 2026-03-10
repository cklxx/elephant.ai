# Refactor Ledger

Updated: 2026-03-10

Tracks architectural issues, proposed fixes, and status. Prefer small, reviewable refactors in dependency order (shared contracts → core logic → adapters → entrypoints).

## Completed

| # | Issue | Fix |
|---|-------|-----|
| 1 | DI config mapping duplicated across entrypoints | Single mapping helper in `internal/di` |
| 9 | Duplicated bridge logic for external coding agents | Consolidated into `internal/infra/external/bridge/` |
| 10 | Kernel coupled to Lark process lifecycle | Separate managed component under shell supervisor |
| 11 | 8 independent store abstractions with overlap | Unified `internal/infra/filestore/` |
| F1 | Tracked env files cause git pollution | Untracked `web/.env.*` |
| F2 | Documentation drift (architecture/structure summaries) | Updated to match current code |
| F3 | Client timer typing uses `NodeJS.Timeout` | Uses `ReturnType<typeof setTimeout>` |
| F4 | Dead code in agent timeline components | Rendered previews/labels, restored file input |

## In Progress

### 2) Entry-point bootstrap too concentrated
- `cmd/alex-server/main.go` was ~700+ LOC mixing config, auth, wiring, lifecycle.
- Moved config/auth/env/analytics/journal + run loop into `internal/server/bootstrap`; `main.go` now delegates.

### 3) Logging split across `internal/utils` and `internal/observability`
- Introduced `internal/logging` with printf-style interface + adapters.
- Migrated server bootstrap/app/http, internal/errors, llm, mcp, di, context, session/filestore, tools/builtin, agent/app.

### 4) SSE auth via `access_token` query parameter
- Added HttpOnly cookie support; frontend omits query token for same-origin SSE.
- Query-token remains as legacy fallback.

### 5) HTTP delivery depends on builtins + domain formatter
- Formatter moved to presentation; SSE handler uses `agent.SubtaskWrapper`.

### 6) Builtin tools package is high fan-out monolith
- Extracted shared/pathutil helper subpackages; updating call sites and tests.

### 12) Domain layer Lark concept leakage
- See `docs/plans/2026-02-17-pr1-domain-decouple-lark.md`.

## Open

| # | Issue | Proposed fix |
|---|-------|-------------|
| 7 | Domain layer contains presentation + log-serialization logic | Move formatting/redaction to output/logging adapters |
| 8 | Large-file hotspots (700–1600 LOC) in core packages | Split by responsibility |

## Next Batches

1. Complete domain-layer Lark decoupling (PR1) 🔄
2. Split god structs (AgentCoordinator, ReactEngine, Gateway) 📋
3. Unify event system decorator chain 📋
