# Lark Mentions

Updated: 2026-03-10

This doc covers how the repo reads and writes `@` mentions in Lark.

## Incoming Mentions

- Real mentions arrive either as placeholder keys such as `@_user_n` plus `event.message.mentions[]`, or as `<at ...>` tags inside message payloads.
- The gateway converts them into readable text like `@Alice(ou_xxx)` before passing them to the model.
- Hand-typed `@_user_n` or `@cli_xxx` is plain text, not a real mention.

## Outgoing Mentions

- Write mentions in readable form: `@Alice(ou_xxx)`.
- Use `@所有人(all)` for `@all`.
- The sender converts that format into real Lark `<at user_id="...">...</at>` tags.

## Group And Multi-Bot Use

1. In the Lark client, do a real `@` once through the picker.
2. The incoming event gives the repo the target `open_id`.
3. Later replies can reuse `@Name(ou_xxx)` and the gateway will render a real mention.

If no real mention has ever arrived, the repo cannot infer the target `ou_...` by itself.

## Rule Of Thumb

- Incoming: Lark mention payload -> readable `@Name(ou_xxx)`
- Outgoing: readable `@Name(ou_xxx)` -> real Lark mention
