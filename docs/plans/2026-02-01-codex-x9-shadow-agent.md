# Plan: X9 Shadow Agent Framework (MVP)

Created: 2026-02-01
Updated: 2026-02-01 23:15
Status: done

## Goal
Create a minimal Shadow Agent framework under `internal/devops/shadow/` with mandatory approval gate and integration to Coding Agent Gateway for execution.

## Plan
- [completed] Survey roadmap specs for Shadow Agent and approval gate requirements.
- [completed] Implement core types/config + Agent lifecycle entrypoint.
- [completed] Implement mandatory approval gate (cannot bypass if approver missing).
- [completed] Implement dispatcher using Coding Gateway (Submit/Stream).
- [completed] Add basic tests for approval gating and dispatcher wiring.
- [completed] Run lint/tests, restart dev services, and commit in incremental steps.
