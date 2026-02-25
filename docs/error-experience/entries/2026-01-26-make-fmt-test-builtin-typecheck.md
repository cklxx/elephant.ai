# 2026-01-26 - make fmt/test blocked by builtin typecheck failures

## Error
- `make fmt` failed during `golangci-lint run --fix ./...` with typecheck errors in `internal/tools/builtin` (`unwrapArtifactPlaceholderName`, `buildAttachmentStoreMigrator`), `internal/tools/builtin/execution` (`parentListenerKey`), and `internal/tools/builtin/sandbox` (attachment helpers), plus a missing embed asset in `internal/tools/builtin/artifacts/pptx_from_images.go`.
- `make test` failed with the same underlying build errors (toolregistry/builtin/execution/sandbox/artifacts), so the full test suite could not complete.

## Impact
- Full lint + test validation cannot pass, blocking engineering-practices compliance for this change set.

## Notes / Suspected Causes
- These errors appear pre-existing and unrelated to the fileops move; they surface whenever the full `./...` build is typechecked.
- Missing embedded asset (`assets/pptx_blank_template.pptx`) suggests a repo content mismatch or build tag mismatch.

## Remediation Ideas
- Restore or vendor the missing PPTX template asset or gate it behind build tags for test runs.
- Reintroduce/repair the missing helpers in builtin/execution/sandbox (or adjust build tags) so `./...` typecheck passes.

## Resolution (This Run)
- None; left unchanged due to scope constraints (no registry wiring or shared helper changes).
