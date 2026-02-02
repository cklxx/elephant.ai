# Lark await_user_input Resume + Reply Plan

## Goal
Ensure Lark handles `question_to_user` (clarify/request_user) correctly:
- Reply with the question instead of “（无可用回复）”.
- Reuse the existing session after user answers, so the run continues with prior context.

## Scope
- Lark gateway behavior for `await_user_input` (question_to_user/request_user).
- Session metadata for pending user input (persisted across restarts).
- Tests for Lark gateway reply + resume input seeding.

## Design
### 1) Detect pending user input in task results
- Extract question content from tool results where `metadata.needs_user_input == true`.
- Prefer `question_to_user` (clarify), fallback to `message` (request_user), else tool result content.

### 2) Reply with the pending question
- When `result.StopReason == await_user_input` and no plan-review pending, reply with the extracted question.
- This avoids “（无可用回复）” for user-input pauses.

### 3) Persist pending state on session
- Store `session.Metadata["await_user_input"] = "true"` and
  `session.Metadata["await_user_input_question"] = <question>` when stop reason is await_user_input.
- Clear these fields when the stop reason is anything else.

### 4) Reuse session on follow-up
- When a new Lark message arrives and session metadata indicates `await_user_input`,
  seed the user-input channel with the new message and set task content to empty string.
  This supports both checkpoint resume and non-resume flows without duplicating the reply.

## Plan
1) Add helper to extract pending user-input question from TaskResult tool results.
2) Update Lark gateway to reply with question on await_user_input (non-plan-review).
3) Persist/clear `await_user_input` metadata in coordinator session save.
4) Update Lark gateway to seed input channel when pending flag exists.
5) Add tests: Lark reply on await_user_input, seeding input channel when pending.
6) Run lint + tests.

## Progress
- 2026-02-02: Plan created.
