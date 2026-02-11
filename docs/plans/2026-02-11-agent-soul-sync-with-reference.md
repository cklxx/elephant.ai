# Plan: Sync Agent SOUL with `docs/reference/SOUL.md` (2026-02-11)

## Goal
- Align the runtime agent SOUL/persona baseline with the expression in `docs/reference/SOUL.md`.
- Ensure bootstrap-generated `~/.alex/memory/SOUL.md` reflects the same SOUL content.
- Keep prompt identity metadata consistent and validated by tests.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.
- Loaded recent error/good summaries and `docs/memory/long-term.md` active memory.
- Created worktree `feat/agent-soul-sync-20260211` from `main` and copied `.env`.

## Scope
1. Locate current SOUL source(s) and prompt assembly path.
2. Port the reference SOUL content into canonical persona/identity configuration.
3. Update SOUL bootstrap template generation to output the new SOUL document.
4. Update/add tests for the new behavior (TDD for changed logic).
5. Run full lint + tests.
6. Perform mandatory code review report before commit.

## Progress
- 2026-02-11 11:27: Confirmed implementation points (`configs/context/personas/default.yaml`, `internal/app/context/manager_memory.go`, `internal/app/context/manager_prompt.go`).
- 2026-02-11 11:27: Found `docs/reference/SOUL.md` exists in main workspace but is not yet tracked in this new worktree; using it as canonical source for this task.
- 2026-02-11 11:31: Added `docs/reference/SOUL.md` into this worktree as a tracked reference source.
- 2026-02-11 11:31: Synced `configs/context/personas/default.yaml` to the full SOUL expression from `docs/reference/SOUL.md`.
- 2026-02-11 11:33: Updated SOUL bootstrap rendering (`renderSoulTemplate`) to emit persona `voice` verbatim.
- 2026-02-11 11:34: Updated context-memory tests and added `TestRenderSoulTemplateUsesPersonaVoiceVerbatim`.
- 2026-02-11 11:36: Ran focused validation `go test ./internal/app/context -count=1` (pass).
- 2026-02-11 11:40: Ran full validation pipeline (`make fmt`, `make vet`, `make test`) successfully.
- 2026-02-11 11:43: Completed mandatory code review and wrote report to `docs/reviews/2026-02-11-agent-soul-sync-review.md`.
- 2026-02-11 11:49: Re-ran full validation after final doc updates; observed known flaky failure in `TestAsyncFlushCoalescesRequests` (`internal/delivery/server/app`) on some runs, then obtained green full run on re-run.

## Validation
- `go test ./internal/app/context -count=1`
- `make fmt`
- `make vet`
- `make test`
- `go test ./internal/delivery/server/app -run TestAsyncFlushCoalescesRequests -count=20 -v` (flake characterization)
