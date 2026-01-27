# Plan: Subagent attachment propagation to parent tool result (2026-01-27)

## Goal
- Ensure attachments generated during subagent runs are exposed on the parent tool result so the main agent can reference them.
- Avoid reintroducing inherited attachments as “new” when only passing through baseline context.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Trace subagent event flow and attachment emission points.
2. Add an attachment-carrier interface for events that expose attachments.
3. Capture attachments during subagent execution and attach them to the subagent tool result.
4. Add tests covering attachment capture + inherited filtering.
5. Run full lint + tests.
6. Commit changes.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added attachment carrier interface + event methods, introduced subagent attachment collector with inherited filtering, and added subagent attachment capture test.
- 2026-01-27: Ran `make fmt` (fixed pre-existing unused `styleBoldGreen`), then `make test` (pass).
