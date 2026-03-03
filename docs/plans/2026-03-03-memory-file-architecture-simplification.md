# Plan: Memory + File-Based Orchestration Simplification

Date: 2026-03-03  
Owner: Codex

## Goal
- Review the current memory system and file-based orchestration architecture.
- Remove low-value complexity, over-defensive branches, and repetitive conversion logic without behavior regressions.

## Scope
- `internal/infra/memory/*`
- `internal/app/context/manager_memory.go`
- `internal/infra/tools/builtin/orchestration/*`
- `internal/domain/agent/taskfile/*`

## Steps
1. Parallel subagent architecture review for memory + orchestration paths.
2. Select safe simplifications (no API/contract changes).
3. Implement focused refactors and update tests.
4. Run targeted tests for changed packages.
5. Summarize simplifications + residual high-risk candidates.

## Progress
- [x] Baseline checks (`git diff --stat`, `git log --oneline -10`, docs/guides + memory summaries).
- [x] Subagent findings consolidated.
- [x] Refactors implemented.
- [x] Tests passing.
- [x] Final architecture simplification report delivered.

## Notes
- Existing unrelated local changes are present in coordinator/react files; do not touch unless required by this task.
