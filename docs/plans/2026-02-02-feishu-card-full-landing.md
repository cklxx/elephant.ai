# Plan: Feishu card full landing (2026-02-02)

## Goals
- Land Feishu/Lark interactive cards end-to-end (templates + send + callback handling).
- Wire cards into Lark channel flows where appropriate (plan approval, task result, error).
- Add tests for card rendering + callback parsing/handling.
- Update docs/config to reflect the new card support.

## Plan
1. Audit current Lark gateway/messenger + card builder to define integration points.
2. Implement card templates + mapping (plan, result, error) and wire into Lark replies.
3. Add callback handler for card actions (button/select) and route into agent input.
4. Add/update tests (TDD) covering card JSON, send path, and callback flow.
5. Run lint/tests and restart dev stack.

## Progress
- [x] Audit Lark gateway/messenger and card builder.
- [x] Implement templates + send path.
- [x] Implement callback handling.
- [x] Tests updated/added.
- [x] Lint/tests + restart.
