## Goal

Audit all Go source comments for stale `TODO` / `FIXME` / `HACK` markers, delete resolved ones, and track still-valid follow-ups outside code comments.

## Audit Result

- 2026-03-11 comment-only scans across `*.go` returned no remaining `TODO` / `FIXME` / `HACK` markers in Go comments.
- The remaining valid follow-ups below are tracked here rather than as inline Go comment markers.
- The snapshot CLI still emits a runtime gap notice, but it is not a source comment marker.

## Follow-ups

### A. CLI snapshot rendering

- Current location: `cmd/alex/cli_sessions.go`
- Render structured `plans`, `world_state`, and `diff` content in `alex session snapshot` output instead of only counts/key lists.
- Keep raw JSON output unchanged.
- Verify against snapshots produced by `internal/domain/agent/react/context.go` and `internal/infra/session/state_store`.

### B. Jira work-item provider

- Current location: `internal/infra/workitems/jira/provider.go`
- Replace the Jira REST stub.
- Implement issue list, single issue fetch, comments, status changes, and accessible workspace discovery.
- Add contract tests beyond current stub-behavior coverage.

### C. Linear work-item provider

- Current location: `internal/infra/workitems/linear/provider.go`
- Replace the Linear GraphQL stub.
- Implement issue list, single issue fetch, comments, status-change synthesis, and workspace resolution.
- Add contract tests beyond current stub-behavior coverage.

### D. Lark calendar provider

- Current location: `internal/infra/calendar/lark_calendar_provider.go`
- Replace the stub with real token acquisition and event listing.
- Preserve current credential validation behavior.
- Add integration coverage around 1:1 filtering and mapping to `domain.Meeting`.

## Outcome

- All `TODO` / `FIXME` / `HACK` markers are removed from Go source comments.
- Detailed inline stub implementation notes were removed from the audited Go files.
- Remaining valid work is tracked here instead of being left as inline code markers.
