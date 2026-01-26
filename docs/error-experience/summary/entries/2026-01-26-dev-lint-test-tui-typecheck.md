# 2026-01-26 - dev.sh lint/test blocked by TUI typecheck errors

- Summary: `./dev.sh lint` failed with redeclared TUI symbols in `cmd/alex`, and `./dev.sh test` failed because `tcell`/`tview` modules are missing.
- Remediation: deduplicate shared TUI helpers or gate tview with build tags; add required Go modules if tview is intended to build.
- Resolution: none in this run (out of scope).
