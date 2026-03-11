# Infra Export Prune

## Scope

- Audit exported functions and types under `internal/infra/`.
- Confirm package-external references before deletion.
- Remove dead exported surface with minimal code changes.
- Validate affected packages and merge back to `main`.

## Plan

1. Enumerate exported symbols in `internal/infra/**`.
2. Find package-external references and shortlist dead exports.
3. Remove confirmed dead code and update tests if needed.
4. Run targeted validation, review, commit, and fast-forward merge.

## Findings

- `internal/infra/lark/approval.go` was a fully dead service surface: all exported types and methods were referenced only by same-package tests.
- `internal/infra/lark/calendar/meetingprep`, `internal/infra/lark/calendar/suggestions`, and `internal/infra/lark/summary` had no code imports anywhere in the repo.
- `internal/infra/lark/calendar_conflict.go`, `internal/infra/skills/custom.go`, and `internal/infra/tools/builtin/shared/validation.go` were also test-only export surfaces.
- In still-live packages, `coding.NewAdapterRegistry`, the verification helper surface, `teamruntime.SelectRoleBinding`, and `bridge.ClassifyOrphan` were exported even though only package-internal callers used them.

## Validation

- `go test ./internal/infra/...`
- `go test ./internal/app/... ./internal/delivery/server/app`
- `go test ./...`
- `python3 skills/code-review/run.py review`
