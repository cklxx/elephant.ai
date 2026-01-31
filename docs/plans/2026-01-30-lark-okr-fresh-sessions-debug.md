# Lark OKR tools + fresh sessions + debug visibility

## Goal
Fix Lark OKR tool availability, default to fresh sessions, and make the conversation debug page surface new Lark sessions.

## Plan
- [x] Add Lark gateway tests for default session mode behavior and tool preset defaults.
- [x] Implement Lark defaults (fresh session mode, tool preset fallback) and update config docs.
- [x] Refresh dev conversation debug Lark session list (auto/manual refresh).
- [x] Run lint + tests. (lint fails: internal/server/bootstrap/subsystem_test.go unused isStarted)
- [x] Update memory timestamp.
