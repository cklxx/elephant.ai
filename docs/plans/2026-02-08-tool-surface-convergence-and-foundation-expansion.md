# Plan: Tool Surface Convergence + Foundation Suite Expansion (2026-02-08)

## Status
- done

## Goal
- Converge conflicting tool surfaces into one canonical set so model discoverability is cleaner and sandbox remains an execution backend detail.
- Expand foundation evaluation collections across:
  - basic tool coverage
  - prompt effectiveness coverage
  - proactivity coverage
  - complex high-value task coverage
- Improve reports with explicit `x/x` counters for collections and cases.

## Scope
- `internal/app/toolregistry/*`
- `internal/shared/agent/presets/*`
- `internal/app/agent/preparation/*` (prompt wording alignment)
- `internal/delivery/output/*`, `internal/delivery/presentation/formatter/*` (canonical naming compatibility)
- `evaluation/agent_eval/*` and `evaluation/agent_eval/datasets/*`
- `cmd/alex/eval_foundation*.go`

## Steps
- [x] Baseline current suite and identify bad cases / ranking conflicts.
- [x] Converge registry tool surface to canonical names and add unified compatibility adapter for legacy names.
- [x] Update prompt/tool wording and formatters to favor canonical names.
- [x] Expand layered foundation datasets with larger case counts.
- [x] Add report `x/x` counters (collections, cases, per-collection cases) in suite outputs.
- [x] Add/update tests for registry compatibility, report fields, and dataset loading.
- [x] Run targeted tests, full lint/test, and foundation-suite rerun for validation.
- [x] Commit incrementally, merge back to `main`, cleanup worktree.

## Progress Log
- 2026-02-08 14:45: Reviewed engineering practices + memory summaries and confirmed isolated worktree branch from `main`.
- 2026-02-08 14:50: Ran baseline foundation suite (`tmp/foundation-suite-baseline-20260208`): top-k pass was stable, but many top-1 bad cases were caused by conflicting legacy/canonical tool names (e.g. `file_edit` outranking `read_file`/`write_file`, `code_execute` outranking `execute_code`).
- 2026-02-08 15:00: Implemented tool surface convergence in registry: removed legacy conflicting tools from static registration and added legacy compatibility alias wrapper (`Get`-time routing to canonical tools with argument/path normalization).
- 2026-02-08 15:03: Expanded layered foundation datasets to larger scale (tool coverage 46, prompt effectiveness 32, proactivity 30, complex tasks 30; total suite cases 138), with canonical tool expectations only.
- 2026-02-08 15:04: Added `x/x` report counters for suite aggregate and per-collection, and updated CLI eval summary output.
- 2026-02-08 15:05: Suite rerun on expanded set reached `4/4` collections and `138/138` cases (Top-K), availability errors `0`.
- 2026-02-08 15:07: Full `./dev.sh lint` and `./dev.sh test` executed; blocked by pre-existing unrelated failures in `internal/devops/*`, `cmd/alex/dev*.go`, and known race/env-guard failures in `internal/delivery/server/bootstrap` + `internal/shared/config`.
- 2026-02-08 15:09: Added good-experience entry capturing canonical tool-surface convergence + scaled layered suite optimization pattern.
- 2026-02-08 15:11: Landed 3 incremental commits, fast-forward merged into `main`, and removed temporary worktree.
