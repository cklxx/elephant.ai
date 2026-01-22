# Tool Base + Web Test Noise Cleanup

**Goal:** Align local tool execution to a consistent working-directory base, update docs, and remove noisy web test network errors.

## Plan
1) Extend local search tools (grep/ripgrep) and shell execution to respect the working-directory base.
2) Document tool path constraints and recommended workflow.
3) Stabilize web tests by stubbing default network calls.

## Progress Log
- 2026-01-22: Created plan; starting implementation.
- 2026-01-22: Updated local search tools and bash execution to respect working directory; documented tool workspace; stubbed default web fetch for tests.
- 2026-01-22: Ran ./dev.sh lint and ./dev.sh test.
