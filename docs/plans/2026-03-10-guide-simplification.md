# 2026-03-10 Guide Simplification

## Goal

Rewrite these guides for faster scanning and lower ambiguity:

- `docs/guides/engineering-workflow.md`
- `docs/guides/code-review-guide.md`
- `docs/guides/memory-management.md`

## Scope

- Remove verbose explanation.
- Keep mandatory rules and execution order.
- Keep cross-references only where they still help.

## Checklist

- [x] Create worktree and marker.
- [x] Read triggered guidance and target docs.
- [x] Rewrite the three guides.
- [x] Run relevant validation.
- [x] Run code review.
- [ ] Commit and merge to `main`.

## Notes

- This is a documentation-only change. Validation should stay proportionate.
- `alex dev lint` failed because `eslint` is not installed in this environment.
- `alex dev test` failed because the Go toolchain is mixed (`go1.26.1` compiled packages vs `go1.26.0` tool).
- `git diff --check` passed.
- `python3 skills/code-review/run.py review` ran successfully but returned raw diff payload instead of a structured findings report.
