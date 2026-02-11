# Plan: Lark think-step upstream rejection log hardening

## Status: Completed
## Date: 2026-02-11

## Problem
Lark chats intermittently fail with:
`task execution failed: think step failed: LLM call failed: Request was rejected by the upstream service. Streaming request failed after 1s.`

Runtime evidence showed malformed Responses API history payloads could contain orphan `function_call_output` items without matching `function_call` context.

## Goals
1. Confirm root cause from runtime request logs with minimal-noise extraction.
2. Add a safety guard in OpenAI Responses input construction to drop orphan `function_call_output` entries.
3. Add warning logs that include dropped `call_id`s for future diagnosis.
4. Add regression tests.
5. Restart Lark service and verify deployment/status after fix.

## Plan
1. Extract compact log evidence from `logs/lark-main.log` and `logs/requests/llm.jsonl`.
2. Implement input sanitizer in `internal/infra/llm/openai_responses_input.go`.
3. Emit warning logs from stream/complete paths with request prefix.
4. Add unit tests for orphan tool-output pruning.
5. Run full lint/tests.
6. Merge to `main`, restart/reconcile supervisor, and validate deployed SHA.

## Progress
- [x] Confirmed failures in main log and identified offending timestamp window (`2026-02-11 10:47:16+08:00`).
- [x] Confirmed orphan `function_call_output` (`call_VSG4rv0zODNqJdVQKacOqVoz`) in failing request payload.
- [x] Confirmed original `call_VSG...` source existed earlier as a real tool call (`2026-02-10 19:12:32+08:00`), then later reappeared as orphan output-only history entries.
- [x] Implemented sanitizer + warning logs.
- [x] Added checkpoint recovery repair to inject missing assistant tool_call context before recovered tool outputs.
- [x] Added regression tests.
- [x] Ran lint/tests.
- [x] Merged to `main` and reconciled supervisor; `main/test` deployed SHA now `bba539aee68f6a742c5481eeb771531a79f1114a`.

## Code Review Summary
- Scope: 7 files changed (core fix + tests), plus this plan file.
- Checklist dimensions covered: SOLID/architecture, security/reliability, correctness, code quality/edge cases, cleanup.
- Result: No P0/P1 blocking issues identified for this patch set.

## Evidence Bundle
- Compact evidence log: `/tmp/lark-think-failure-compact-20260211-112238.log`
