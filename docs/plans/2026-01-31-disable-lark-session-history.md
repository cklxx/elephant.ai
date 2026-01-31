# Disable Lark session history injection

**Date**: 2026-01-31
**Status**: In Progress
**Author**: cklxx

## Scope
- Disable session history injection for all Lark messages regardless of session_mode.
- Update tests and docs to reflect chat-only context expectations.

## Plan
1. Force Lark gateway to disable session history injection.
2. Update Lark gateway tests to assert history is always disabled.
3. Document the behavior in CONFIG reference.
4. Run full lint + tests.

## Progress Log
- 2026-01-31: Plan created.
