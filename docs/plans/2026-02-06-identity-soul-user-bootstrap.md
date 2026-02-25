# Plan: Identity Bootstrapping for SOUL.md and USER.md (2026-02-06)

## Goal
- Align runtime identity boot with `SOUL.md` / `USER.md` requirements.
- Map `SOUL.md` to the active persona source (`configs/context/personas/default.yaml` for default persona).
- Auto-create missing `USER.md` under memory storage and make prompt paths explicit.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.
- Loaded recent memory/error/good summaries and `docs/memory/long-term.md` active memory.

## Scope
1. Update context memory snapshot assembly to include identity files first.
2. Add bootstrap logic to auto-create missing `SOUL.md` and `USER.md`.
3. Update identity prompt wording to explicitly state file locations.
4. Update memory-system reference doc for path + auto-create behavior.
5. Add/adjust tests (TDD) for snapshot and prompt behavior.
6. Run full lint and tests before delivery.

## Progress
- 2026-02-06 13:45: Created worktree branch `eli/identity-soul-user-bootstrap` from `main` and copied `.env`.
- 2026-02-06 13:46: Reviewed engineering practices and memory summaries/long-term memory.
- 2026-02-06 13:47: Located implementation points: `internal/app/context/manager_memory.go`, `internal/app/context/manager_prompt.go`, `docs/reference/MEMORY_SYSTEM.md`.
- 2026-02-06 13:52: Added TDD coverage for identity bootstrap (`SOUL.md`/`USER.md`) and identity path hints in prompt section.
- 2026-02-06 13:54: Implemented identity bootstrap logic in memory snapshot loading: auto-create missing `SOUL.md` and `USER.md`, then inject sections in boot order.
- 2026-02-06 13:58: Optimized bootstrap path to avoid re-parsing persona config when `SOUL.md` already exists.
- 2026-02-06 13:54: Updated identity prompt section with explicit `SOUL.md` / `USER.md` locations and fallback behavior.
- 2026-02-06 13:55: Updated `docs/reference/MEMORY_SYSTEM.md` session-boot instructions with explicit paths + auto-create rules.
- 2026-02-06 13:56: Updated `docs/memory/long-term.md` `Updated:` timestamp to hour precision for first load of the day.
- 2026-02-06 13:57: Ran full validation pipeline (`make fmt`, `make vet`, `make test`) successfully.

## Validation
- `go test ./internal/app/context -count=1`
- `make fmt`
- `make vet`
- `make test`
