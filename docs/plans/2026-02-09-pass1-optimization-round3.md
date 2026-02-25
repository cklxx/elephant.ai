# 2026-02-09 pass@1 Optimization Round 3

**Status**: Completed  
**Branch**: `feat/pass1-opt-round3-20260209`  
**Updated**: 2026-02-09

## Goal
- Continue improving pass@1 from `390/505` on the harder suite while preserving `pass@5=505/505`.

## Workstreams
- [x] Cluster top1 misses by expected-vs-top1 conflict pairs.
- [x] Add targeted heuristic boosts/penalties for high-frequency collision pairs.
- [x] Extend regression tests for new heuristics.
- [x] Run full foundation-suite and compare deltas.
- [x] Run formatting and full test validation.

## Targeted Fixes
- `request_user` vs `clarify`: penalize `clarify` under manual-approval gate language.
- `lark_upload_file` vs `write_attachment`: boost upload-to-thread semantics; penalize attachment tool under explicit lark upload context.
- `memory_search` vs `search_file`: penalize file search under memory-recall intent terms.
- `cancel_timer` vs `set_timer`: add explicit cancellation/cleanup boosts and set-timer penalties for stale/duplicate cleanup intent.

## Result
- New run: `foundation-suite-20260209-051143`
- pass@1: `398/505` (`+8` vs round2)
- pass@5: `505/505` (unchanged)
- failed: `0`

## Acceptance
- pass@1 improved with hard-suite intact: ✅
- pass@5 remained perfect: ✅
- full format/tests pass: ✅
