# Plan: Fix garbled subagent UI title (2026-01-27)

## Goal
- Prevent subagent card titles from showing garbled characters by ensuring preview truncation is UTF-8 safe.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Locate where subagent preview text is derived and truncated.
2. Implement rune-safe truncation for subtask preview generation.
3. Add unit test for UTF-8 safety and truncation behavior.
4. Run full lint + tests.
5. Commit changes.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Implemented rune-safe subtask preview truncation and added UTF-8 safety test.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (tests failed: invalid OpenAI API key in `internal/rag` integration test); logged error experience.
