# 2026-01-26 - tui macos input failure

- Summary: tview-based TUI failed to accept input on macOS during interactive mode.
- Remediation: replaced tview with gocui for the interactive TUI and retained line-mode fallback.
