# Plan: Sync Roadmap with Code + Runtime Snapshot (2026-02-08)

Status: Completed

## Goal
- Correct roadmap statements to match current repository architecture and verified runtime state.
- Remove/adjust stale `Done` claims that are not backed by code.
- Keep `docs/roadmap/roadmap.md` as authoritative source and avoid introducing speculative status.

## Scope
- Update `docs/roadmap/roadmap.md`.
- Update `docs/roadmap/roadmap-pending-2026-02-08.md` only if needed for consistency.
- Add dated runtime snapshot based on current command results.

## Steps
1. Capture runtime evidence (`./dev.sh status`, `./lark.sh status`) with concrete timestamp.
2. Audit roadmap code paths against current layered architecture (`delivery/app/domain/infra/shared`).
3. Fix stale paths and incorrect status claims (especially `Done` items without implementation).
4. Validate formatting and perform a quick path sanity check.
5. Run full lint + test before delivery.

## Progress Log
- 2026-02-08 13:14 +0800: Captured runtime snapshot from `./dev.sh status` and `./lark.sh status`.
- 2026-02-08 13:15 +0800: Completed mismatch scan for stale paths and `Done` claims.
- 2026-02-08 13:18 +0800: Updated `docs/roadmap/roadmap.md` to align package paths with current layering and corrected Decision/Entity memory status to `Not started`.
- 2026-02-08 13:20 +0800: Ran `./dev.sh lint` and `./dev.sh test`.
  - `lint` failed on pre-existing baseline issues (`errcheck/unused/staticcheck`) in `internal/devops/*` and `cmd/alex/*`.
  - `test` failed on pre-existing baseline issues:
    - race in `alex/internal/delivery/server/bootstrap` (`TestRunLark_FailsWhenLarkDisabled`, `TestRunLark_FailsWhenCredentialsMissing`)
    - env guard failure in `alex/internal/shared/config` (`TestNoUnapprovedGetenv`).
