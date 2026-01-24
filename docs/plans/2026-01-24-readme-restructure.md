# Plan: README restructure (2026-01-24)

## Goal
- Reorder README content into a clear, progressive flow: what it is → why it matters → how it works → how to run it → where to go next.

## Plan
1. Review current README sections and confirm key messages that must remain.
2. Draft a progressive outline and reorganize content without changing technical meaning.
3. Update README content/section order and tighten transitions.
4. Run full lint and test suite.

## Progress
- 2026-01-24: Reviewed engineering practices and captured the current README structure.
- 2026-01-24: Reordered README sections into overview → architecture → getting started → demo → references.
- 2026-01-24: Simplified README logo markup to keep only the outer ring around the mascot.
- 2026-01-24: Ran lint + Go tests; Playwright e2e failed across browsers due to missing console UI elements and / not redirecting to /conversation.
