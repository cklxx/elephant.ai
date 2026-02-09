# Plan: Product Tool-Routing Optimization (R15)

Owner: cklxx  
Date: 2026-02-10  
Worktree: `/Users/bytedance/code/elephant.ai-wt-r15-opt-all-20260210-004200`  
Branch: `feat/agent-systematic-optimization-r15-20260210-004200`

## Goal
Improve real product capability (not eval-only scoring) by tightening system routing guidance and tool definition boundaries in production paths, then verify with full tests and foundation-suite metrics.

## Scope
- Update production prompt routing guidance:
  - `internal/shared/agent/presets/prompts.go`
  - `internal/app/context/manager_prompt.go`
- Update production tool definitions to reduce high-frequency conflict pairs:
  - files vs memory (`read_file` vs `memory_get`)
  - semantic search vs visual capture (`search_file` vs `browser_screenshot`)
  - in-place patch vs cleanup (`replace_in_file` vs `artifacts_delete`)
  - deterministic compute vs browser/calendar (`execute_code` vs `browser_action`/`lark_calendar_query`)
  - scheduler inventory/delete boundaries
- Add/extend tests for routing descriptions and prompt guardrails.
- Run full lint + full test + foundation-suite validation.

## Progress
- [x] Reverted temporary eval-only heuristic edits to avoid metric-only optimization.
- [x] Implemented product-side routing updates in prompt/context and tool definitions (local + sandbox variants).
- [x] Added/updated routing boundary tests across aliases/sandbox/browser/lark/scheduler/memory/context/presets.
- [x] Ran targeted package tests for changed modules.
- [x] Ran foundation-suite before/after comparison.
- [x] Ran full lint and `go test ./...`.

## Validation Snapshot
- Baseline (before product routing edits): `pass@1 207/257`, `pass@5 243/257`, `failed=14`.
- After product routing edits (no eval heuristic changes): `pass@1 216/257`, `pass@5 257/257`, `failed=0`.

