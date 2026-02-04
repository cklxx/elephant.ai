# Plan: Drop UI `send_message`, keep `lark_send_message` only

**Goal:** Remove the duplicated `send_message` tool implementation under `internal/tools/builtin/ui/` and keep only the Lark-specific `lark_send_message` tool.

## Rationale
- `ui/` is meant for channel-agnostic orchestration/interaction tools, but `send_message` is currently Lark-only and duplicates `lark_send_message`.
- Reduce surface area and avoid confusing tool choices for the model.

## Scope
- Remove `send_message` tool registration and implementation/tests.
- Update docs/policy to reference `lark_send_message` only.
- Keep `lark_send_message` semantics unchanged (reply target derived from Lark context; no `reply_to_message_id` parameter).

## Checklist
- [ ] Remove tool + registry wiring
- [ ] Update docs + default policy
- [ ] Run full lint + tests
- [ ] Commit in small steps and merge to `main`

## Progress Log
- 2026-02-04: Plan created.

