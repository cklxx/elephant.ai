# 2026-01-26 - make fmt/test fail due to missing builtin helpers/assets

## Error
- `make fmt` failed with typecheck errors: undefined `unwrapArtifactPlaceholderName`, `buildAttachmentStoreMigrator`, and `parentListenerKey`, plus missing embedded asset `assets/pptx_blank_template.pptx`.
- `make test` failed with the same missing helper/asset errors across builtin packages (`internal/tools/builtin`, `internal/tools/builtin/execution`, `internal/tools/builtin/sandbox`, `internal/tools/builtin/artifacts`).

## Impact
- Full lint and test validation cannot complete.

## Notes / Suspected Causes
- Recent builtin refactors likely moved helper symbols/assets without updating references or embed paths.
- The missing PPTX template likely needs to be restored under `internal/tools/builtin/artifacts/assets/` or the embed path updated.

## Resolution (This Run)
- Not resolved; recorded for follow-up.
