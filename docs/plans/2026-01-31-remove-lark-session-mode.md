# Remove Lark session_mode config and fix behavior

**Date**: 2026-01-31
**Status**: Completed
**Author**: cklxx

## Scope
- Remove Lark `session_mode` configuration wiring.
- Keep session history injection disabled for all Lark messages.
- Update tests and docs to reflect fixed session behavior.

## Plan
1. Remove `session_mode` from config structs/loaders/logging and gateway wiring.
2. Normalize Lark gateway to use stable chat-derived session IDs only.
3. Update tests to match new behavior.
4. Update CONFIG + event-flow docs.
5. Run full lint + tests.

## Progress Log
- 2026-01-31: Plan created.
- 2026-01-31: Removed session_mode config, fixed Lark gateway/tests/docs.
- 2026-01-31: Ran `./dev.sh lint` + `./dev.sh test` (failed due to unrelated cmd/alex typecheck + orchestration/react/context failures; LC_DYSYMTAB linker warnings observed).
