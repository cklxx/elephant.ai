# 2026-01-26 - dev.sh test blocked by MCP data race

## Error
- `./dev.sh test` failed with Go race detector output in `internal/mcp`: `TestProcessManagerReinitializesStopChan` reports data races between `ProcessManager.Start()` and `monitorExit/monitorStderr`.

## Impact
- Full test run fails under `-race`, blocking engineering-practices compliance.

## Notes / Suspected Causes
- Concurrent access to `ProcessManager` fields (`stopCh`, `stderrDone`/`exitDone`) is not synchronized.

## Remediation Ideas
- Guard shared fields with a mutex or rework goroutine startup so monitors capture immutable copies of the channels.

## Resolution (This Run)
- Not resolved; left unchanged (out of scope for share page fix).
