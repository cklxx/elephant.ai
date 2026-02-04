# Plan: Remove `reply_to_message_id` from send_message tools

**Goal:** Remove the explicit `reply_to_message_id` parameter from `send_message` and `lark_send_message` tool schemas. Reply threading (when applicable) is derived from Lark message context instead of user-provided parameters.

## Scope
- Update tool schemas to only accept `content`.
- Enforce unsupported-parameter errors for `reply_to_message_id`.
- Keep reply behavior by using `message_id` from tool execution context (if present); otherwise send a normal message.
- Exclude `send_message` from tool result cache.

## Checklist
- [ ] Update `send_message` tool implementation + tests
- [ ] Update `lark_send_message` tool implementation + tests
- [ ] Update tool cache exclusion list
- [ ] Run full lint + tests
- [ ] Commit in small steps and merge back to `main`

## Progress Log
- 2026-02-04: Plan created.

