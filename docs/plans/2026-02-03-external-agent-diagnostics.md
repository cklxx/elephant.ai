# Plan: External Agent Diagnostics (Codex/Claude)

## Status: Completed
## Date: 2026-02-03

## Problem
Background external-agent failures (Codex/Claude) currently surface as generic errors like "signal: killed" or "streaming failed" without actionable stderr or exit details. Local CLI runs succeed, so we need richer error context from background subprocesses.

## Plan
1. Add stderr tail capture to subprocess runner (Claude) and MCP process manager (Codex).
2. Surface stderr tail + exit/signal details in executor errors with consistent formatting.
3. Add TDD coverage for stderr tail capture and error formatting (Claude/Codex).
4. Run full lint + tests per engineering practices, then update records if needed.

## Progress
- [x] Capture stderr tails for subprocess + MCP process manager.
- [x] Add executor error formatting with stderr tail and exit/signal details.
- [x] Add/adjust tests covering stderr tail capture and error formatting.
- [x] Run full lint + tests.
