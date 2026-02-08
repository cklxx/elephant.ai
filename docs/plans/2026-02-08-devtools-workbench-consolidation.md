# Plan: Dev Tools Workbench Consolidation and Large-Log Performance

## Summary

Consolidate duplicated Dev Tools into three workbench routes:

- `/dev/diagnostics` (conversation debugger + structured log analyzer)
- `/dev/configuration` (runtime/apps/context tooling)
- `/dev/operations` (evaluation/sessions/plan operations)

Then optimize the diagnostics log UI for high-volume data by reducing render cost,
using virtualization, and deferring expensive payload work.

## Scope

- Web frontend route consolidation under `web/app/dev/`
- Shared dev-tools UI primitives under `web/components/dev-tools/`
- `dev.sh` Dev Tools shortcuts update
- Documentation and experience record updates

Out of scope:

- Backend API schema changes
- Non-dev business pages redesign

## Checklist

- [x] Create implementation plan and checkpoints
- [x] Add shared perf-oriented UI primitives and tests
- [x] Build structured log workbench component (virtualized + deferred payload rendering)
- [x] Build diagnostics workbench route (aggregate SSE debugger and structured logs)
- [x] Build configuration and operations workbench routes
- [x] Update dev home navigation + `dev.sh` links
- [x] Remove duplicated legacy route surfaces (or make them non-entry)
- [x] Run full lint and test suite
- [x] Add good-experience and summary records
- [x] Update long-term memory timestamp and active notes
- [ ] Commit incrementally and merge back to `main`

## Progress Notes

- 2026-02-08 16:40: Implemented diagnostics/configuration/operations workbench routes and updated `/dev` to three consolidated entries.
- 2026-02-08 16:40: Added virtualized event stream rendering and structured log workbench with deferred payload expansion.
- 2026-02-08 16:40: Updated `dev.sh logs-ui` readiness + open URL to `/dev/diagnostics`.
- 2026-02-08 16:40: Validation ran: `web` lint/test passed; root `make vet` + `make check-arch` passed; `make fmt` and `make test` are currently blocked by pre-existing repo issues (errcheck violations and env usage guard failure in unrelated files).

## Acceptance Criteria

- `/dev` only shows the three workbench entries.
- `/dev/diagnostics` provides both SSE/session debugging and structured log browsing.
- Large log data remains responsive (virtualized lists, no eager full payload stringify on hidden rows).
- `./dev.sh logs-ui` opens diagnostics workbench entry instead of old log-analyzer route.
- Lint/tests pass before final merge.
