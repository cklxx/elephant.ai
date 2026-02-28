# 2026-02-28 — Codebase conciseness optimization across all architecture layers

Impact: Removed 208 net lines across 14 files (497 deleted, 289 added) with zero behavior change. Applied "if 30 lines can be 5, write 5" principle systematically across domain, infra, delivery, and app layers.

## What changed

### Domain layer (react/)
- Removed 12x redundant `rw == nil || rw.recorder == nil` guards in `workflow.go` — the `workflowRecorder` methods already nil-check themselves, so the caller guards were pure bloat.
- Extracted `toFloat64()` to collapse 7 numeric type cases repeated in `feedback.go` and reusable elsewhere.
- Extracted `truncateWithEllipsis()` to DRY 3 functions repeating identical truncate+append logic in `tool_args.go`.
- Extracted `transformMapValues()`/`transformSliceValues()` to eliminate duplicated changed-tracking loops in `compactToolArgumentValue`.
- Replaced 4-case switch with `var rolePrefixMap` map lookup in `context.go`.
- Simplified `summarizeWorldMetadata` by removing unnecessary key sorting (output was a map — iteration order is inherently non-deterministic).
- Replaced 10-line manual map merge in `decorateFinalResult` with existing `ports.MergeAttachmentMaps(dst, src, false)`.
- Inlined trivial 5-line wrapper `offloadToolResultAttachmentData` at its 2 call sites.

### Infra layer
- Replaced 14 if-blocks in `classifyLLMError` with a table-driven `[]errorClassificationRule` — each rule is `{patterns, permanent, message}`, collapsing 80 lines to 25.
- Collapsed verbose error-return patterns in `extractCostUSD`.

### Delivery layer
- Extracted `contextData(ctx)` helper to eliminate 5 repeated hierarchy fields (`level/agent_id/session_id/run_id/parent_run_id`) across 6 SSE render methods.
- Extracted `payloadString(e, key)` to DRY envelope payload extraction in progress listener.
- Extracted `doSend(text, messageID, logPrefix)` to eliminate duplicated send-or-update block in `flush()` and `Close()`.

### App layer
- Extracted pointer-based `enabled *bool` / `binary *string` approach to collapse 3x copy-pasted agent auto-enable blocks (codex/claude_code/kimi) into one loop body.
- Extracted `stringOr()`/`intOr()` helpers to replace 10x repeated `if x == "" { x = default }` pattern in kernel engine builder.

## Why this worked

1. **Subagent-driven scan first, manual verification second** — Used explore subagents to scan all 4 architecture layers for patterns, then read every candidate file to confirm before editing. Zero false positives.
2. **Batch-by-layer with build+test gates** — Each batch was independently buildable and testable. Failures were caught immediately, not deferred to the end.
3. **Worktree isolation** — All changes made in a worktree, rebased onto advancing main before ff-merge. Clean separation from concurrent work.
4. **Real E2E validation via Lark inject** — After push, restarted backend and injected real Lark messages that exercised tool execution, progress listener, workflow recording, feedback signals, and result finalization. Both tests returned correct responses.
5. **Mechanical transformations only** — Every change was a provably behavior-preserving refactoring: extract helper, table-drive, inline trivial wrapper, replace manual merge with existing utility. No logic changes.

## Key patterns to reuse

- **Redundant nil guards**: When a method receiver already nil-checks, callers don't need to. Check the callee before adding guards.
- **Table-driven classification**: Any chain of 5+ if-blocks doing pattern matching + constructor dispatch → extract a rule table.
- **Pointer-based DRY for config structs**: When 3+ switch cases do identical operations on different struct fields, take `*bool`/`*string` pointers to the fields and operate once.
- **"Already exists" check**: Before writing any helper, grep for existing utilities (`MergeAttachmentMaps` was already there but unused in `decorateFinalResult`).

## Validation

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/domain/agent/react/... -count=1` — pass
- `go test ./internal/infra/llm/... -count=1` — pass
- `go test ./internal/infra/tools/... -count=1` — pass
- `go test ./internal/delivery/... -count=1` — pass (1 pre-existing failure unrelated)
- `go test ./internal/app/... -count=1` — pass (1 pre-existing failure unrelated)
- `alex dev restart backend` + `alex lark inject` — 2 real E2E tests passed

## Metadata
- id: good-2026-02-28-codebase-conciseness-optimization
- tags: [good, conciseness, readability, maintainability, refactoring, cross-layer, DRY]
- links:
  - internal/domain/agent/react/workflow.go
  - internal/domain/agent/react/feedback.go
  - internal/domain/agent/react/tool_args.go
  - internal/domain/agent/react/context.go
  - internal/domain/agent/react/finalize.go
  - internal/domain/agent/react/observe.go
  - internal/domain/agent/react/world.go
  - internal/infra/llm/retry_client.go
  - internal/infra/tools/sla_executor.go
  - internal/delivery/output/sse_renderer.go
  - internal/delivery/channels/lark/progress_listener.go
  - internal/app/di/container_builder.go
  - internal/app/di/builder_hooks.go
