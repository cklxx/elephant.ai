status: completed

## Scope

- Audit `internal/shared/**` exported API surface.
- Lower or remove exports that are unused outside their own package.
- Check `internal/shared/utils` for trivially mergeable helpers.
- Validate with `go vet` and focused `go test`, then commit.

## Plan

1. Scan low-risk shared packages and confirm package-local-only exports.
2. Lower dead or package-private exports; merge tiny duplicated helpers in `utils`.
3. Run `go vet` and focused `go test`, then commit.

## Findings

- Lowered `modelregistry.Registry` to package-local `registry`; external callers already used package-level lookup helpers only.
- Lowered test-only `utils.WaitForRequestLogQueueDrain` to package-local helper and kept tests on the same package seam.
- Merged the tiny whitespace helper overlap in `utils` by making `HasContent` delegate to `IsBlank`.

## Validation

- `go vet ./internal/shared/... ./internal/app/... ./internal/infra/... ./internal/delivery/... ./cmd/alex`
- `go test ./internal/shared/... ./internal/app/... ./internal/infra/... ./internal/delivery/... ./cmd/alex`
- `python3 skills/code-review/run.py review`
