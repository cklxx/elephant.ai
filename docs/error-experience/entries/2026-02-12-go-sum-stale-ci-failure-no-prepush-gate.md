# go.sum stale after dependency removal — CI failure, no pre-push gate

**Date:** 2026-02-12
**Severity:** Medium
**Category:** CI / Developer Experience

## What happened

Pushed code to `main` that removed `sqlite-vec-go-bindings` from `go.mod` but did not run `go mod tidy` before pushing. The CI `security` job's "Verify module tidiness" step runs:

```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

This caught the stale `go.sum` entry and failed the pipeline. The web and lint jobs also failed independently.

## Root cause

1. **No local pre-push gate** — the `.git/hooks/pre-push` only ran `git lfs pre-push`, with zero CI parity checks.
2. **Manual discipline gap** — relying on developers to remember `go mod tidy` after dependency changes is fragile.

## Fix

1. Created `scripts/pre-push.sh` — a local CI gate that mirrors critical CI checks:
   - `go mod tidy` + `git diff --exit-code go.mod go.sum`
   - `go vet`
   - `go build` (all binaries)
   - `golangci-lint`
   - Architecture boundary checks
   - Web lint + build (only when `web/` files changed)
2. Updated `.git/hooks/pre-push` to call the script.
3. Added `make ci-local` target for manual runs.
4. Skip mechanism: `SKIP_PRE_PUSH=1 git push` for emergencies.

## Lesson

- **Pre-push hooks should mirror CI's fast-fail checks.** The cost of a 30-second local check is far less than a failed CI pipeline + context switch.
- Always run `go mod tidy` after any dependency change; automate this check rather than relying on memory.
- Smart change detection (Go vs web files) keeps the hook fast for partial changes.

## Prevention checklist

- [x] Pre-push hook installed
- [x] `make ci-local` for manual verification
- [x] Hook is skippable for emergencies
- [x] Hook detects changed file types and skips irrelevant checks
