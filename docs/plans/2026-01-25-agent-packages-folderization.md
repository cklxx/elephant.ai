# Plan: Folderize agent app/domain by prefix (2026-01-25)

## Goal
- Group `internal/agent/app` and `internal/agent/domain` files by prefix into subpackages for readability without compatibility shims.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. **App layer**: split into subpackages (context, preparation, cost, coordinator) and move tests accordingly.
2. **Domain layer**: move `react_engine_*` + `react_runtime*` into a `domain/react` subpackage; adjust imports/tests.
3. Update all imports across repo.
4. Run `make fmt`, `make vet`, `make test` after each stage.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Split app layer into subpackages (coordinator/preparation/context/cost/config), updated imports, and renamed prep/cost/context files to drop redundant prefixes.
- 2026-01-25: Moved ReAct engine into `internal/agent/domain/react`, added shared type aliases, updated event references/imports, and renamed files to remove `react_engine_*` prefixes.
- 2026-01-25: Moved tool formatter to `internal/agent/domain/formatter`, updated renderers/SSE handler imports, and refreshed docs/Makefile references.
- 2026-01-25: Ran `make fmt`, `make vet`, `make test`.
