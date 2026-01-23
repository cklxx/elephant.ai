# 2026-01-22 - attachments lost after restart + wrong MIME fallback

- Summary: Inline attachments were dropped during session persistence and fallback MIME guesses overrode HTTP headers, leading to missing attachments after restart and invalid PNG errors in PPTX/PDF generation.
- Remediation: run attachment migration before saving sessions and prioritize HTTP Content-Type when resolving attachment bytes.
