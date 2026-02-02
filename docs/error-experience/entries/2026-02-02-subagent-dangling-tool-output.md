# 2026-02-02 - Subagent dangling tool output causes 400

## Error
- Subagent LLM streaming failed with `Request was rejected by the upstream service` (HTTP 400).
- Requests to the OpenAI Responses API included `function_call_output` for a subagent call without the corresponding `function_call` in the message history.

## Impact
- Subagent executions failed, surfacing `LLM call failed` errors in the web UI.

## Root Cause
- `buildSubagentStateSnapshot` removed the assistant message containing the subagent tool call but left the tool-output message for the same call ID, producing an invalid Responses input.

## Remediation
- Prune tool-output messages associated with the removed subagent tool call ID when building subagent snapshots.
- Add tests to ensure tool-output pruning behavior is preserved.

## Status
- fixed
