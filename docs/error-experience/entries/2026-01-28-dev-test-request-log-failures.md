# 2026-01-28 - dev.sh test blocked by request log expectations

## Error
- `./dev.sh test` failed in `internal/tools/builtin/web` with `web_fetch_test.go:68: failed to read request log: .../llm.jsonl: no such file or directory`.
- `./dev.sh test` failed in `internal/tools/builtin/media` with `seedream_test.go:515: failed to read request log: .../streaming.log: no such file or directory`.
- Go linker warnings about malformed `LC_DYSYMTAB` also appeared, but tests continued.

## Impact
- Full test run failed; changes cannot be fully validated.

## Notes / Suspected Causes
- Request logging may be asynchronous or writing to a different path than the tests expect.
- `ALEX_REQUEST_LOG_DIR` is set in tests, but log writes might be skipped or delayed by dedupe/queueing.

## Remediation Ideas
- Align tests with actual log path/format (e.g., `logs/requests/llm.jsonl` if applicable).
- Ensure request log writes are flushed before test assertions (use `WaitForRequestLogQueueDrain`).
