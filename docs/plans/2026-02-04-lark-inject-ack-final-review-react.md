# 2026-02-04 — Lark: injected message ACK + final review 👀 + group/@ docs

## Goal

Improve Lark IM UX for long-running tasks:

- ACK injected user messages (during an active session) immediately with a reaction.
- Make the existing “final answer review” extra iteration observable via a reaction.
- Document Lark platform constraints for receiving all group messages and how @mentions work.

## Scope

In scope:

- Lark gateway only (no new channel-agnostic `send_message` tool).
- ReAct runtime emits synthetic tool events for `final_answer_review` so Lark can react on trigger.
- Config additions under `channels.lark` for reaction emoji types.
- Docs updates: `docs/reference/CONFIG.md` + a focused mentions guide.

Out of scope:

- Any UI/web changes.
- Tool migration/compat shims.

## Checklist / Progress

- [x] Add `channels.lark.injection_ack_react_emoji` and use it for injected-message ACK reaction.
- [ ] Add `channels.lark.final_answer_review_react_emoji` and react when final review triggers.
- [x] Emit `workflow.tool.started/completed` for `final_answer_review` in ReAct runtime.
- [ ] Add/extend tests for injection ACK + final review events + Lark reaction listener.
- [ ] Update docs for group message permissions + @mention syntax.
- [ ] Run `go test ./...` + `make fmt` + `make vet`, then merge back to `main` (ff preferred).
