# 2026-01-26 - dev.sh lint/test blocked by TUI typecheck errors

## Error
- `./dev.sh lint` failed during `golangci-lint` with typecheck errors in `cmd/alex`: duplicated symbols between `tui_tview.go` and `tui_bubbletea.go` (e.g., `tuiAgentName`, `normalizeSubtaskEvent`, `indentBlock`, `maxInt/minInt`) and missing `tcell` identifiers.
- `./dev.sh test` failed to compile `cmd/alex` because `github.com/gdamore/tcell/v2` and `github.com/rivo/tview` modules are missing from `go.mod`.

## Impact
- Full lint + test validation cannot pass, blocking engineering-practices compliance for this change set.

## Notes / Suspected Causes
- The repo includes parallel TUI implementations (Bubble Tea + tview) with overlapping helper names in the same package.
- The tview implementation likely requires new Go module dependencies that are not yet added.

## Remediation Ideas
- Move shared helpers into a common file or namespace to avoid redeclarations.
- Add `tcell` and `tview` modules if the tview path is meant to compile, or gate the tview build with build tags.

## Resolution (This Run)
- None; left unchanged due to scope (share page frontend fix only).
