# Plan: Lark Slow Progress Summary LLM Style

**Status:** completed
**Branch:** main
**Created:** 2026-02-26

## Problem

Lark 30s slow-progress reply can expose internal workflow IDs (for example `react:iter:*` and `call_*`) and engineering-style sections, which does not match the expected LLM-style progress summary for end users.

## Solution

1. Keep LLM summary as the primary output text when available.
2. Sanitize/transform internal workflow node identifiers before they enter summary signals.
3. Filter internal diagnostic-like summary content from signal sources.
4. Add regression tests for LLM output path and internal-ID filtering.

## Tasks

- [x] Update `slow_progress_summary_listener.go` summary rendering and sanitization logic.
- [x] Add tests in `slow_progress_summary_listener_test.go`.
- [x] Run focused Lark tests for slow progress summary behavior.
- [x] Run full quality gate + mandatory code review.
- [x] Commit and push.
