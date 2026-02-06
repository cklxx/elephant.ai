# Plan: Tool SLA Profiles in Event Stream

## Goal
Provide per-tool latency and cost profiling that is persisted in-memory and surfaced via workflow tool events to the web.

## Current State (Findings)
- Tool SLA tracking exists in `internal/infra/tools/sla.go` with sliding-window latency percentiles, error rate, call count, and success rate.
- `internal/infra/tools/sla_executor.go` records per-call latency and errors when an `SLACollector` is configured.
- Tool registry wraps executors with SLA when `toolregistry.Config.SLACollector` is provided, but DI does not currently set it (`internal/app/di/container_builder.go`).
- Workflow tool events are translated in `internal/app/agent/coordinator/workflow_event_translator.go`.
  - `workflow.tool.completed` payload includes `duration`, `metadata`, and `attachments` only.
- SSE payloads are built in `internal/delivery/server/http/sse_render.go` and forwarded as-is to the web.
- Web event schemas for tool events live in `web/lib/schemas.ts` and expect the current payload shape.

## Plan
1. Wire SLA collector in DI.
   - Instantiate a shared `SLACollector` during container build and pass it into `toolregistry.Config.SLACollector`.
   - Keep collector accessible for event translation (store in coordinator or pass into workflow translator).

2. Extend SLA collector to record tool cost and expose it in snapshots.
   - Add optional cost tracking in `internal/infra/tools/sla.go` (e.g., total cost, p50/p95/p99 cost, and mean cost).
   - Update `RecordExecution` signature to accept `costUSD` (float64, optional).
   - Provide a helper to read `cost_usd` from `ToolResult.Metadata`.

3. Record cost at execution time.
   - Update `SLAExecutor.Execute` to extract cost from `ToolResult.Metadata` and pass into `RecordExecution`.
   - Standardize the metadata key to `cost_usd` (float64) for tool implementations that can compute cost.

4. Emit SLA profile in workflow tool events.
   - Extend the workflow event translator to include a `tool_sla` payload on `workflow.tool.completed` when an `SLACollector` is present.
   - Include latency percentiles, success/error rates, call count, and cost stats.

5. Accept new payload on the web.
   - Update `web/lib/schemas.ts` to allow `payload.tool_sla` for tool events.
   - Add or update UI handling only if needed; otherwise keep data available for debugging panels.

## Tests
- `internal/infra/tools/sla_test.go`: new tests for cost aggregation and `RecordExecution` cost path.
- `internal/infra/tools/sla_executor_test.go` or existing SLA executor tests: verify `cost_usd` metadata gets recorded.
- `internal/app/agent/coordinator/workflow_event_translator_test.go`: ensure `workflow.tool.completed` emits `tool_sla` payload when collector is configured.
- `web/lib/events/__tests__/normalize.test.ts` or schema tests: validate tool events accept `tool_sla` payload.

## Notes
- Keep `agent/ports` free of memory/RAG dependencies.
- Maintain YAML-only config examples if any are added.
