# 2026-01-22 - attachment uri fetch blocked pptx and sandbox write

- Summary: Attachment references were normalized to remote URIs, but `sandbox_write_attachment` expected inline payloads and `pptx_from_images` used default TLS settings, causing attachment-not-found errors and TLS handshake timeouts when fetching images.
- Remediation: add a shared attachment resolver that can fetch attachment URIs (with explicit TLS handshake timeout) and use it in `sandbox_write_attachment` and `pptx_from_images`.
