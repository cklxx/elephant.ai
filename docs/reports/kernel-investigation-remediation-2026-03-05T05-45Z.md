# Kernel Investigation & Remediation Plan — 2026-03-05T05:45Z

## Bottom Line
Kernel instability is currently dominated by **two non-code-quality issues**: (1) stale validation target drift (`larktools` path removed) and (2) upstream/model-route fragility (`openai/kimi-for-coding` think-step failures causing full fan-out failure).

## What Was Verified
1. **Repo/runtime snapshot**
   - Branch `main`, HEAD `fd207415`, origin divergence `0/0`.
   - Working tree dirty (`STATE.md`, `web/lib/generated/skillsCatalog.json`, untracked reports).
2. **Validation target drift confirmed**
   - `./internal/infra/tools/builtin/larktools/...` no longer exists; commands fail with `lstat ... no such file or directory`.
   - Active package path is `./internal/infra/lark/...` and tests/lint pass.
3. **Deterministic baseline passes**
   - `go test -count=1` passes for:
     - `./internal/infra/teamruntime/...`
     - `./internal/app/agent/...`
     - `./internal/infra/kernel/...`
     - `./internal/infra/lark/...`
   - `golangci-lint run ./internal/infra/lark/...` passes.
4. **Failure signature remains operational**
   - Latest kernel runtime block still records 5/5 agent failures in cycle `run-Psf4prIqoCXU` with `think step failed: LLM call failed: [openai/kimi-for-c...]`.

## Root-Cause Hypothesis (Ranked)
1. **High confidence:** stale validation references produce false blocker noise and hide real status.
2. **Medium-high confidence:** planner/executor model resolution still falls back to default `openai/kimi-for-coding` in some cycles (selection resolution mismatch/credential mismatch path), re-exposing known upstream instability.
3. **Medium confidence:** no pre-dispatch provider health gate, so one upstream failure can cascade into all-agent fan-out failure.

## Actionable Fix Plan

### P0 — Stop false failures (today)
- Replace all kernel automation references from:
  - `./internal/infra/tools/builtin/larktools/...`
- To canonical set:
  - `go test -count=1 ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/... ./internal/infra/lark/...`
  - `golangci-lint run ./internal/infra/lark/...`
- Acceptance criteria:
  - No `lstat ... larktools` in cycle logs for 24h.

### P0 — Add pre-dispatch LLM health gate (today)
- Before fan-out dispatch, run one lightweight planner/provider probe.
- On failure: switch cycle mode to deterministic audit lane (git/test/lint/state-write) instead of dispatching all agents.
- Acceptance criteria:
  - During upstream outage, failed agents per cycle drops from `N` to `0` (dispatch skipped intentionally), with explicit degraded-mode status.

### P1 — Resolve selection fallback ambiguity (next)
- Add structured logging at planner/executor LLM resolution point:
  - selected scope key
  - resolve success/fail
  - final provider/model used
  - fallback reason
- Acceptance criteria:
  - Every cycle has one unambiguous `final_llm_profile` log record.

### P1 — Working-tree hygiene lane (next)
- Isolate generated drift handling for `web/lib/generated/skillsCatalog.json` (commit or revert policy).
- Acceptance criteria:
  - Kernel audit cycles run with stable baseline diff signal.

## Artifacts Produced in This Investigation
- `docs/reports/kernel-cycle-2026-03-05T05-39Z.md`
- `docs/reports/kernel-cycle-2026-03-05T05-41Z.md`
- `docs/reports/kernel-risks-next-actions-2026-03-05T05-39Z.md`
- `docs/reports/kernel-investigation-remediation-2026-03-05T05-45Z.md`

