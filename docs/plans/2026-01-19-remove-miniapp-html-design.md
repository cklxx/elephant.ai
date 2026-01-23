# Remove MiniAppHTML Tool Design

## Overview
MiniAppHTML is redundant now that artifacts/file tools can create HTML that the UI renders directly. This change removes the tool entirely and makes the artifacts tool family explicitly advertise HTML generation as the supported path.

## Goals
- Remove `miniapp_html` from the tool surface (implementation, registry, tests).
- Preserve HTML generation via `artifacts_write` with explicit tool descriptions.
- Avoid any impact on attachment placeholder replacement and LLM catalog behavior.

## Non-Goals
- Replacing MiniAppHTML with a new generator tool.
- Changing UI rendering for HTML artifacts.

## Architecture
- Delete the `miniapp_html` tool implementation and registration.
- Remove registry tests that require `miniapp_html`.
- Update artifacts tool definitions to mention HTML generation (e.g., `media_type: text/html`, `format: html`).
- Update documentation references that mention MiniAppHTML.

## Data Flow
- LLM generates HTML content and calls `artifacts_write` to save `*.html`.
- Attachments remain inline in agent/tool flow; HTML is externalized only at the HTTP/SSE boundary.

## Risks
- Workflows calling `miniapp_html` will fail after removal.
- Downstream docs or prompts may need manual updates.

## Testing
- Add a regression test to confirm artifacts tool definition mentions HTML.
- Remove the MiniAppHTML registry test.
- Run `go test ./...` after changes.
