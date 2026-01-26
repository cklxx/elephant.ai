# 2026-01-26 - dev.sh test blocked by MCP data race

- Summary: `./dev.sh test` failed with race detector errors in `internal/mcp` (`TestProcessManagerReinitializesStopChan`).
- Remediation: synchronize access to `ProcessManager` fields or pass immutable channel references into goroutines.
- Resolution: none in this run (out of scope).
