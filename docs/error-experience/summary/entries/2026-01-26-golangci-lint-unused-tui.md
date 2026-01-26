# 2026-01-26 - golangci-lint reports unused shouldForceLineInput

- Summary: `./dev.sh lint` failed because `cmd/alex/tui.go:60:6` reports `shouldForceLineInput` as unused.
- Remediation: delete the dead helper or wire it into the TUI path, then rerun lint.
- Resolution: not resolved in this run.
