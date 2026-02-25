# Plan: Remove `reply_to_message_id` from Lark send-message tool

**Goal:** Remove the explicit `reply_to_message_id` parameter from `lark_send_message` tool schema. Reply threading (when applicable) is derived from Lark message context instead of user-provided parameters.

## Scope
- Update tool schema to only accept `content`.
- Enforce unsupported-parameter errors for `reply_to_message_id`.
- Keep reply behavior by using `message_id` from tool execution context (if present); otherwise send a normal message.

## Checklist
- [x] Update `lark_send_message` tool implementation + tests
- [x] Run full lint + tests
- [x] Commit in small steps and merge back to `main`

## Progress Log
- 2026-02-04: Plan created.
- 2026-02-04: Removed `reply_to_message_id` param; derive reply target from context; added tests; updated cache exclude; ran lint+tests.
- 2026-02-04: Dropped duplicated UI `send_message`; keep `lark_send_message` only.
