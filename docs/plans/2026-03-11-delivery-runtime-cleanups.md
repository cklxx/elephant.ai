## Goal

Audit `internal/delivery/` and `internal/runtime/` for three cleanup classes:

- unused exported functions that can be unexported or removed safely
- redundant type assertions
- error comparisons that should use `errors.Is` / `errors.As`

## Scope

- Limit changes to `internal/delivery/**` and `internal/runtime/**`
- Only apply cleanups that are behavior-preserving and supported by existing or added tests
- Avoid API redesign outside the touched packages

## Approach

1. Scan exported identifiers and verify whether they have cross-package callers.
2. Scan for direct error equality / string matching and inspect whether sentinel or typed errors already exist.
3. Scan for unnecessary assertions where the concrete type is already known or where `errors.As` is the correct form.
4. Land a small, safe batch of fixes with focused validation.

## Validation

- `go test ./internal/delivery/... ./internal/runtime/...`
- package-targeted lint for touched packages
- `python3 skills/code-review/run.py review`
