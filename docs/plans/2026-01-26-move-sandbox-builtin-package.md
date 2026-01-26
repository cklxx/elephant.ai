# Plan: Move sandbox builtin tools into sandbox package

## Goal
Move sandbox builtin tool implementations into a dedicated `internal/tools/builtin/sandbox` package, update imports/packages accordingly, and keep registry wiring unchanged.

## Steps
- [x] Inventory current sandbox tool files and call sites; confirm registry wiring references.
- [ ] Move sandbox files into `internal/tools/builtin/sandbox`, update `package sandbox`, and adjust imports/usages (add shim if needed to avoid registry changes).
- [ ] Run full lint and test suite; capture results.

## Progress
- 2026-01-26: Plan created.
- 2026-01-26: Located sandbox files, registry references in `internal/toolregistry/registry.go`, and helper dependency in `internal/tools/builtin/artifacts/attachment_uploader.go`.
