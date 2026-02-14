# Plan: Kernel State Fallback Persistence

Owner: cklxx
Date: 2026-02-14

## Context
Kernel state writes can fail under sandbox path restrictions (e.g., `~/.alex/kernel`). We need a deterministic fallback persistence path inside the repo workspace and clear observability when fallback is used.

## Goals
- Persist updated `STATE.md` content to `/Users/bytedance/code/elephant.ai/artifacts/kernel_state.md` when state writes are blocked by sandbox restrictions.
- Avoid repeated writes to restricted paths once detected.
- Emit a clear fallback note (logs/runtime block).
- Keep behavior aligned with atomic-write and reliability best practices (POSIX tmp+rename, fail-soft persistence fallback, error classification).

## Plan
1. **Inspect current kernel state persistence flow** (state file + engine fallback logic).
2. **Add restricted-write detection + disable further writes** to avoid repeated sandbox failures.
3. **Update fallback path + runtime/log notes** to the required artifacts location.
4. **Update/extend tests** for fallback path and restricted-write behavior.
5. **Run lint + tests, review, commit, and merge.**

## Status
- [x] Step 1: Inspect current kernel state persistence flow.
- [x] Step 2: Add restricted-write detection + disable further writes.
- [x] Step 3: Update fallback path + runtime/log notes.
- [x] Step 4: Update/extend tests.
- [ ] Step 5: Run lint + tests, review, commit, and merge.
