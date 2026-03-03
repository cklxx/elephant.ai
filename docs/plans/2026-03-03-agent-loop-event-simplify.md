# Agent Loop / Event System Simplification Plan (2026-03-03)

## Context
- Scope: simplify `agent loop` and `event system` paths that accumulated defensive/duplicated logic.
- Inputs: parallel codebase audits from two explorer subagents on `internal/domain/agent/react`, `internal/app/agent/coordinator`, and delivery event adapters.

## Goals
1. Remove duplicated filtering and repeated map lookups in hot paths.
2. Replace string-matching error branching with typed sentinel errors.
3. Remove dead abstractions/unused dependencies in workflow event translation.
4. Eliminate no-op category branching in LLM tool-result rendering.
5. Prevent async session-save loop from being tied to a single request cancellation.

## Non-Goals
- No protocol-level rewrite of event contract (`domain.Event` vs `WorkflowEventEnvelope`) in this change.
- No cross-layer architecture shifts.

## Change Set
- [x] `internal/domain/agent/react/runtime.go`
  - Remove duplicate tool-call filtering in `planTools`.
  - Remove low-value nil-receiver guards in internal runtime helpers.
- [x] `internal/domain/agent/react/background.go`
  - Add typed not-found sentinel for `CancelTask`.
  - Merge duplicate task lookup blocks in `ReplyExternalInput`.
- [x] `internal/app/agent/coordinator/background_registry.go`
  - Use `errors.Is` against typed not-found sentinel.
- [x] `internal/app/agent/coordinator/session_manager.go`
  - Start save loop with `context.WithoutCancel(ctx)` to decouple from first request cancellation.
- [x] `internal/app/agent/coordinator/workflow_event_translator*.go`
  - Remove unused translator logger dependency.
  - Inline trivial `translateTool` forwarding wrapper.
- [x] `internal/delivery/output/llm_renderer.go`
  - Collapse category switch in `formatToolResult` to single output path.

## Validation
- Targeted:
  - `go test ./internal/domain/agent/react -run CancelTask`
  - `go test ./internal/app/agent/coordinator -run 'WorkflowEventTranslator|Session|Background'`
  - `go test ./internal/delivery/server/app -run TaskProgressTracker`
  - `go test ./internal/delivery/output`
- Broad:
  - `go test ./internal/app/agent/coordinator ./internal/domain/agent/react ./internal/delivery/server/app ./internal/delivery/output`

## Progress Log
- 2026-03-03: plan created; implementation in progress.
- 2026-03-03: simplifications implemented and validated (`make fmt`, targeted package tests, `go test ./...`, code-review skill).
