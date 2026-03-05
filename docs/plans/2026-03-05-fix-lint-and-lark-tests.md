# 2026-03-05 Fix Lint and Lark Tests

## Goal
- Resolve the reported `alex dev lint` failure (`eslint` not found) with a robust fix.
- Resolve current `alex dev test` failures in Lark model command tests.
- Verify with full lint + tests, then merge and push.

## Plan
1. Reproduce failures in clean worktree and capture exact failing commands.
2. Fix lint command/toolchain path issue in project scripts (prefer deterministic invocation).
3. Fix brittle Lark tests so they are deterministic and isolated from host env.
4. Run full `alex dev lint` and `alex dev test` to green.
5. Run mandatory code review script, commit, merge to `main`, push.

## Progress
- [x] Plan created.
- [x] Failure reproduction complete.
- [x] Fixes implemented.
- [ ] Full validation passed. (`go run ./cmd/alex dev lint` passed; `go run ./cmd/alex dev test` still fails in `internal/infra/integration` E2E bridge suite)
- [ ] Merged and pushed.
