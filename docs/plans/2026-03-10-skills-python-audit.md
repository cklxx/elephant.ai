# 2026-03-10 Skills Python Audit

## Goal

Audit `skills/` Python code for:

- stale dependencies
- unused imports
- dead code
- removable obsolete skill directories

## Checklist

- [x] Create worktree and marker.
- [x] Inspect skill Python entrypoints, tests, and references.
- [x] Remove verified unused imports, dead code, and redundant bootstrap code.
- [x] Run targeted validation and review.
- [ ] Commit and push.

## Notes

- Current repository references still point at all existing skill directories under `skills/`; no directory is yet proven obsolete enough to delete safely.
- First lint pass found unused imports concentrated in tests plus a few generated `run.py` files.
- Several `run.py` files duplicate `sys` / `Path` imports after skill-runner bootstrap; these are safe cleanup targets.
- Cleaned redundant imports across skill runtimes plus unused mock/test parameters that were creating lint noise.
- Removed an unused helper parameter in `skills/soul-self-evolution/run.py`.
- Validation passed:
  - `python3 -m ruff check skills --select F401,F811,F821,F841,ARG001,ARG002,PLC0414,UP035`
  - `python3 -m pytest skills -q`
  - `python3 skills/code-review/run.py review`
  - `scripts/pre-push.sh`
- `scripts/pre-push.sh` initially failed because `web/node_modules` was absent and `eslint` was unavailable. Resolved by running `npm ci` in `web/`, then reran successfully.
