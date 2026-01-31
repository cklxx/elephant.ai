# Engineering Practices

These are the local engineering practices for this repo. Keep them short and actionable.

## Core
- Prefer correctness and maintainability over short-term speed.
- Make small, reviewable changes; avoid large rewrites unless explicitly needed.
- Use TDD when touching logic; include edge cases.
- Run full lint and tests before delivery.
- On macOS, prefer `CGO_ENABLED=0` for Go tests with `-race` to avoid LC_DYSYMTAB linker warnings; set `CGO_ENABLED=1` when cgo is required.
- Keep config examples in YAML only (no JSON configs).

## Planning & Records
- Every non-trivial task must have a plan file under `docs/plans/` and be updated as work progresses.
- Continuously review best practices and execution flow; record improvements and update guides/plans when new patterns emerge.
- Log notable incidents in `docs/error-experience/entries/` and add a summary entry under `docs/error-experience/summary/entries/`.
- Log notable wins in `docs/good-experience/entries/` and add a summary entry under `docs/good-experience/summary/entries/`.
- Keep `docs/error-experience.md` and `docs/error-experience/summary.md` as index-only.
- Keep `docs/good-experience.md` and `docs/good-experience/summary.md` as index-only.

## Safety
- Avoid destructive operations or history rewrites unless explicitly requested.
- Prefer reversible steps and explain risks when needed.

## Code Style
- Avoid unnecessary defensive code; trust invariants when guaranteed.
- Keep naming consistent; follow local naming guidelines when present.
- Be cautious with long parameter lists; if a function needs many inputs, prefer grouping into a struct or options pattern and document the boundary explicitly.

## Go + OSS (Condensed)
- Formatting/imports: always run `gofmt`; use `goimports` to manage imports.
- Naming: package names are lowercase and avoid underscores/dashes; avoid redundant interface names (`storage.Interface` not `storage.StorageInterface`).
- Comments: exported identifiers have full-sentence doc comments that start with the identifier name.
- Context: pass `context.Context` explicitly (first param); never store it in structs.
- Errors: check/handle errors; avoid `panic` for normal flow; wrap with context when returning.
- Concurrency & tests: avoid fire-and-forget goroutines (make lifetimes explicit); prefer table-driven tests for multi-case coverage.

Sources: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Kubernetes Coding Conventions](https://www.kubernetes.dev/docs/guide/coding-convention/).
