# Plan: Eval Path Automation For Real E2E

## Goal
- Make `alex eval` runnable from non-repo working directories without manual path env setup.
- Eliminate manual steps for:
  - `ALEX_CONTEXT_CONFIG_DIR`
  - loading repo `.env` for runtime API key expansion
  - resolving repo-relative dataset paths when launched outside repo root

## Scope
- `cmd/alex/eval.go`
- New unit tests for eval path/runtime preparation logic

## Implementation Steps
1. Add failing tests for eval runtime path preparation.
2. Implement project-root detection and runtime env preparation in `alex eval`.
3. Re-run targeted tests and full regression (`make dev-test`, `make dev-lint`).
4. Re-run real-subscription E2E eval to verify end-to-end behavior without manual path wiring.
5. Run mandatory code review workflow and record report.

## Progress Log
- [x] Created worktree/branch and copied `.env`.
- [x] Reviewed `docs/guides/engineering-practices.md`.
- [x] Add tests.
- [x] Implement code.
- [x] Validate lint/tests.
- [x] Real E2E rerun.
- [x] Code review.
- [ ] Commit + merge + cleanup.

## Validation Snapshot
- `make dev-lint`: pass.
- `OPENAI_API_KEY=sk-test make dev-test`: pass.
- Real subscription E2E from `/tmp` without manual path env wiring:
  - command: `alex eval --output /tmp/agent-e2e-real-subscription-auto-path-20260213-223740 --limit 3 --workers 1 --timeout 120s -v`
  - result: path/runtime bootstrap succeeded (no API/context/dataset path errors), 3/3 reached execution and timed out at 120s each.
