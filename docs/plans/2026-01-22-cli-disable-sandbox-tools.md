# CLI Sandbox Tools Disable Plan

**Goal:** Ensure CLI tool presets never expose sandbox_* tools so CLI-in-Docker uses local tools only.

## Plan
1) Update CLI tool preset filtering to deny all sandbox_* tools and adjust preset/mode descriptions if needed.
2) Update preset tests to reflect sandbox_* tools blocked in CLI.
3) Run full lint + test suite and record results.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Blocked sandbox_* tools in CLI presets and updated sandbox mode description.
- 2026-01-22: Updated preset tests for CLI sandbox tool blocking.
- 2026-01-22: Updated ACP reference mode description to match CLI sandbox tool behavior.
- 2026-01-22: Added error-experience entry + summary for CLI sandbox tool connection refusal.
- 2026-01-22: Tests run: `./dev.sh lint`, `./dev.sh test` (web tests pass with happy-dom AbortError logs during teardown).
