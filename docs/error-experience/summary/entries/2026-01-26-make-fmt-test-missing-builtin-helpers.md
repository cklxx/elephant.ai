# 2026-01-26 - make fmt/test fail due to missing builtin helpers/assets

- Summary: `make fmt` and `make test` fail because builtin helper symbols (`unwrapArtifactPlaceholderName`, `buildAttachmentStoreMigrator`, `parentListenerKey`) and the embedded `assets/pptx_blank_template.pptx` are missing.
- Remediation: restore/move the missing helpers, fix their imports, and ensure the PPTX template exists under the embedded path or update the embed directive.
- Resolution: not resolved in this run.
