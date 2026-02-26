# 2026-02-26 — Single-embed mask for history attachments in LLM message conversion

Impact: Eliminated repeated historical file payload embedding in multi-turn requests while preserving latest-turn attachment availability.

## What changed

- Introduced `attachmentEmbeddingMask` in `internal/infra/llm/message_content.go`.
- Restrained attachment embedding to the **latest eligible user message** only.
- Explicitly excluded `user_history` / `debug` / `evaluation` / `tool_result` sources from attachment embedding.
- Applied the mask consistently across:
  - `internal/infra/llm/openai_client.go`
  - `internal/infra/llm/anthropic_client.go`
  - `internal/infra/llm/openai_responses_input.go`
- Added regression tests covering all three conversion paths.

## Why this worked

- Kept behavior change local to message-conversion boundary (minimal blast radius).
- Preserved text history while preventing repeated binary/image payload injection.
- Unified policy across providers to avoid per-provider drift.

## Validation

- `go test ./internal/infra/llm -count=1`
- `./scripts/pre-push.sh`

## Metadata
- id: good-2026-02-26-history-attachment-single-embed-mask
- tags: [good, llm, attachments, history, lark, prompt-efficiency]
- links:
  - docs/plans/2026-02-26-lark-message-flow-history-attachment-thinking.md
  - internal/infra/llm/message_content.go
