# 2026-01-26 - dev.sh lint blocked by unused styleYellow

## Error
- `./dev.sh lint` failed in `golangci-lint` with: `cmd/alex/tui_styles.go:9:2: var styleYellow is unused`.

## Impact
- Full lint step cannot complete for this run.

## Notes / Suspected Causes
- Likely leftover styling after recent TUI refactors; the variable is no longer referenced.

## Remediation Ideas
- Remove `styleYellow` or wire it to an actual usage in the TUI renderer.

## Resolution (This Run)
- Not resolved; left unchanged (out of scope for share page fix).
