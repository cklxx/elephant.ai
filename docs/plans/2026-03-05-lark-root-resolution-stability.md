# 2026-03-05 Lark Root Resolution Stability

## Context
- Incident: `afx-20260305T083011Z-main-4746df96`
- Symptom:
  - upgrade storms from repeated `main`/`kernel` restart failures.
  - autofix and loop flows fail with:
    - `Not a git repository (cannot resolve main worktree)`
    - `fatal: this operation must be run in a work tree`
- Likely cause:
  - multiple scripts resolve repo root from caller CWD or unvalidated `git worktree list` entries.
  - stale/non-worktree paths can be selected as `MAIN_ROOT`.

## Goal
Restore stable supervisor/main/test/loop behavior with minimal, maintainable changes.

## Plan
1. Add shared resolver in `scripts/lib/common/git_worktree.sh`:
   - anchor git calls to script directory,
   - validate candidate worktree paths before accepting,
   - fallback to anchored top-level repo.
2. Migrate lark scripts to shared resolver:
   - `scripts/lark/{main.sh,kernel.sh,test.sh,loop.sh,loop-agent.sh,cleanup_orphan_agents.sh,supervisor.sh,autofix.sh}`.
3. Add/extend script-level tests for resolver behavior.
4. Validate:
   - `bash -n` on lark/common scripts,
   - `./tests/scripts/lark-supervisor-smoke.sh`.

## Progress
- [x] Root cause confirmed in script root-resolution flow.
- [x] Shared resolver implemented.
- [x] Script migrations completed.
- [x] Added `tests/scripts/lark-main-root-resolver.sh` and validated stale-worktree fallback behavior.
- [ ] `tests/scripts/lark-supervisor-smoke.sh` is currently out of sync with active supervisor schema (`test_*` fields); tracked as existing baseline mismatch.
