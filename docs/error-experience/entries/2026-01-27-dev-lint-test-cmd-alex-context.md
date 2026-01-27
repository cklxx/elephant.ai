# 2026-01-27 - dev lint/test blocked by cmd/alex context errors

## Error
- `./dev.sh lint` failed with typecheck errors in `cmd/alex/cost.go` (missing `context` import) and `cmd/alex/acp.go` (unused `context` import).
- `./dev.sh test` failed with `cmd/alex/acp.go` unused `context` import, so the full test suite could not complete.

## Impact
- Full lint + test validation cannot pass, blocking engineering-practices compliance for this change set.

## Notes / Suspected Causes
- The errors were introduced while switching CLI entry points to `cliBaseContext()`, which required import adjustments.

## Remediation Ideas
- Add the missing `context` import in `cmd/alex/cost.go` and remove the unused import in `cmd/alex/acp.go` (or use `context` where required), then rerun lint/test.

## Resolution (This Run)
- Added the missing `context` import in `cmd/alex/cost.go` and removed the unused import in `cmd/alex/acp.go`; reran lint/test successfully.
