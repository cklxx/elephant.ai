# Plan: Deduplicate Lark message handling (2026-01-29)

- Reviewed `docs/guides/engineering-practices.md`.

## Goals
- Prevent duplicate Lark replies caused by repeated message delivery.
- Keep behavior stable for group vs. P2P replies.
- Add regression coverage for message dedupe TTL.

## Plan
1. Add an in-memory message-id deduper to the Lark gateway with TTL + LRU.
2. Wire dedupe check early in `handleMessage`.
3. Add tests for dedupe behavior (repeat + expiry).
4. Run `./dev.sh lint` and `./dev.sh test`.
5. Update plan progress and commit.
- 2026-01-29: Added message-id dedupe (LRU+TTL) in Lark gateway and tests for repeat/expiry.
- 2026-01-29: Ran `./dev.sh lint` and `./dev.sh test` (pass; linker warnings about LC_DYSYMTAB observed during Go tests).
