# Plan: X8 Coding Agent Gateway (MVP)

Created: 2026-02-01
Updated: 2026-02-01 23:15
Status: done

## Goal
Introduce a minimal Coding Agent Gateway under `internal/coding/` with a unified interface (Submit/Stream/Cancel/Status), adapter registry, and initial adapters wrapping existing external executors (Codex + Claude Code).

## Plan
- [completed] Survey existing external executors + config wiring for Codex/Claude Code.
- [completed] Implement gateway interface + types + adapter registry.
- [completed] Add adapters for Codex and Claude Code (wrapper around existing external executors).
- [completed] Add CLI detection helper + task translator + workspace/verify_build stubs.
- [completed] Add unit tests for routing/adapter selection and adapter behavior.
- [completed] Run lint/tests, restart dev services, and commit in incremental steps.
