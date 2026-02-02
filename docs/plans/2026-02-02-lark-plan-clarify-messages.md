# Lark Plan + Clarify Message Emission Plan

## Goal
Send plan/clarify tool outputs as Lark messages (without [Plan]/[Clarify] prefixes), with config gating and no duplicate await-user-input replies.

## Scope
- Add Lark config flag to enable plan/clarify message emission.
- Emit messages on plan/clarify tool completion events.
- Suppress duplicate await_user_input reply when question already sent.
- Add tests for listener + gateway integration.

## Plan
1) Add Lark config flag + YAML wiring for plan/clarify message emission.
2) Implement plan/clarify event listener and message extraction.
3) Wire listener into gateway and suppress duplicate await_user_input reply.
4) Add unit tests for listener + update scenario config types.
5) Run lint + tests.

## Progress
- 2026-02-02: Plan created.
- 2026-02-02: Added Lark config flag + YAML wiring for plan/clarify message emission.
- 2026-02-02: Implemented plan/clarify listener with message extraction and await-user-input dedupe.
- 2026-02-02: Wired listener into gateway; updated scenario config + tests.
- 2026-02-02: Ran `./dev.sh lint` and `./dev.sh test`.
