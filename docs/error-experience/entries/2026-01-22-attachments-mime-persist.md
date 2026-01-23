# 2026-01-22 - attachments lost after restart + wrong MIME fallback

- Error: Session persistence stripped inline attachment payloads without first migrating to the attachment store, so attachments disappeared after dev restarts; `resolveAttachmentBytes` also preferred fallback MIME types over HTTP response headers, causing PPTX/PDF to treat non-PNG payloads as PNG.
- Remediation: migrate attachments during session persistence to stable store URIs, and prefer HTTP response Content-Type when resolving attachment bytes.
