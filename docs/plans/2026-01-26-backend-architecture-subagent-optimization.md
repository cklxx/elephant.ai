# Plan: Backend architecture optimization via subagents (2026-01-26)

## Goal
- Implement the identified backend architecture optimizations in small, reviewable batches using subagent analysis support.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope (Phased)
1. Presentation extraction
   - Move ANSI/tool display formatting out of domain into output/presentation.
2. Delivery façade
   - Introduce `server/app` façade (tool metadata + event formatting) so HTTP handlers stop importing builtins/domain formatter.
3. Builtin tools split
   - Split `internal/tools/builtin` into per-domain subpackages + thin registry wiring.
4. Large-file decomposition
   - Incrementally split large files (`cli_renderer.go`, `middleware.go`, `sse_handler_render.go`, `openai_responses_client.go`) by responsibility.

## Execution Approach
- Use subagents for focused analysis per phase; implement changes in small commits with full tests each batch.
- TDD when touching logic; refactors aimed to preserve behavior.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Phase 1 started — moved tool formatter out of domain into `internal/presentation/formatter`, updated imports.
- 2026-01-26: Phase 1 validated with `make fmt`, `make vet`, `make test`.
- 2026-01-26: Phase 2 started — SSE handler now uses `agent.SubtaskWrapper` to remove builtin dependency.
- 2026-01-26: Phase 2 validated with `make fmt`, `make vet`, `make test` (one transient test failure; recorded in error-experience).
- 2026-01-26: Phase 3 started — extracted builtin helpers into `internal/tools/builtin/shared` + `internal/tools/builtin/pathutil`, updated usages and toolregistry/cmd wiring.
- 2026-01-26: Phase 3 validated with `make fmt`, `make vet`, `make test`.
- 2026-01-26: Phase 3 continued — moved builtin tools into domain subpackages and rewired registry/callers to use the new packages (shims removed).
- 2026-01-26: Phase 3 consolidated per-package move notes into the main builtin split plan; removed redundant plan files.
