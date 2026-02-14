# 2026-02-14 — Lark coding agent completion notify in trigger conversation

## Goal
- Ensure coding-agent background task completion is always notified in the same Lark conversation that triggered the task, even after the foreground turn has already returned.

## Context
- Existing listener model supports two shutdown modes:
  - `Close()`: immediate shutdown.
  - `Release()`: foreground released; keep listener alive until all tracked background tasks complete.
- `/task` direct-dispatch path already uses `Release()`, but normal conversation path still used `Close()`, which can drop late completion notifications.

## Best-practice references
- Event-driven async handling should prefer graceful lifecycle release over hard shutdown when background work remains (reliability-first completion delivery).
- Regression tests should cover lifecycle boundaries (foreground return vs background completion) to prevent notification loss regressions.

## Plan
1. [x] Locate and confirm the lifecycle mismatch between direct-dispatch and normal conversation paths.
2. [x] Update normal conversation background listener cleanup to use release semantics.
3. [x] Add regression test: foreground returns first, background completion arrives later, completion is still notified in triggering conversation.
4. [x] Run lint + full tests.
5. [x] Run mandatory code review workflow and fix findings.
6. [x] Commit incrementally, merge worktree branch back to `main`, and clean up worktree.

## Progress Log
- 2026-02-14 12:58 +0800: Confirmed root cause in `setupListeners`: normal path uses `bgLn.Close`, which can stop post-return completion notifications.
- 2026-02-14 13:03 +0800: Switched `setupListeners` background listener cleanup to `Release` and added delayed-completion regression test.
- 2026-02-14 13:10 +0800: Validation done — targeted regression test passes, `go test ./...` passes; lint run reports pre-existing unrelated issues (`cookie_token.go`, `file_event_history_store.go`) in base branch.
- 2026-02-14 13:12 +0800: Completed code-review checklist pass (SOLID/security/quality/removal); no new P0-P3 findings on this diff.
- 2026-02-14 13:16 +0800: Rebased branch onto latest `main`, fast-forward merged, removed temporary worktree.
