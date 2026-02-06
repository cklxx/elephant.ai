# Lark Interactive Cards

This document describes the Lark/Feishu interactive card integration used by elephant.ai.

## What we send

- **Plan review card**: triggered when the agent stops with `await_user_input` and plan review cards are enabled. Includes goal, plan JSON, and action buttons.
- **Result card**: sent on success when result cards are enabled (summary + attachments).
- **Error card**: sent on failure when error cards are enabled (default on).
- **Model selection card**: sent by `/model` / `/model list` when cards are enabled; users can click a model button directly.

## Callback endpoint

Cards are interactive; button clicks invoke a callback endpoint. Configure your Lark app to call:

- `POST /api/lark/card/callback`

### Required configuration

```yaml
channels:
  lark:
    cards_enabled: true
    cards_plan_review: false
    cards_results: false
    cards_errors: true
    card_callback_verification_token: "${LARK_VERIFICATION_TOKEN}"
    card_callback_encrypt_key: "${LARK_ENCRYPT_KEY}"
```

- `card_callback_verification_token` is the verification token from the Lark app settings.
- `card_callback_encrypt_key` is optional when callback encryption is disabled.
- `channels.lark` supports `${ENV}` interpolation. If these fields are omitted in YAML, server fallback env keys are also supported:
  - verification token: `LARK_CARD_CALLBACK_VERIFICATION_TOKEN`, `LARK_VERIFICATION_TOKEN`, `FEISHU_CARD_CALLBACK_VERIFICATION_TOKEN`, `FEISHU_VERIFICATION_TOKEN`, `CARD_CALLBACK_VERIFICATION_TOKEN`
  - encrypt key: `LARK_CARD_CALLBACK_ENCRYPT_KEY`, `LARK_ENCRYPT_KEY`, `FEISHU_CARD_CALLBACK_ENCRYPT_KEY`, `FEISHU_ENCRYPT_KEY`, `CARD_CALLBACK_ENCRYPT_KEY`

## Action tags and behavior

- `plan_review_approve` → injects `OK` as user input.
- `plan_review_request_changes` → injects the `plan_feedback` form value when provided, otherwise injects `需要修改`.
- `confirm_yes` → injects `OK`.
- `confirm_no` → injects `取消`.
- `model_use` → injects `/model use <provider>/<model>` from `action.value.text`.

## Card input fields

Plan review cards include an optional input element:

- `name: plan_feedback`

When the user submits via the **提交修改** button, the input value is passed in `action.form_value.plan_feedback`.

## Notes

- Card callbacks are handled asynchronously; the endpoint returns immediately with a toast, and the action is injected into the Lark gateway as a normal user message.
- When verification token is missing, callback route still stays active for action events, but URL verification challenge may fail until token is configured.
- If callbacks are not configured, cards still render but button clicks will not trigger actions; users can respond manually via text.
