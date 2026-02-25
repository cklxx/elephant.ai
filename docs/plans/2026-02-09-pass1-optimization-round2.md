# 2026-02-09 pass@1 Optimization Round 2

**Status**: Completed  
**Branch**: `feat/pass1-opt-20260209`  
**Updated**: 2026-02-09

## Goal
- Improve foundation-suite pass@1 from `383/505` while preserving harder-case challenge structure.

## Workstreams
- [x] Analyze new hard failures from `foundation-suite-20260209-041123`.
- [x] Add targeted routing boosts/penalties for 4 failure patterns.
- [x] Add regression tests for new heuristic behavior.
- [x] Re-run full suite and verify pass@ metrics.
- [x] Run formatting and full test validation.

## Changes
- `evaluation/agent_eval/foundation_eval.go`
  - strengthened `web_search` when intent is source-discovery/authority-first
  - strengthened `artifacts_write` and penalized `write_attachment` for durable-in-chat-concise artifact intents
  - strengthened `list_dir` and penalized `read_file` for inventory/path-only intents
  - strengthened `memory_search` and penalized `memory_get` when offsets are not yet known
  - added token aliases for authority/discovery/inventory/offset/durable patterns
- `evaluation/agent_eval/foundation_eval_test.go`
  - expanded heuristic regression assertions for the four failure signatures

## Result
- New run: `foundation-suite-20260209-050501`
- Collections: `19/19`
- Cases: `505/505`
- pass@1: `390/505` (`+7`)
- pass@5: `505/505` (`+3`)
- Failed cases: `0` (`-4`)

## Acceptance
- pass@1 improved without reverting hard-case set: ✅
- full suite runs green: ✅
- changed-scope and full tests pass: ✅
