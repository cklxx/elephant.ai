## Goal

Let the leader see sibling progress like `2/5` when handling `EventChildCompleted`, without changing the event type or breaking callers that omit the new payload fields.

## Scope

- Add `sibling_total` and `sibling_completed` to `EventChildCompleted` payloads in runtime publish paths.
- Extend `handleChildCompleted` prompt generation to include progress and the "all complete" summary hint.
- Add runtime and leader tests for the new payload and prompt behavior.

## Plan

1. Inspect the only `EventChildCompleted` publish path and add a runtime helper that computes sibling progress from existing session state.
2. Update leader prompt construction to read optional payload counts and degrade cleanly when absent.
3. Add focused tests in `internal/runtime/runtime_test.go` and `internal/runtime/leader/leader_test.go`.
4. Run `go test ./internal/runtime/...`, then code review, commit, fast-forward merge to `main`, and clean up the worktree.
