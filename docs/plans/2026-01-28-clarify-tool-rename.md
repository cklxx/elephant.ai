# Plan: Rename clearify tool to clarify

Date: 2026-01-28
Owner: Codex

## Goal
Rename the UI orchestration tool from `clearify` to the correct English `clarify` across backend, frontend, tests, and docs.

## Plan
1. Replace tool name strings, identifiers, and references across Go/TS code, tests, and relevant docs.
2. Rename tool implementation and UI components/files to match `clarify` naming.
3. Remove legacy misspelling hooks (e.g., `claify`) to keep naming consistent.
4. Run full lint + test suite and fix any fallout.

## Progress
- 2026-01-28: Plan created.
- 2026-01-28: Replaced clearify naming with clarify across backend/frontend/tests/docs and renamed UI tool + timeline component files.
- 2026-01-28: Removed legacy `claify` alias in CLI renderer helpers.
- 2026-01-28: Ran `./dev.sh lint` and `./dev.sh test` (Go tests passed; linker warnings observed).
