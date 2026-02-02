# Plan: Lark card roadmap + Lark reply investigation (2026-02-02)

## Goals
- Add Feishu/Lark card-related roadmap items (feature + design requirements).
- Identify and fix why Lark messages are not being replied to.

## Plan
1. Inspect current Lark gateway handling and logs to locate drop points; propose fix.
2. Add/adjust tests to capture the regression (TDD) and implement the fix.
3. Update ROADMAP with Feishu card feature + design items.
4. Run full lint + tests, then restart `./dev.sh down && ./dev.sh`.

## Progress
- [x] Inspect Lark gateway/logs and identify drop cause.
- [x] Add tests + implement fix for Lark replies.
- [x] Update ROADMAP with Lark card feature/design items.
- [x] Run lint/tests + restart dev stack.

## Notes
- `make test` timed out in `internal/notification` (`TestCriticalNotificationsGoToAllChannels`).
