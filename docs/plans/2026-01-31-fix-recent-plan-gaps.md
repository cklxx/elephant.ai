# Fix recent plan gaps (attachments + Lark history + presigned refresh)

**Date**: 2026-01-31
**Status**: In Progress
**Author**: cklxx

## Scope
- Ensure attachment persistence behavior matches degraded storage expectations.
- Re-enable Lark session history injection when stable sessions are enabled.
- Add presigned URL refresh fallback on the web frontend.

## Plan
1. Add/update tests for Lark session history toggling by session_mode.
2. Align attachment persister wiring with degradation guarantees (avoid async loss) and add coverage if needed.
3. Implement presigned URL refresh fallback in web attachment URI helpers with tests.
4. Run full lint + tests.

## Progress Log
- 2026-01-31: Plan created.
- 2026-01-31: Updated Lark session history toggle, switched attachment persister to sync, added presigned refresh logic + tests.
