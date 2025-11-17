# Attachments Architecture & Tool Compatibility

This document summarizes how attachments move through the system today and what each
component must do to remain compatible. The intent is to give tool authors a single
reference so every built‑in or custom tool behaves consistently when emitting or
consuming binary payloads.

## Terminology

- **Attachment** – A binary payload plus metadata defined by
  `internal/agent/ports.Attachment`. The server accepts either raw Base64 data or an
  already hosted URI, and tools should populate `Source`, `MediaType`, `Description`,
  and `Name` whenever possible.
- **Workspace path** – A sandbox path (e.g.
  `/workspace/.alex/sessions/<session>/attachments/report.txt`) automatically stored
  in `Attachment.WorkspacePath` so tools can open mirrored files without
  reconstructing URLs. These paths stay inside the server/LLM context and are never
  exposed in user-facing events.
- **Placeholder** – Text token in the format `[filename.ext]`. Placeholders are
  inserted into LLM outputs so renderers know which attachment to show inside the
  conversation. The domain layer normalizes placeholder names via
  `normalizeToolAttachments` to prevent duplicates.
- **Attachment map** – Record keyed by placeholder → `Attachment`. Maps flow through
  events and are persisted in `TaskState.Attachments` so future iterations (and
  delegated agents) can reuse previously generated assets.

## End-to-end lifecycle

1. **Upload / task creation**
   - The REST API accepts attachments alongside a task via
     `internal/server/http/api_handler.go`. `HandleCreateTask` validates filenames,
     MIME types, Base64 payloads, and URIs before calling
     `agentapp.WithUserAttachments`. At this point attachments are tagged as
     `user_upload` and stored in context so downstream services can stage them.
   - `HandleCreateTask` normalizes placeholder-friendly names (stripping
     whitespace and illegal characters) through
     `internal/server/http/attachment_validation.go` so downstream components can
     rely on deterministic keys when referencing user uploads.
   - As soon as the `user_task` event is emitted the server-side
     `EventBroadcaster` forwards attachments to the sandbox archiver, which saves
     binaries under `/workspace/.alex/sessions/<session>/attachments` so tools
     can access user uploads just like generated assets. When an attachment only
     specifies an HTTP(S) URI the archiver now downloads the payload directly (up
     to 8 MiB per file) before mirroring it in the workspace, keeping the local
     catalog consistent regardless of where users originally hosted the file.
    Operators can restrict which hosts are eligible for mirroring by setting
    `ATTACHMENT_REMOTE_HOST_ALLOWLIST` / `ATTACHMENT_REMOTE_HOST_DENYLIST`; the
    archiver rejects downloads that fall outside these allow/deny rules so the
    workspace never ingests binaries from unknown domains. Before any bytes are
    written to disk, the archiver now calls the pluggable
    `AttachmentScanner` (configured via `ATTACHMENT_SCAN_WEBHOOK` plus optional
    `ATTACHMENT_SCAN_SECRET`/`ATTACHMENT_SCAN_TIMEOUT_MS`). The default HTTP
    scanner posts Base64 payloads and metadata to the provided webhook and
    expects a `{verdict:"clean"|"infected", details:"..."}` response; infected
    files are quarantined by simply skipping the archive step so downstream LLMs
    never see unsafe data. Whenever the scanner flags an attachment as
    infected, the archiver notifies the `EventBroadcaster`, which emits an
    `attachment_scan_status` event. The event includes the placeholder,
    scanner-provided details, and attachment metadata so the UI can show a
    friendly "attachment blocked" message without revealing sandbox paths.
2. **Execution preparation**
   - `ExecutionPreparationService` composes the `TaskState` with all available
     attachments. It merges previously archived assets (from the session), any
     attachments inherited through subagent delegation, and user uploads. Iteration
     counters are initialized so we know when each placeholder first appeared.
  - `ExecutionPreparationService` now keeps the richest metadata when merging
    attachments (`internal/agent/app/execution_preparation_service.go` prefers
    delegated > user > archived entries and backfills missing descriptions,
    sizes, and sources).
3. **ReAct loop ingestion**
   - When `ReactEngine` starts solving a task it copies
     `state.PendingUserAttachments` into the next user message, registers every
     attachment (to keep the catalog message up to date), and clears the pending set.
     Later on, each call to `resolveContentAttachments` and
     `ensureAttachmentPlaceholders` enforces that every attachment referenced in the
     final answer has a placeholder and Markdown fallback.
  - The catalog message includes attachment provenance, workspace paths (for LLM
    consumption only), and approximate payload sizes because
    `resolveContentAttachments` enriches records with `Source`, `WorkspacePath`, and
    `SizeBytes` metadata.
4. **Tool execution**
   - Right before invoking a tool the engine captures a snapshot of
     `state.Attachments` and `state.AttachmentIterations` and injects it into the
     `context.Context` via `ports.WithAttachmentContext`. This is the canonical way
     for tools to read available attachments.
   - When a tool returns a `ToolResult.Attachments` map the engine normalizes the
     keys, merges them into `state.Attachments`, updates iteration counters, and
     emits them with the corresponding `ToolCallCompleteEvent`. Messages stored in
     history also receive the normalized map so placeholder substitution works when
     replaying context to the LLM.
  - `normalizeToolAttachments` (see `internal/agent/domain/react_engine.go`)
    preserves case-sensitive placeholders and automatically appends `-N`
    suffixes when duplicates appear, preventing collisions in the catalog.
5. **Delegated agents**
   - The subagent tool calls `ports.GetAttachmentContext` to capture the shared
     snapshot, then propagates it to each delegated task via
     `agentapp.WithInheritedAttachments`. Execution preparation treats these as
     preloaded assets so nested agents can reference parent attachments without
     re-uploading.
  - `agentapp.WithInheritedAttachments` records the parent task ID on each
    propagated attachment, giving audit logs a consistent provenance trail for
    delegated work.
6. **Task completion & streaming**
   - `decorateFinalResult` resolves any `[placeholder]` tokens that appear in the
     final response, ensures Markdown fallbacks exist for attachments that were not
     mentioned explicitly, and surfaces the attachment map on the final
     `TaskCompleteEvent`.
   - On the client side, `web/lib/events/attachmentRegistry.ts` tracks every map
     received from user_task, tool_call_complete, and task_complete events. It
     deduplicates placeholders that were already shown via tool events and lazily
     resolves any placeholders left in the final answer text so the React UI can show
     a single gallery per task.
  - `attachmentRegistry.ts` persists consolidated maps through
    `web/lib/stores/taskStore.ts`, so page reloads and history views reuse the
    same attachment ordering established during live streaming. Workspace paths are
    intentionally stripped before events reach the client, so the React UI never
    exposes sandbox locations.
7. **Session teardown & CDN staging**
   - `EventBroadcaster` now caches the union of every attachment emitted during a
     session. When the last SSE client disconnects (typically when the user closes the
     tab), it drains that cache and invokes the configurable
     `AttachmentExporter` hook.
   - Operators can supply `ATTACHMENT_EXPORT_WEBHOOK` to have the exporter POST
     a JSON payload to their CDN ingress service (`cmd/alex-server/main.go` wires the
     environment variable to `NewHTTPAttachmentExporter`). Retries/backoff can be tuned
     via `ATTACHMENT_EXPORT_MAX_ATTEMPTS`/`ATTACHMENT_EXPORT_BACKOFF_MS`, and
     `ATTACHMENT_EXPORT_SECRET` enables HMAC-SHA256 signing (header
     `X-Attachment-Signature`) so downstream workers can verify the payload
     authenticity. This leaves a clean extension point so uploads can be replicated to
     durable storage as soon as users leave the app, instead of only living inside the
     sandbox workspace.
  - Once an export attempt finishes the broadcaster emits an
    `attachment_export_status` event. The event captures success/failure, attempt
    counts, duration, exporter metadata, and any CDN-supplied attachment updates
    (such as signed HTTPS URLs). The broadcaster also applies those updates to the
    stored attachment maps so when users reopen a session the UI immediately renders
    the CDN-hosted links instead of stale workspace mirrors.

## Tool authoring guidelines

Every tool must follow the same contract to stay compatible with the global
attachment lifecycle:

1. **Read inputs via context**
   - Use `ports.GetAttachmentContext(ctx)` inside `ToolExecutor.Execute` to access
     the latest attachments and their iteration metadata. Never reach into domain
     state directly – the context snapshot is the supported API.
2. **Reference attachments explicitly**
   - When tool output text discusses a generated asset, include its placeholder
     (e.g., `"See [analysis.png] for details"`). `ensureToolAttachmentReferences`
     automatically appends a checklist of available placeholders if the tool omitted
     them, but explicitly mentioning the placeholder produces better UX.
3. **Return normalized metadata**
  - Populate `ToolResult.Attachments` with deterministic placeholder keys. If the
    key is empty the engine will fall back to `Attachment.Name`, but providing both
    avoids accidental clashes. Fill `MediaType`, `Description`, and either `URI` or
    `Data`. When only `Data` is available the server will synthesize a `data:` URI
    before events reach the client, and it will add a `workspace_path` (for internal
    use) when the placeholder is deterministic.
4. **Keep payload sizes reasonable**
   - Attachments are echoed through events and persisted with the session history.
     Prefer referencing remote URIs for large files. Built-in tooling such as
     `web_fetch` already emits trimmed attachments (for example thumbnails) that fit
     within event limits.
5. **Propagate attachments in delegated scenarios**
   - Tools that spawn nested agent work (e.g., subagent, future planner tools) must
     wrap child contexts with `agentapp.WithInheritedAttachments` so downstream
     agents inherit the current attachment catalog. Failing to do so results in
     placeholder resolution errors when children attempt to reference parent assets.

Following the above rules keeps attachments discoverable in the UI, allows LLMs to
reliably refer to binary artifacts, and prevents duplicated uploads across tool
boundaries.

## Outstanding tasks (TODO)

None at this time — the current attachment pipeline (mirroring, scanning,
exporting, and CDN reconciliation) is fully implemented.
