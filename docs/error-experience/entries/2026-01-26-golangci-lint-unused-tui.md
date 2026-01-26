# 2026-01-26 - golangci-lint reports unused shouldForceLineInput

## Error
- `./dev.sh lint` failed: `cmd/alex/tui.go:60:6: func shouldForceLineInput is unused (unused)`.

## Impact
- Full lint step could not complete for this run.

## Notes / Suspected Causes
- Likely a leftover helper after recent TUI refactors; the function is no longer referenced.

## Resolution (This Run)
- Not resolved; recorded for follow-up.
