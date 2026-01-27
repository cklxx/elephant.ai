# Engineering Practices

These are the local engineering practices for this repo. Keep them short and actionable.

## Core
- Prefer correctness and maintainability over short-term speed.
- Make small, reviewable changes; avoid large rewrites unless explicitly needed.
- Use TDD when touching logic; include edge cases.
- Run full lint and tests before delivery.
- Keep config examples in YAML only (no JSON configs).

## Planning & Records
- Every non-trivial task must have a plan file under `docs/plans/` and be updated as work progresses.
- Log notable incidents in `docs/error-experience/entries/` and add a summary entry under `docs/error-experience/summary/entries/`.
- Log notable wins in `docs/good-experience/entries/` and add a summary entry under `docs/good-experience/summary/entries/`.
- Keep `docs/error-experience.md` and `docs/error-experience/summary.md` as index-only.
- Keep `docs/good-experience.md` and `docs/good-experience/summary.md` as index-only.

## Safety
- Avoid destructive operations or history rewrites unless explicitly requested.
- Prefer reversible steps and explain risks when needed.

## Code Style
- Avoid unnecessary defensive code; trust invariants when guaranteed.
- Keep naming consistent; follow local naming guidelines when present.
