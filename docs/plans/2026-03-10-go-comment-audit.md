## Goal

Audit all Go source comments for stale `TODO` / `FIXME` / `HACK` markers, delete resolved ones, and track still-valid follow-ups outside code comments.

## Inventory

1. `cmd/alex/cli_sessions.go:200`
   - Original: surface structured diff/plan output in snapshot CLI.
   - Status: still valid, but the note claiming runtime does not populate the fields is outdated.
   - Tracking: follow-up A below.
2. `internal/infra/workitems/jira/provider.go:44`
   - Original: implement Jira issue listing.
   - Status: still valid stub.
   - Tracking: follow-up B below.
3. `internal/infra/workitems/jira/provider.go:56`
   - Original: implement Jira single-item fetch.
   - Status: still valid stub.
   - Tracking: follow-up B below.
4. `internal/infra/workitems/jira/provider.go:66`
   - Original: implement Jira comment listing.
   - Status: still valid stub.
   - Tracking: follow-up B below.
5. `internal/infra/workitems/jira/provider.go:75`
   - Original: implement Jira status-change listing.
   - Status: still valid stub.
   - Tracking: follow-up B below.
6. `internal/infra/workitems/jira/provider.go:85`
   - Original: implement Jira workspace resolution.
   - Status: still valid stub.
   - Tracking: follow-up B below.
7. `internal/infra/workitems/linear/provider.go:42`
   - Original: implement Linear issue listing.
   - Status: still valid stub.
   - Tracking: follow-up C below.
8. `internal/infra/workitems/linear/provider.go:52`
   - Original: implement Linear single-item fetch.
   - Status: still valid stub.
   - Tracking: follow-up C below.
9. `internal/infra/workitems/linear/provider.go:61`
   - Original: implement Linear comment listing.
   - Status: still valid stub.
   - Tracking: follow-up C below.
10. `internal/infra/workitems/linear/provider.go:72`
    - Original: implement Linear status-change synthesis.
    - Status: still valid stub.
    - Tracking: follow-up C below.
11. `internal/infra/workitems/linear/provider.go:83`
    - Original: implement Linear workspace resolution.
    - Status: still valid stub.
    - Tracking: follow-up C below.
12. `internal/infra/calendar/lark_calendar_provider.go:51`
    - Original: implement Lark Calendar API calls for 1:1 discovery.
    - Status: still valid stub.
    - Tracking: follow-up D below.

## Follow-ups

### A. CLI snapshot rendering

- Render structured `plans`, `world_state`, and `diff` content in `alex session snapshot` output instead of only counts/key lists.
- Keep raw JSON output unchanged.
- Verify against snapshots produced by `internal/domain/agent/react/context.go` and `internal/infra/session/state_store`.

### B. Jira work-item provider

- Replace the Jira REST stub in `internal/infra/workitems/jira/provider.go`.
- Implement issue list, single issue fetch, comments, status changes, and accessible workspace discovery.
- Add contract tests beyond current stub-behavior coverage.

### C. Linear work-item provider

- Replace the Linear GraphQL stub in `internal/infra/workitems/linear/provider.go`.
- Implement issue list, single issue fetch, comments, status-change synthesis, and workspace resolution.
- Add contract tests beyond current stub-behavior coverage.

### D. Lark calendar provider

- Replace the stub in `internal/infra/calendar/lark_calendar_provider.go` with real token acquisition and event listing.
- Preserve current credential validation behavior.
- Add integration coverage around 1:1 filtering and mapping to `domain.Meeting`.

## Outcome

- All `TODO` / `FIXME` / `HACK` markers are removed from Go source comments.
- Remaining valid work is tracked here instead of being left as inline code markers.
