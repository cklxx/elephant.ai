# Lark Cards

Updated: 2026-03-10

This repo uses a small set of Lark cards.

## Used Today

- Auth cards for in-chat OAuth authorization.
- Leader notification cards for blocker alerts, weekly pulse, daily summary, and milestone updates.

## Plan Review

- `await_user_input` plan review is resumed through plain-text replies.
- Pending review state is stored locally and restored from session state when needed.
- Old action-tag lists are not part of the current contract.

## Callback Setup

If interactive card callbacks are enabled, configure:

- `POST /api/lark/card/callback`

Message/event callbacks still use:

- `POST /api/lark/callback`

## Required Config

- `channels.lark.enabled`
- `channels.lark.app_id`
- `channels.lark.app_secret`
- `channels.lark.persistence.*`

## Notes

- Auth cards open a verification URL.
- Leader cards are notification-oriented; they are not the control surface for normal task continuation.
- Plain-text replies remain the fallback when card interaction is unavailable.
