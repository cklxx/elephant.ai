# Refactor Ledger

This document tracks architectural and design issues discovered during the ongoing refactor work, plus the proposed fixes and their status.

## Goals

- Preserve behavior while improving layering, dependency direction, and maintainability.
- Prefer small, reviewable refactors with tests kept green.
- Execute in dependency order (shared contracts → core logic → adapters → entrypoints).

## Backend (Go)

### 1) DI config mapping duplicated across entrypoints

- **Symptoms**: `cmd/alex/container.go` and `cmd/alex-server/main.go` both hand-map `internal/config.RuntimeConfig` into `internal/di.Config`.
- **Impact**: Drift risk; harder to evolve runtime config and container wiring safely.
- **Fix**: Add a single mapping helper in `internal/di` (runtime → di.Config) and reuse it.
- **Status**: done

### 2) Entry-point bootstrap logic too concentrated

- **Symptoms**: `cmd/alex-server/main.go` was ~700+ LOC and mixed config, env snapshots, observability, auth bootstrap, server wiring, and process lifecycle.
- **Impact**: Hard to test; hard to reason about failure modes; makes incremental changes risky.
- **Fix**: Extract bootstrap modules (config/load, env snapshot, server wiring) into `internal/server/...` packages with unit tests.
- **Status**: in progress (moved config/auth/env/analytics/journal bootstrap into `internal/server/bootstrap`; `main.go` now ~280 LOC)

### 3) Logging split across `internal/utils` and `internal/observability`

- **Symptoms**: Two logger implementations with different semantics (printf-style vs slog structured), plus domain-level `ports.Logger` interface.
- **Impact**: Inconsistent log formatting/context fields; makes observability adoption uneven.
- **Fix**: Define a single “application logger” adapter that implements `ports.Logger` and can be backed by slog, keeping file-based debug logging as an optional sink.
- **Status**: planned

### 4) Auth for SSE relies on `access_token` query parameter

- **Symptoms**: `internal/server/http/middleware.go` falls back to `?access_token=...`; frontend sets it in `web/lib/api.ts`.
- **Impact**: Tokens in URL can leak via logs/referers; harder to operate securely.
- **Fix**: Prefer cookie-based SSE auth (or short-lived signed SSE token) and keep query-token as legacy fallback.
- **Status**: in progress (added HttpOnly access-token cookie support; frontend omits query token for same-origin SSE)

## Frontend (web/)

### 1) Tracked env files cause git pollution

- **Symptoms**: `web/.env.development` and `web/.env.production` are tracked, while `dev.sh`/`deploy.sh` auto-create/patch `web/.env.development`.
- **Impact**: Running scripts can dirty the working tree; increases merge churn.
- **Fix**: Untrack/remove committed `web/.env.development` and `web/.env.production`, rely on scripts and/or `.env.local.example` as template.
- **Status**: done

### 2) Documentation drift (architecture/structure summaries)

- **Symptoms**: `web/README.md`, `web/STRUCTURE.md`, `web/PROJECT_SUMMARY.md` claim Next.js 14 / no tests / small file tree, but repo has Next 16 + tests + expanded event pipeline.
- **Impact**: Misleads contributors; slows onboarding; causes incorrect assumptions.
- **Fix**: Update or replace these docs to reflect current code (SSE pipeline, auth, test layout, versions).
- **Status**: done

### 3) Client timer typing uses `NodeJS.Timeout` in browser hooks

- **Symptoms**: hooks use `useRef<NodeJS.Timeout | null>` (e.g. `web/hooks/useSSE.ts`).
- **Impact**: Minor typing friction; can confuse DOM-vs-Node runtime expectations.
- **Fix**: Use `ReturnType<typeof setTimeout>` / `ReturnType<typeof setInterval>` in client code.
- **Status**: done

### 4) Small UI regressions / dead code in agent timeline components

- **Symptoms**:
  - `IntermediatePanel` computed preview text but never rendered it.
  - Subagent header computed concurrency but didn’t display it.
  - `ToolOutputCard` computed `previewText` but didn’t surface it in the collapsed header.
  - Artifact card actions were icon-only and missing accessible labels / “View” affordance.
- **Impact**: Lower UX clarity; tests brittle; real attachment picker path broke.
- **Fix**: Render previews/labels explicitly and restore missing file input wiring.
- **Status**: done

## Proposed Execution Order (next batches)

1. Untrack/cleanup env files (`web/.env.*`) to stop git pollution. ✅
2. Add DI runtime→container mapping helper; replace duplicated mappings. ✅
3. Tighten docs to match current frontend/backend behavior. ✅
4. (Optional) Split server bootstrap into testable packages.
5. (Optional) Rework SSE auth away from query tokens.
