# Plan: Remove final_answer_review runtime branch

Date: 2026-03-03
Owner: Codex

## Context
Lark channel intermittently misses final replies after `final_answer_review` extra-iteration behavior was introduced.
Goal is to remove this branch end-to-end so completion always converges directly to `final_answer`.

## Steps
- [x] Locate all runtime, channel, and config references to `final_answer_review`.
- [x] Remove ReAct final review injection/event path and related tests.
- [x] Remove Lark final review reaction listener and config surface.
- [x] Remove shared config schema/merge/default entries and update docs/examples.
- [x] Run affected tests + mandatory code review script.
- [ ] Commit changes.
