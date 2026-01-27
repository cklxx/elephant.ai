# Plan: Add research report guidance + humanized prompt examples (2026-01-26)

## Goal
- Extend static context with a research report template reference and make the intent-understanding example prompt more natural.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add a knowledge reference for the research report template in `configs/context/knowledge`.
2. Create the research report template doc under `docs/guides/`.
3. Update the user intent understanding example to use a human-style prompt.
4. Run full lint + tests.
5. Commit changes.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Added research report knowledge reference + template doc; updated intent example to a human-style prompt.
- 2026-01-26: Ran `./dev.sh lint` and `./dev.sh test` (both pass).
