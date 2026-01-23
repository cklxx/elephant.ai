# Attachment Persistence + PPTX Image Type Reliability

**Goal:** Ensure attachments survive dev restarts by storing payloads before session persistence, and ensure PPTX image ingestion respects response content-type to avoid invalid PNG errors.

## System View
- Session persistence strips inline attachment payloads; only stable URIs survive in the session store.
- Without migrating inline payloads into the attachment store, attachments disappear after restart.
- `resolveAttachmentBytes` currently trusts fallback MIME types over HTTP response headers; incorrect fallback (e.g., `.png` guess) can mislabel WebP/JPEG and break PDF/PPTX rendering.

## Plan
1) Run attachment migrator during session persistence to rewrite inline payloads/remote URLs into stable attachment-store URIs.
2) Prefer HTTP response Content-Type over fallback MIME guesses when fetching attachment bytes.
3) Add tests for MIME precedence and attachment migration in session persistence.
4) Run full lint/tests.

## Progress Log
- 2026-01-22: Planned fixes for attachment persistence and MIME precedence.
- 2026-01-22: Implemented session persistence migration + MIME precedence fix; added tests for migrator usage and header preference; recorded error-experience entry.
- 2026-01-22: Ran `./dev.sh lint` and `./dev.sh test` (passes; happy-dom AbortError noise after vitest teardown).
