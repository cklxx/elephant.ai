# 2026-02-25 Rate Limit Fallback Hardening

## Context
- User reports Lark path gets rate-limited and asks whether subscription model is used.
- Existing behavior should gracefully degrade when pinned subscription model is exhausted.

## Plan
1. Reconfirm root cause from runtime evidence and selection source.
2. Reintroduce runtime fallback in main execution path for pinned model 429 / usage_limit_reached.
3. Add equivalent fallback in memory-capture hook to avoid repeated post-task failures.
4. Add focused unit tests for both paths.
5. Run package tests and representative E2E Lark inject verification.
6. Summarize prevention mechanisms and operational guardrails.

## Progress
- [x] Plan file created.
- [x] Root cause reconfirmed.
- [x] Execution-path fallback implemented.
- [x] Memory-capture fallback implemented.
- [x] Tests green.
- [x] E2E + logs verified (targeted suites + session/log grep).
