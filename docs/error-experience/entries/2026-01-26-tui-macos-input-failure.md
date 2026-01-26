Error: macOS could not enter the interactive TUI when using the tview-based implementation (input focus/entry failed).
Remediation: switched the interactive TUI from tview to gocui with plain rendering and kept line-mode fallback.
