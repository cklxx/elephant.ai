# Plan: Subagent Upstream Rejection (400)

## Status: Completed
## Date: 2026-02-02

## Problem
Subagent runs intermittently fail with `LLM call failed: Request was rejected by the upstream service. Streaming request failed...` when using OpenAI Responses API. Logs show 400 responses with requests that include a `function_call_output` for a subagent call without the corresponding `function_call` in the message history.

## Plan
1. Trace subagent snapshot construction and confirm dangling tool output messages in request payloads.
2. Update snapshot pruning to remove tool outputs tied to the removed subagent tool call.
3. Add/adjust tests covering tool-output pruning for subagent snapshots.
4. Run lint/tests and document the incident.

## Progress
- [x] Trace request payloads and identify dangling tool outputs.
- [x] Prune tool outputs for removed subagent calls.
- [x] Add tests for tool-output pruning.
- [x] Run lint/tests and record incident summary.
