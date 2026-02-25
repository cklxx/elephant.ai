# LLM Indicator Menu UI Fixes

**Goal:** Ensure the LLM indicator dropdown has an opaque background and clearly renders available model options (including empty/error states).

## Plan
1) Adjust the dropdown content styling to enforce an opaque background and clearer separation.
2) Refactor model list rendering to remove empty placeholders, add clear status messaging, and show per-provider empty/error states.
3) Validate UI behavior with lint/tests.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Updated LLM indicator dropdown styling/status rendering and ensured catalog always includes Ollama target by default.
- 2026-01-22: Ran ./dev.sh lint and ./dev.sh test (happy-dom AbortError logs still emitted).
