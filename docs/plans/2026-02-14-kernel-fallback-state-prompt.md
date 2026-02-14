# Plan: Kernel Fallback Persistence for State/System Prompt

Owner: cklxx
Date: 2026-02-14

## Context
Kernel state artifacts (`STATE.md`, `SYSTEM_PROMPT.md`) can fail to write under sandbox/permission restrictions. We need a fallback path in the repo artifacts to persist updated content and clear logs without changing primary behavior.

## Goals
- On sandbox/permission write failures for `STATE.md` or `SYSTEM_PROMPT.md`, also persist to `/Users/bytedance/code/elephant.ai/artifacts/kernel_state.md`.
- Emit clear logs when fallback is used.
- Keep primary write behavior and error returns unchanged.
- Add/adjust tests where reasonable.

## Plan
1. Inspect current kernel state/prompt persistence and fallback paths. (done)
2. Implement fallback handling for permission-restricted writes to `STATE.md` and `SYSTEM_PROMPT.md`.
3. Add/adjust tests covering fallback behavior and logging expectations.
4. Run full lint + test suite.
5. Code review, commit (incremental), and merge back to `main`.

## Status
- [x] Step 1: Inspect current kernel state/prompt persistence and fallback paths.
- [x] Step 2: Implement fallback handling for permission-restricted writes to `STATE.md` and `SYSTEM_PROMPT.md`.
- [x] Step 3: Add/adjust tests covering fallback behavior and logging expectations.
- [ ] Step 4: Run full lint + test suite.
- [ ] Step 5: Code review, commit (incremental), and merge back to `main`.

## Notes
- Step 4 attempted via `./scripts/pre-push.sh`; `go test -race` failed due to LLM profile config mismatch (openai provider + `sk-kimi-*` key) and a config admin path expectation.
