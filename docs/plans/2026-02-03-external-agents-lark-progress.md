# Codex/Claude Code External Execution + Lark 10-Min Progress Updates

Owner: Codex CLI agent

## Goal
- Make `bg_dispatch(agent_type=codex|claude_code)` work end-to-end in Lark.
- Collect external agent notifications/metadata, aggressively de-noise (esp. Codex `codex/event` raw templates).
- If task not finished: every 10 minutes, summarize progress based on the most recent 10-minute window and update a single reply message in Lark.
- If task finished: immediately update the same reply message with final result.
- `bg_dispatch` task id must be system-generated; user-supplied `task_id`/`id` must error.

## Acceptance
- No more `external agent executor not configured for type "codex"` in normal development env when `codex` CLI exists (auto-enable) and executor registered.
- MCP client is compatible with Codex MCP: `initialize.capabilities` present; request IDs route correctly; notifications handled; no hardcoded 30s timeout.
- Codex executor consumes `codex/event` notifications -> progress callback + thread id metadata.
- Claude Code executor path has unit tests verifying progress + usage parsing.
- Lark: progress listener replies and updates only one message; 10m interval summaries; immediate final update.

## Scope
In scope:
- MCP layer fixes
- External executors (codex + claude_code)
- Domain events for external progress
- Lark background progress listener + wiring
- Config auto-enable for dev env

Out of scope:
- Production auto-enable (kept dev-only by default)

## Work Items / Commits
1) Plan file (this doc)
2) MCP fixes + tests
3) Codex executor: notification parsing/filtering + tests
4) Claude Code executor: injectable subprocess + tests
5) Domain: enrich external progress events
6) Lark: background progress listener + tests
7) Config: auto-enable executors (dev-only) + tests

## Notes
- Keep changes isolated: only commit our work (explicit path staging).
- Prefer reply-to-original-message in group chats; update one message to avoid spam.

## Status
- Implemented end-to-end; unit tests and lint are green (`CGO_ENABLED=0 go test ./...`, `./scripts/run-golangci-lint.sh run ./...`).
