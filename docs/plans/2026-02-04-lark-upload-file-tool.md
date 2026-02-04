# Lark Tool: Upload + Send File (`lark_upload_file`)

**Goal:** Add a builtin tool to upload a file (from local path or task attachment) and send it to the current Lark chat as a `file` message.

## Requirements
- Only available inside a Lark chat context (`*lark.Client` + `chat_id` must be present in tool context).
- Reply target is derived from the current Lark message context (no explicit `reply_to_message_id` parameter).
- Accept exactly one input source:
  - a local filesystem `path` (must stay within working directory), **or**
  - an `attachment_name` from the current task attachment context.
- After uploading, send a `msg_type="file"` message to the current chat.
- Enforce size cap only (no extension allowlist):
  - default `max_bytes` = 20 MiB
  - allow per-call override via `max_bytes`
- Tool must be excluded from tool result cache.

## Plan
1) Add tool skeleton + wire it into the tool registry.
2) Implement upload + send logic:
   - local path mode (within working dir)
   - attachment mode (resolve bytes via attachment resolver)
3) Add unit tests for argument validation and candidate preparation.
4) Run full lint + tests.

## Progress Log
- 2026-02-04: Planned `lark_upload_file` tool implementation.
- 2026-02-04: Wired tool registry + cache exclusion; implemented upload+send logic; added unit tests; ran `make fmt`, `make vet`, `make test`.
