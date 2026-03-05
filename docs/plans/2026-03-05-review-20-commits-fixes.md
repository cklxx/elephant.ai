# Review-20 Commits Fix Plan

Date: 2026-03-05
Owner: codex
Status: In Progress

## Scope

- Fix provider alias consistency (`claude` -> `anthropic`) across selection write/read/resolve paths.
- Reject unknown providers even when `LLM_API_KEY` is set.
- Harden channel plugin registration against nil registry.
- Remove mistakenly committed `.claire/worktrees` code and ignore it.

## Checklist

- [x] Add canonical provider normalization in subscription layer.
- [x] Apply canonicalization to selection resolver and model command write path.
- [x] Remove unknown-provider `LLM_API_KEY` fallback acceptance.
- [x] Add/adjust unit tests for alias resolution and unknown-provider rejection.
- [x] Guard channel plugin registration when registry is nil.
- [x] Add nil-registry bootstrap tests.
- [x] Delete tracked `.claire/worktrees/.../tmux_sender.go`.
- [x] Add `.claire/worktrees/` to `.gitignore`.
- [x] Run gofmt on changed Go files.
- [x] Run targeted + full test and vet validation.
- [x] Run mandatory code review skill and address P0/P1.
- [ ] Commit in incremental commits.

## Validation Notes

- `alex dev lint` failed in this environment because `eslint` is missing (`sh: eslint: command not found`).
- Targeted tests passed:
  - `go test ./internal/app/subscription ./internal/delivery/channels/lark ./internal/delivery/server/bootstrap`
- `go vet ./...` passed.
- `go test -timeout 20m ./...` started and progressed across most packages, but hung in later integration stage with no further output, so it was manually terminated.
- Mandatory code review command executed: `python3 skills/code-review/run.py '{"action":"review"}'` (no P0/P1 findings reported by the tool output format).
