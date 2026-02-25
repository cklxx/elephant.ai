# Restore Web Architect Tool Access

**Goal:** Re-enable full web-safe tool access for the web architect preset while keeping local-only tools blocked.

## Plan
1) Update web architect preset to allow all tools except the web denied list.
2) Keep CLI architect restrictions unchanged.
3) Run lint/tests and record progress.

## Progress Log
- 2026-01-22: Updated web architect preset to allow all web-safe tools.
- 2026-01-22: Ran ./dev.sh lint and ./dev.sh test (happy-dom AbortError logs still emitted).
