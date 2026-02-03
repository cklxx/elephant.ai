# Plan: Skills Catalog Prompt Cleanup + Memory Indexing Doc Split

Date: 2026-02-03

## Context
- The system prompt currently injects a full "Other Available Skills" catalog (names + descriptions). This bloats the prompt and duplicates what the `skills` tool can provide on demand.
- The memory SOP doc `docs/reference/MEMORY_SYSTEM.md` includes technical indexing/search implementation details that are not useful for the LLM at runtime. Those details should live in a separate reference doc and not be injected into the LLM prompt.

## Goals
1. Stop injecting the full skills catalog into the system prompt; keep skill discovery via the `skills` tool.
2. Keep the LLM-facing memory SOP high-signal (boot/read/write/retrieve) and move indexing/search internals into a separate file.
3. Keep behavior deterministic, tested, and token-efficient.

## Non-goals
- Changing the skill matcher/scoring logic.
- Changing the memory engine implementation.
- Changing web skill catalog generation.

## Changes
### Skills prompt
- Replace "Other Available Skills" list with a short instruction pointing to `skills(action=list|search|show)`.
- Ensure the system prompt never embeds the full skills index by default.

### Memory docs
- Remove / slim the indexing implementation section from `docs/reference/MEMORY_SYSTEM.md`.
- Add `docs/reference/MEMORY_INDEXING.md` for the technical details.
- Update `configs/context/knowledge/memory.yaml` so LLM injection does not include the technical indexing section.

## Test plan
- Unit test: system prompt composition does not contain "Other Available Skills" / skill catalog bullets.
- Unit test: still renders activated skills when auto-activated.
- Run full lint + tests.

## Progress
- [x] Update skills prompt rendering
- [x] Split memory docs + update knowledge refs
- [x] Add/adjust tests
- [x] gofmt + full lint/tests
- [x] Commit (incremental) + merge back to `main`
