Summary: Running full pre-push checks on a dirty `main` workspace produced unrelated failures (interface-drift test stubs + env-usage guard), obscuring refactor validation.
Remediation: Flag dirty diffs early, prefer worktree isolation for broad cleanup, and explicitly separate scoped-pass from full-gate blockers.

## Metadata
- id: errsum-2026-02-26-main-dirty-workspace-quality-noise
- tags: [summary, validation, workspace, process]
- derived_from:
  - docs/error-experience/entries/2026-02-26-main-branch-dirty-workspace-quality-gate-noise.md
