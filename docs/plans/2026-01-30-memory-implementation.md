# Memory Implementation Plan

> **Status:** In Progress
> **Author:** cklxx
> **Created:** 2026-01-30
> **Updated:** 2026-01-30

## Goals
- Implement Plan 2 (stable sessions + reset + staleness) and Plan 1 Phase 1 (ConversationCaptureHook + cleanup dormant Lark memory manager).
- Keep channel behavior consistent with proactive memory policy and avoid import cycles.

## Plan
1. **Context + config wiring**
   - Add channel/chat/group context helpers and wire Lark/WeChat gateways.
   - Extend config: `lark.session_mode`, `agent.session_stale_after`, `proactive.memory.capture_group_memory`.
   - Ensure server/bootstrap + DI propagate new config values.

2. **Session lifecycle**
   - Add session staleness clearing (session store + history snapshots).
   - Implement `/reset` in Lark by calling coordinator reset (clears session + history).
   - Add coordinator ResetSession API.

3. **Memory capture hooks**
   - Add ConversationCaptureHook for pure chat turns (skip when tool calls present).
   - Enrich MemoryCaptureHook slots with channel/chat metadata.
   - Fix TaskResult user ID resolution to be context-aware.

4. **Cleanup**
   - Remove dormant Lark memory manager + tests.
   - Update docs if needed for implementation details.

5. **Validation**
   - Add/adjust tests for reset, staleness, and new hooks.
   - Run `./dev.sh lint` and `./dev.sh test`.

## Progress Log
- 2026-01-30: Drafted execution plan; began channel context wiring and history manager clear support.
- 2026-01-30: Added channel/chat/group context wiring, stable Lark sessions + /reset handling, session staleness clearing, conversation capture hook, memory capture slot enrichment, removed dormant Lark memory manager, and added tests for staleness + session mode/reset + group recall.
- 2026-01-30: Tests run: `go test ./internal/channels/lark/...`, `go test ./internal/agent/app/hooks/...`, `go test ./internal/agent/app/preparation/...`, `go test ./internal/context/...` OK. Full lint/test failed due to pre-existing import cycle in `internal/external` and undefined `causationID` in `internal/agent/domain/react/background.go`.
