# Plan: Lark reply vs proactive messages

Owner: cklxx
Date: 2026-02-01

## Goal
Ensure Lark responses reply to inbound messages while proactive sends avoid reply threading.

## Steps
1. Review current Lark gateway send/reply flow and replyTarget usage. DONE
2. Add/adjust tests to cover replyTarget behavior for allowed/disallowed replies. DONE
3. Update replyTarget/dispatch logic to reply when allowed and message ID is available. DONE
4. Ensure proactive sends (progress updates) bypass reply threading. DONE
5. Run full lint + tests and note any follow-ups. BLOCKED
   - `./dev.sh lint` fails on existing typecheck errors in `cmd/alex` and unused imports in `internal/toolregistry/registry.go`.
   - `./dev.sh test` was started but interrupted (user requested to proceed with commits).
