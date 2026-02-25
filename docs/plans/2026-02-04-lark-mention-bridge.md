# 2026-02-04 — Lark: mention bridge (incoming `mentions` + outgoing `<at>`)

## Goal

Make Lark @mentions reliable for both incoming and outgoing text:

- Incoming: resolve `@_user_n` placeholders using `event.message.mentions[]` into readable `@Name(ou_...)`.
- Outgoing: render readable `@Name(ou_...)` back into real Lark mention tags `<at user_id="ou_...">Name</at>`.
- Fix `post` message mentions where `tag=at` uses placeholder keys.
- Document mention formats and the `cli_...` vs `ou_...` identifier mismatch.

## Scope

In scope:

- Lark gateway message extraction and outbound payload rendering.
- `lark_send_message` tool payload rendering (text).
- Tests + docs updates.

Out of scope:

- Any `cli_... (app_id)` → `ou_... (open_id)` reverse lookup.
- New tools or channel-agnostic mention helpers.

## Checklist / Progress

- [x] Add plan + worktree setup for mention bridge task.
- [x] Resolve incoming `@_user_n` placeholders via `event.message.mentions[]` (text + post) with tests.
- [x] Render outgoing `@Name(ou_...)` into `<at user_id="ou_...">Name</at>` (gateway + tool) with tests.
- [x] Update `docs/reference/LARK_MENTIONS.md` with the correct protocols and steps to @ another bot.
- [x] Run `go test ./...` + repo lint targets, then merge back to `main` (ff preferred).
