# Lark Attachment & Reply Fixes

**Date:** 2026-01-29
**Status:** Done

## Issues

1. **Emoji reaction not working** — `addReaction` uses event handler context which may be cancelled
2. **Markdown files not uploaded** — Lark file API only accepts: opus, mp4, pdf, doc, xls, ppt, stream; current code sends "md" as file_type
3. **Only send final message attachments + strip placeholders** — Currently collects from ALL messages; placeholders leak into reply text

## Changes

### 1. Fix emoji reaction — `internal/channels/lark/gateway.go`
- Use `context.Background()` for the reaction API call (detach from event handler lifecycle)

### 2. Fix Lark file type mapping — `internal/channels/lark/gateway.go`
- Add `larkFileType()` helper that maps file extensions to Lark-supported types
- Unsupported extensions → "stream"

### 3. Add Attachments to TaskResult — `internal/agent/ports/agent/types.go`
- Add `Attachments map[string]core.Attachment` field

### 4. Strip all placeholders from final answer — `internal/agent/domain/react/attachments.go`
- Change `ensureAttachmentPlaceholders` to strip ALL `[placeholder]` patterns without re-appending

### 5. Store resolved attachments on TaskResult — `internal/agent/domain/react/finalize.go`
- `finalize()`: store resolved attachments on result
- `decorateFinalResult()`: merge a2ui attachments onto result

### 6. Use TaskResult.Attachments in gateway — `internal/channels/lark/gateway.go`
- `sendAttachments()`: prefer `result.Attachments` over `collectAttachmentsFromResult`

### 7. Update tests
- Update `ensureAttachmentPlaceholders` tests for new strip-only behavior
- Add test for Lark file type mapping
- Update `decorateFinalResult` tests

## File List
- `internal/agent/ports/agent/types.go`
- `internal/agent/domain/react/attachments.go`
- `internal/agent/domain/react/finalize.go`
- `internal/channels/lark/gateway.go`
- `internal/channels/lark/gateway_test.go`
- `internal/agent/domain/react/engine_internal_test.go`
