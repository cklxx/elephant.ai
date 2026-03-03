# Plan: Remove Dead `workflow.replan.requested` Event Chain

Date: 2026-03-03
Owner: Codex

## Goal
Remove unused `workflow.replan.requested` event definitions/translators/schemas after runtime no longer emits replan events.

## Scope
- Domain event type/constructor
- Coordinator workflow translator + tests
- SSE allowlist + tests
- Web event types/schemas/normalize tests

## Steps
- [x] Locate all internal/web references to `workflow.replan.requested`.
- [x] Remove domain event constant and constructor.
- [x] Remove translator and SSE mapping/tests.
- [x] Remove web schema/type/test entries.
- [x] Run targeted backend and web tests.
- [ ] Commit.
