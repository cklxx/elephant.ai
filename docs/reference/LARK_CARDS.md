# Lark Interactive Cards

This document describes interactive card behavior used by elephant.ai in Lark.

## What we send

- **Plan review card**: triggered when the agent stops with `await_user_input`.
- **Result card**: sent on successful task completion when card rendering succeeds.
- **Error card**: sent on failures.
- **Model selection card**: sent by `/model` / `/model list` when supported by the current flow.

## Callback endpoint

Cards are interactive; button clicks invoke:

- `POST /api/lark/card/callback`

## Configuration note

Card callback and card-toggle fields were removed from `channels.lark` runtime config.
Current Lark setup only requires:

- `channels.lark.enabled`
- `channels.lark.app_id`
- `channels.lark.app_secret`
- `channels.lark.persistence.*` (for task/plan/session local persistence)

## Action tags and behavior

- `plan_review_approve` → injects `OK` as user input.
- `plan_review_request_changes` → injects `plan_feedback` when provided, else `需要修改`.
- `confirm_yes` → injects `OK`.
- `confirm_no` → injects `取消`.
- `model_use` → injects `/model use <provider>/<model>` from `action.value.text`.

## Notes

- Card callbacks are handled asynchronously.
- If callback verification is not configured on the Lark platform side, card actions may not be delivered.
- If callback delivery is unavailable, users can always continue via plain text replies.
