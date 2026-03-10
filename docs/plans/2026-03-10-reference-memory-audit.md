# 2026-03-10 Reference And Memory Doc Audit

## Goal

Audit and simplify these reference docs:

- `docs/reference/LARK_CARDS.md`
- `docs/reference/LARK_MENTIONS.md`
- `docs/reference/lark-voice-message.md`
- `docs/reference/SOUL.md`
- `docs/reference/reuse-catalog-and-folder-governance.md`
- `docs/reference/reuse-path-index.md`

Also inspect `docs/memory/` files and simplify any that are still verbose or outdated.

## Checklist

- [x] Create worktree and marker.
- [x] Read target reference and memory docs.
- [x] Rewrite only the docs that need simplification.
- [x] Run relevant validation.
- [x] Run code review.
- [ ] Commit, merge, and push.

## Notes

- Simplified all six requested reference docs.
- Simplified the memory docs that still carried historical detail: `README.md`, `long-term.md`, `eval-routing.md`, `lark-devops.md`, `runtime-events.md`, and `networked/README.md`.
- Left `user-patterns.md` and `networked/entry-template.md` unchanged because they were already concise.
- `git diff --check` passed.
- `python3 -m pytest skills/soul-self-evolution/tests -q` passed (`6 passed`).
- `python3 skills/code-review/run.py review` ran successfully but returned raw diff payload instead of a structured findings report.
