# E2E Status Mapping and Path Automation (2026-02-14)

## Goal
- Fix false-positive `completed` statuses for internally failed ReAct runs.
- Reduce path discovery / ad-hoc cloning churn in real SWE-Bench E2E runs by injecting deterministic workspace guidance.
- Align quality scoring with real workflow outcome signals.

## Scope
- `evaluation/swe_bench/alex_agent_integration.go`
- `evaluation/swe_bench` tests (new/updated)
- `evaluation/agent_eval/metrics.go`
- `evaluation/agent_eval` tests (new/updated)

## Plan
1. Add status mapping from workflow end-state:
   - If workflow phase is failed or execute node stop is `max_iterations`, do not mark task as `completed`.
   - Preserve timeout / execution errors as existing behavior.
2. Add deterministic workspace hint in SWE-Bench prompt:
   - Include explicit local repo candidate paths and “prefer existing local checkout” guidance.
   - Keep prompt backward-compatible and concise.
3. Improve quality scoring heuristics:
   - Penalize quality when workflow indicates failure-like termination.
   - Avoid fixed high quality purely based on text length.
4. Add unit tests first for all above logic; implement until green.
5. Run targeted Go tests + real subscription E2E spot check.

## Progress
- [x] Create isolated worktree branch and copy `.env`.
- [x] Read engineering practices and collect baseline failure evidence.
- [x] Implement status mapping + tests.
- [x] Implement prompt path automation + tests.
- [x] Implement quality scoring adjustment + tests.
- [x] Run tests and real E2E validation.
- [ ] Commit incremental changes and summarize results.

## Validation Criteria
- A workflow ending with `max_iterations` no longer appears as successful completion.
- Prompt includes deterministic local-path guidance.
- Quality score differentiates true completions from workflow-failed completions.
- Real E2E spot run shows corrected status semantics in `detailed_results.json`.
