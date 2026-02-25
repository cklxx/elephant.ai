# Fix: Model Subscription Should Be Agent-Global for Lark

## Status: COMPLETED (2026-02-08)

## Problem

Model subscription (`/model use <provider>/<model>`) stored selections per (channel, chatID, userID). A model set in a private chat was invisible in a group chat. Both bots in group would fail with LLM call errors.

Root cause: `applyPinnedLarkLLMSelection` did an exact key lookup `lark:chat=<chatID>:user=<userID>` with no fallback.

## Solution

- Default scope: **channel-level** (`key="lark"`) — agent-global across all Lark chats
- Optional `--chat` flag: **per-chat scope** (`key="lark:chat=xxx:user=yyy"`) — override for specific chat
- Lookup: **fallback chain** → chat-specific first, then channel-level
- Existing per-chat entries in store remain functional as overrides

## Changes

### Commit 1: `GetWithFallback` on `SelectionStore`
- `internal/app/subscription/selection_store.go` — new `GetWithFallback(ctx, scopes...)` method
- Single lock acquisition + single file read for all lookups
- 4 new tests: ChatSpecificFirst, ChannelFallback, NoneFound, ReturnsMatchedScope

### Commit 2: Scope helpers + fallback in `applyPinnedLarkLLMSelection`
- `internal/delivery/channels/lark/model_command.go`:
  - Replaced `modelSelectionScope()` with `channelScope()` and `chatScope(msg)`
  - `applyPinnedLarkLLMSelection` uses `GetWithFallback(ctx, chatScope, channelScope)`
  - `buildModelStatus` shows `[全局]` or `[当前会话]` scope label
  - `setModelSelection` / `clearModelSelection` default to channel scope

### Commit 3: `--chat` flag + usage/card updates + tests
- `handleModelCommand` parses `--chat` flag via `hasFlag()` / `firstNonFlag()` helpers
- `setModelSelection(ctx, msg, spec, chatOnly)` — stores at chatScope when `chatOnly=true`
- `clearModelSelection(ctx, msg, chatOnly)` — same pattern
- Updated usage text and card note to reflect global scope
- 9 new tests covering all behaviors

## Verification

- `go vet` clean
- `go test ./internal/app/subscription/...` — all pass
- `go test ./internal/delivery/channels/lark/...` — all pass (existing + 13 new tests)
