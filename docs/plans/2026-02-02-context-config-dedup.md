# Plan: Context config dedup + test updates (2026-02-02)

## Goals
- Remove remaining redundant context guidance while keeping core intent intact.
- Align context models/types/tests with the trimmed config.
- Verify lint + tests are green.

## Plan
1. Inspect context config for redundant lines; trim and keep unique guidance.
2. Update context models/types/tests to match the new config surface.
3. Run full lint + tests and capture results.

## Progress
- [x] Inspect and trim redundant context guidance.
- [x] Update models/types/tests for the trimmed config.
- [x] Lint and tests green.

## Follow-up
1. Remove redundant workflow policy config.
2. Update context loader docs to reflect policy/knowledge trims.
3. Re-run lint + tests.

## Follow-up Progress
- [x] Remove redundant workflow policy config.
- [x] Update context loader docs to reflect policy/knowledge trims.
- [x] Lint and tests green.
