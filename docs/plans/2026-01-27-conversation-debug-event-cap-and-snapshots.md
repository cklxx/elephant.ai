# Plan: Expand conversation debug event buffer + show latest turn snapshot

## Goal
- Allow inspecting more than 500 SSE events in `/dev/conversation-debug`.
- Show latest turn snapshot (including messages) even when session record has `messages: null`.

## Context
- Event buffer is capped at 500 in the debug page.
- Session storage may be Postgres (`session.database_url`), so session dir is not used.
- Turn snapshots already exist via `/api/sessions/:id/snapshots` and `/api/sessions/:id/turns/:turn`.

## Steps
1. Make event cap configurable in the debug UI (default > 500).
2. Fetch latest turn snapshot alongside session/task payloads and render it in the debug UI.
3. Update API client helpers for snapshot endpoints.
4. Run full lint + tests.

## Progress Log
- 2026-01-27: Plan created.
