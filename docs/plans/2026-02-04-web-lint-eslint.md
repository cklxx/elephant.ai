# Plan: Fix web lint eslint availability (2026-02-04)

## Goal
- Ensure `./dev.sh lint` can run web lint when `web/node_modules` is missing.
- Keep changes minimal and local.

## Constraints
- Bash scripts only; no JSON config examples.
- Run full lint + tests before delivery.

## Plan
1. Confirm current lint flow and failure mode.
2. Add a dependency preflight for web lint to ensure eslint is available.
3. Run `./dev.sh lint` and `./dev.sh test` to validate.

## Progress
- 2026-02-04: Plan created.
- 2026-02-04: Added web lint dependency preflight to install eslint when missing.
- 2026-02-04: Ran `./dev.sh lint` and `./dev.sh test`.
