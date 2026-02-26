# 2026-02-26 Tool Orthogonality And Session ToolCall Fidelity

## Context
- User requires non-web tool interfaces to be orthogonal and conflict-free.
- User reports session artifacts with `role=assistant, content=""` but missing `tool_calls`.
- Need evidence from runtime LLM request logs that tool arguments exist upstream.

## Goals
1. Remove/repair conflicting tool parameter semantics in non-web tools.
2. Ensure parsed tool calls are re-injected into persisted assistant messages.
3. Prevent meaningless empty assistant shell messages from polluting session history.
4. Validate with targeted tests and request-log evidence.

## Plan
1. Tool parameter orthogonality
- `lark_upload_file`: replace dual-source args (`path`/`attachment_name`) with single canonical source arg.
- `channel` upload action: align schema to canonical upload arg.
- `run_tasks`: enforce exclusive mode (`file` xor `template`) instead of implicit precedence.
- `execute_code`: enforce exclusive source (`code` xor `code_path`).
- `shell_exec`/`execute_code`: remove duplicated attachment param alias (`output_files`) and keep one canonical `attachments` path.

2. Session fidelity for tool calls
- In ReAct iteration, write back parsed/validated tool calls into assistant message before persistence.
- Avoid persisting assistant messages that have neither visible content nor tool calls/tool outputs.

3. Verification
- Run focused Go tests for touched packages.
- Inspect `logs/requests/llm.jsonl` and session journal/event artifacts for tool call argument presence.

## Progress
- [x] Root cause and evidence collection
- [x] Code changes
- [x] Tests
- [ ] Final verification and commit (blocked: concurrent unexpected workspace edits detected)
