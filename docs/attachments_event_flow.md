# Attachment Event Flow

This repo moves attachments from tools to the final answer through a shared registry. The current flow and guardrails are:

- **Backend emission**: tools emit `tool_call_complete` events with `attachments` plus `metadata.attachment_mutations` (add/update/remove/snapshot). The SSE handler de-duplicates payloads per connection but keeps mutation metadata intact.
- **Frontend pipeline**: SSE events pass through the `EventPipeline` → `defaultEventRegistry`. The `attachmentRegistry` merges `attachment_mutations` (snake- or camel-case, string or object) into the event payload, updates its store, and records what a tool has already shown.
- **Task completion handling**: the registry hydrates `task_complete` events from the store and placeholder references. After streaming chunks are merged in `useSSE`, the registry runs again so placeholders that span chunks (e.g., `[file` + `].md]`) resolve with the full final answer instead of the last delta only.
- **Preview isolation**: final answers can reuse attachments even if a tool already previewed them; the registry will surface stored attachments once streaming is finished rather than treating tool previews as “consuming” the asset.
- **Fallbacks**: when the final event still lacks attachments, the registry pulls from its store so artifacts created earlier remain available in the last answer card.
