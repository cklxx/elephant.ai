# Kernel Risks & Next Actions — 2026-03-05T05:39Z

## Validated Risks
1. **Stale validation target (active)**
   - Signal: `go test ./internal/infra/tools/builtin/larktools/...` fails with `lstat ... no such file or directory`.
   - Impact: false-negative kernel cycles; real regressions can be masked by path error.
   - Decision: treat `internal/infra/lark` as canonical runtime validation target.

2. **Runtime dispatch reliability (active)**
   - Signal (from kernel runtime state): latest cycle `run-Psf4prIqoCXU` failed `5/5` agents with `LLM call failed: [openai/kimi-for-c...]`.
   - Impact: autonomous multi-agent cycles can stall despite healthy code/test baseline.
   - Decision: keep deterministic local validation lane independent from external LLM availability.

3. **Working tree drift (active, low-medium)**
   - Signal: modified `STATE.md`, modified `web/lib/generated/skillsCatalog.json`, untracked cycle reports.
   - Impact: noisy diffs and audit contamination across cycles.
   - Decision: isolate generated docs/artifacts and classify `skillsCatalog.json` drift next cycle.

## Completed This Cycle
- Re-ran kernel baseline tests: `teamruntime + app/agent + kernel + lark` ✅
- Re-ran lint on active lark package ✅
- Confirmed docx markdown->blocks convert route coverage exists in `internal/infra/lark/docx_test.go` ✅
- Updated `STATE.md` latest audit and artifact references ✅
- Wrote cycle report: `docs/reports/kernel-cycle-2026-03-05T05-39Z.md` ✅
- Wrote evidence bundle: `artifacts/kernel_cycle_20260305T0538Z/*` ✅

## Next Autonomous Actions
1. Replace stale `larktools` validation targets in scripts/templates with canonical set:
   - `go test ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/... ./internal/infra/lark/...`
   - `golangci-lint run ./internal/infra/lark/...`
2. Add a single-source kernel validation script and route all cycles through it to prevent target drift.
3. Add degraded-mode dispatch policy: when LLM provider fails, auto-fallback to deterministic audit executor (git+test+lint+state write).
4. Classify and resolve `web/lib/generated/skillsCatalog.json` drift (commit/regenerate/revert) in isolated change lane.

