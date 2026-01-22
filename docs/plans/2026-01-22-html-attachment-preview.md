# HTML Attachment Preview Click Fix

**Goal:** Make HTML attachment previews open reliably, including when the attachment only has inline data (no URI).

## System View
- Attachment previews are rendered by `ArtifactPreviewCard` with optional inline previews for Markdown/HTML.
- HTML previews rely on `resolveAttachmentDownloadUris` for usable content URLs and load via `loadHtmlSource`.
- Inline-data attachments can yield a usable download URI but were blocked by the `htmlAsset` gate, preventing click-to-open.

## Plan
1) Allow HTML previews when a usable HTML source URI exists (even without preview assets).
2) Ensure dialog rendering uses the HTML path for inline-data attachments.
3) Add a test covering HTML attachments with inline data.
4) Run full lint + tests and log results.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Allowed HTML previews when an HTML source URI is available; adjusted dialog gating.
