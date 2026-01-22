# 2026-01-22 - attachment uri fetch blocked pptx and sandbox write

- Error: Image attachments were normalized to remote URIs only; `sandbox_write_attachment` required inline payloads and `pptx_from_images` used default HTTP/TLS settings, causing attachment-not-found and TLS handshake timeouts when fetching images.
- Remediation: introduce a shared attachment resolver that can fetch URI payloads with longer TLS handshake timeout, and use it in `sandbox_write_attachment` + `pptx_from_images`.
