# Tool Render + PPT Attachment Reliability

**Goal:** Fix tool call rendering so tool-start events render via tool UI, and make PPT generation resilient when images are referenced by attachments/URLs.

## System View
- Tool events flow into `ConversationEventStream` → `EventLine`, which chooses between rich tool cards and plain text.
- Tool-start events currently fall back to plain text/JSON for core events, so the UI bypasses the tool renderer.
- Image-driven PPT generation (`pptx_from_images`) and sandbox materialization (`sandbox_write_attachment`) rely on attachment payloads; many image attachments only carry remote URIs after normalization.
- Remote fetches use default TLS handshake timeouts; long handshakes can fail even when overall client timeout is large.

## Plan
1) Render `workflow.tool.started` with `ToolOutputCard` for core events (keep nested/subagent text summary).
2) Add a shared attachment resolver that accepts placeholders, attachment names, and URLs; can fetch remote payloads when only URIs exist.
3) Update `pptx_from_images` and `sandbox_write_attachment` to use the resolver with longer TLS handshake timeout.
4) Add tests for tool-start rendering and remote/URI attachment resolution.
5) Run full lint/tests and log results.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Implemented tool-start rendering via `ToolOutputCard` for core events; added shared attachment resolver + TLS handshake tuning; wired resolver into `pptx_from_images` and `sandbox_write_attachment`; added tests for tool-start rendering and attachment resolver.
- 2026-01-22: Ran `./dev.sh lint` and `./dev.sh test` (passes; happy-dom AbortError noise after vitest teardown).
