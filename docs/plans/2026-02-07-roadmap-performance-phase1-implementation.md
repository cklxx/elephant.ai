# Plan: Roadmap Performance Phase-1 Implementation (2026-02-07)

## Goal
- Complete the first executable roadmap slice focused on runtime stability/performance:
  - server restart resume for pending/running tasks,
  - explicit replan signal in workflow stream,
  - runtime tool SLA profile emission in workflow tool-complete events.

## Scope
- Backend only for resume orchestration + persistence.
- Backend + web contract updates for replan and `tool_sla` payload.
- Keep architecture boundaries unchanged (`delivery/app/domain/infra`).

## Checklist
- [x] Add persisted task store option and resumable task query (`pending`/`running`).
- [x] Add server boot recovery hook to resume persisted tasks after coordinator init.
- [x] Add explicit `workflow.replan.requested` domain event and stream translation.
- [x] Wire `SLACollector` in DI so runtime tool execution records SLA metrics.
- [x] Extend SLA snapshot with cost fields and expose `tool_sla` on `workflow.tool.completed`.
- [x] Update SSE allowlist and web event types/schemas for new payload/event.
- [x] Add/extend tests across server app, coordinator translator, react runtime, SLA, SSE, web normalize.
- [x] Run lint/tests and record known pre-existing failures.

## Validation
- Backend targeted tests passed:
  - `go test ./internal/delivery/server/app ./internal/delivery/server/bootstrap ./internal/delivery/server/http`
  - `go test ./internal/domain/agent/react ./internal/app/agent/coordinator ./internal/infra/tools ./internal/app/di`
- Repo test suite passed:
  - `./dev.sh test`
- Lint status:
  - `./dev.sh lint` fails on pre-existing unused const (`internal/domain/agent/react/steward_state_parser.go`).
- Web checks:
  - `pnpm lint` passed in `web/`.
  - `pnpm exec vitest run lib/events/__tests__/eventPipeline.test.ts lib/events/__tests__/normalize.test.ts` passed.
