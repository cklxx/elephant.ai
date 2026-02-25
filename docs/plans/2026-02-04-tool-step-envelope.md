# Plan: Fix missing tool-step completion envelope (FAST/SLOW gate)

Date: 2026-02-04

## Context
- Failing test: `TestExecuteTaskRunsToolWorkflowEndToEnd` expects a `workflow.node.completed` envelope for tool-call step `react:iter:1:tool:call-1`.
- Goal: restore expected envelope emission with minimal/local change.

## Steps
1. Reproduce the failing test and inspect emitted envelopes.
2. Identify where the tool-call node completion event is dropped/filtered.
3. Implement a minimal fix to ensure the tool-call node emits a `workflow.node.completed` envelope.
4. Add/adjust regression coverage if needed.
5. Run full lint + tests; commit; merge back to `main`.

## Progress Log
- 2026-02-04: Started investigation from failing coordinator tool workflow test.
- 2026-02-04: Reproduced flake under `go test -shuffle=on` (missing envelopes due to async event delivery).
- 2026-02-04: Added `SerializingEventListener.Flush` barrier + coordinator deferred flush to ensure envelopes are delivered before `ExecuteTask` returns.
- 2026-02-04: Added unit test for `Flush`; ran `make fmt vet test`.
