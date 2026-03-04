# Session System Prompt Multiplicity Misread (2026-03-04)

## What happened
Session `lark-3ASwX52mwXVZ1E54p4RxMPkpYVn` was suspected of injecting the same core system prompt four times.

After tracing request logs end-to-end, the observed "4 system prompts" came from **4 separate LLM requests under the same log_id**, not from duplicate system injection inside one request payload.

## Impact
- Debug time was spent on a suspected prompt-duplication bug that was not present.
- Signal-to-noise in session-level inspection made request-stage boundaries easy to misread.

## Root cause
Investigation target mixed two levels:
1. **Session/log_id-level aggregation** (multiple LLM calls in one run).
2. **Single request payload-level prompt construction** (where duplication would actually be a bug).

For codex endpoint calls, `system/developer` messages are merged into a single `instructions` field, so each request has one effective system instruction bundle.

## Evidence
- `log-3ASz9R3x378ZHoL1soriHsMcnpX` has 4 request ids:
  - `llm-3ASz9SEftofCodqStgRlHWUdvtH` (main)
  - `llm-3ASzBWO2OnqNCF2WFJRdDjCyZ2T` (main)
  - `llm-3ASzCzYPsmDVz2iih6Rlyd6zYmH` (memory_extract)
  - `llm-3ASzDMHrzH51xTqvSZ5GL5Cu8EN` (rephrase)
- In codex requests, system/developer content is collected into one `instructions` string:
  - `internal/infra/llm/openai_responses_input.go` (`buildResponsesInputAndInstructions`)
- Rephrase requests intentionally add their own rephrase system prompt:
  - `internal/delivery/channels/lark/rephrase.go`

## Preventive rule
When checking "duplicate system prompt" issues:
1. Count duplicates **within a single request payload** first.
2. Separate main/memory/rephrase requests before concluding prompt duplication.
3. Only file a prompt-injection bug when one request contains repeated core prompt blocks.

## Validation checklist for next time
- Confirm whether the report is at `request_id` level or `log_id/session` level.
- For codex requests, inspect `instructions` body content frequency, not only request count.
- Distinguish auxiliary LLM passes (`memory_extract`, `rephrase`) from main reasoning loop.

## Metadata
- id: err-2026-03-04-session-system-prompt-multi-call-misread
- tags: [observability, prompt, lark, debugging]
- links: []
