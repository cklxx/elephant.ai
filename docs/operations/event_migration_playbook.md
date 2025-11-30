# Workflow Event Migration Playbook

This playbook assigns concrete actions for the workflow-first event stream migration so backend/frontend/QA owners can execute in parallel. It complements `web/docs/EVENT_STREAM_ARCHITECTURE.md` (the canonical IDL).

## Owners & Tracks

- Backend owner: event mapper + dual emission.
- Frontend owner: schemas/pipeline/aggregation/UI updates.
- QA/Analytics owner: fixtures, tracking plan, regression.

## Backend (mapper + emission)

1. Implement an event translation layer near `internal/agent/app/agent_workflow.go`:
   - Build a shared envelope `{version,event_type,timestamp,agent_level,workflow_id,run_id|task_id,parent_task_id,session_id,node_id,node_kind,payload,legacy_type?}`.
   - Map existing domain events to new namespaced `event_type` values (see IDL), add `legacy_type` during the migration window.
2. Emit missing events:
   - `workflow.plan.generated` (plan steps/estimates).
   - `workflow.subflow.progress|completed` (delegated agent aggregation).
   - `workflow.tool.progress` for streaming tool output.
3. Diagnostics namespace:
   - Emit `workflow.diagnostic.context_compaction`, `tool_filtering`, `browser_info`, `environment_snapshot`, `sandbox_progress`.
4. SSE handler:
   - Serialize the envelope; optionally dual-emit legacy `event_type` inside `legacy_type`.
   - Update metrics to track new `event_type` values.

## Frontend (schemas + pipeline + UI)

1. Update `web/lib/types.ts` and `web/lib/schemas.ts` to the new `event_type` set; remove inference branches once dual-read is stable.
2. Event pipeline:
   - Adjust `web/lib/events/sseClient.ts` subscription list to new names.
   - Update `eventPipeline` to drop `subagentDeriver` and consume backend `workflow.subflow.*`.
3. Aggregation/UI:
   - Update `web/lib/eventAggregation.ts`, `useAgentStreamStore`, and components (`VirtualizedEventList`, `EventLine`, tool/iteration/step views) to use workflow/node/subflow events.
   - Refresh mocks and dev previews (`web/lib/mocks`, `web/app/dev/console-preview`) to new event names.

## QA/Analytics (validation + tracking)

1. Fixtures:
   - Use `docs/operations/event_fixture_sample.json` as a golden SSE fixture covering all new event types: plan, lifecycle, node start/complete, tool start/progress/complete, subflow progress/complete, diagnostics, final.
   - Extend/clone the fixture for cancel/error cases; use it for store/UI regression and SSE serialization tests.
2. Tracking parity:
   - Sync `internal/analytics/tracking_plan_test.go` with frontend event list; update analytics event registry to new names.
3. Regression:
   - End-to-end flow: SSE → store → UI selectors/components with the fixture.
   - Remove legacy-only paths after the dual-read window closes.

## Migration control

- Dual period: backend emits `legacy_type`, frontend accepts both but prefers new `event_type`.
- Exit criteria: all consumers updated, no legacy event usage in telemetry; then remove dual emission and inference logic.
