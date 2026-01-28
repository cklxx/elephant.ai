# Plan: LLM request/response JSON log

Date: 2026-01-28
Owner: Codex

## Goal
Replace the unreadable streaming request log with a JSONL file that captures LLM request/response payloads, and ensure streaming responses only log the final aggregated payload.

## Plan
1. Update request log writer to emit JSONL entries and switch to a new log file name (stop writing `logs/requests/streaming.log`).
2. Ensure streaming LLM paths only log the final aggregated response payload, then remove summary-only request log entries.
3. Update log fetcher, docs, and tests to match the new JSONL format and file path.
4. Run full lint + test suite and commit the changes.

## Progress
- 2026-01-28: Plan created.
- 2026-01-28: Switched request logs to JSONL (`logs/requests/llm.jsonl`), removed streaming summaries, and ensured streaming responses log only final aggregated payloads.
- 2026-01-28: Updated log fetcher, tests, and log docs to match JSONL format.
- 2026-01-28: Ran `./dev.sh lint` and `./dev.sh test` (tests pass with existing ld warnings on macOS toolchain).
