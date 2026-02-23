# 2026-02-23 Kernel Startup State & Logging Fix Plan

Created: 2026-02-23
Status: In Progress
Owner: Codex

## Background
- Symptom: kernel is not in startup state.
- Goal: diagnose by logs, fix startup-state failure, add missing high-signal logs, and remove noisy low-value logs.

## Active Memory (selected)
- Kernel cadence should stay conservative and runtime status should be observed via `STATE.md` runtime block.
- `alex-server kernel-once` is the deterministic way to validate kernel end-to-end behavior.
- Keep observability focused: high-signal logs over verbose detail logs.
- Configuration mismatches can silently block startup; precheck and actionable errors are needed.
- Engineering practice: correctness/maintainability first, TDD for logic changes, full lint/tests before delivery.

## Plan
1. Reproduce current startup-state issue and collect logs.
2. Trace startup-state decision path in code and identify root cause.
3. Implement fix for startup-state transition/health reporting.
4. Add/adjust logs: keep action-level and decision-level logs; trim noisy detail logs.
5. Add/adjust tests (TDD) for startup-state and logging behavior.
6. Run full lint + tests.
7. Run mandatory code review workflow and fix findings.
8. Commit incremental changes.
9. Merge branch back to `main` (fast-forward preferred) and remove temporary worktree.

## Progress Log
- 2026-02-23 22:36: Created worktree branch `fix/kernel-startup-log`; copied `.env`.
- 2026-02-23 22:37: Loaded engineering practices and memory summaries.
- 2026-02-23 22:42: Reproduced issue: kernel repeatedly marked `down` and restarted in supervisor logs.
- 2026-02-23 22:43: Root cause identified: `scripts/lark/cleanup_orphan_agents.sh` used prefix match and misclassified `alex-server kernel-daemon` as orphan main process, killing kernel.
- 2026-02-23 22:44: Implemented fix + observability cleanup:
  - tighten orphan candidate matching to exact managed commands only;
  - add restart diagnostics in supervisor (`reason=pid_missing|stale_pid|state_mismatch`);
  - make `notify.sh` JSON escaping portable to remove sed noise logs;
  - improve kernel readiness detection by checking both `lark-kernel.log` and `alex-kernel.log` from new log offsets.
- 2026-02-23 22:45: Targeted validation passed: orphan cleanup no longer includes kernel daemon; kernel survives cleanup; kernel restart reports ready.
