# Plan: Kernel Empty Response Summary Validation

**Status:** in_progress
**Branch:** main
**Created:** 2026-02-26

## Problem

Kernel cycle summary can show diagnostic garbage like:
`Empty response: {'content': , 'stop_reason': 'end_turn', ...}`.

That output is currently treated as a successful actionable dispatch if a real tool
ran, which is misleading and hides invalid final response quality.

## Solution

1. Add kernel dispatch validation for execution summary quality.
2. Treat empty/diagnostic-only summaries as invalid and trigger one retry.
3. Keep final result as failed if retry is still invalid.
4. Add targeted tests for invalid-summary recovery and failure behavior.

## Tasks

- [ ] Tighten validation in `internal/app/agent/kernel/executor.go`
- [ ] Add tests in `internal/app/agent/kernel/coordinator_executor_test.go`
- [ ] Run formatting + kernel tests
- [ ] Run mandatory code review before commit
- [ ] Commit and push
