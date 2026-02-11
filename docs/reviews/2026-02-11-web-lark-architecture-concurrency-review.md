# Web/Lark Architecture Concurrency Review

**Date**: 2026-02-11  
**Reviewer**: Architecture Review (Principal-Level)

## Executive Summary

Current architecture is structurally sound for single-instance and moderate load:

- layering is clear (`Delivery -> App -> Domain -> Infra`)
- Web and Lark runtime boundaries are mostly explicit
- per-chat serialization in Lark and non-blocking SSE fan-out are good baseline decisions

However, under high concurrency and multi-instance deployment, there are structural gaps:

- no global task admission control
- task resumption is not lease-claimed (duplicate execution risk)
- SSE real-time fan-out is in-memory local (cross-instance blind spot)
- long-lived in-memory maps have incomplete lifecycle cleanup

Overall judgment:

- **Reasonable as V1/V1.5 architecture**
- **Not yet production-grade for horizontal scale without concurrency hardening**

## Runtime Architecture Snapshot

### Web Mode

Entry and execution path:

1. `cmd/alex-server/main.go` -> `RunServer()`
2. `POST /api/tasks` -> `internal/delivery/server/http/api_handler_tasks.go`
3. `ServerCoordinator.ExecuteTaskAsync` -> `TaskExecutionService.ExecuteTaskAsync`
4. background execution in goroutine (`async.Go`)
5. task engine via `AgentCoordinator.ExecuteTask`
6. events fan-out through `EventBroadcaster` -> SSE handler

Concurrency-relevant anchors:

- task goroutine spawn: `internal/delivery/server/app/task_execution_service.go:196`
- detached task context: `internal/delivery/server/app/task_execution_service.go:189`
- resume scan and re-dispatch: `internal/delivery/server/app/task_execution_service.go:559`, `internal/delivery/server/app/task_execution_service.go:625`
- SSE client channel buffer: `internal/delivery/server/http/sse_handler_stream.go:100`
- broadcaster non-blocking drop policy: `internal/delivery/server/app/event_broadcaster.go:347`, `internal/delivery/server/app/event_broadcaster.go:355`

### Lark Mode

Entry and execution path:

1. `cmd/alex-server/main.go` with `lark` arg -> `RunLark()`
2. `startLarkGateway()` -> WebSocket event loop
3. incoming message -> `Gateway.handleMessageWithOptions`
4. per-chat `sessionSlot` state machine gates foreground turns
5. task run via `g.agent.ExecuteTask(...)`

Concurrency-relevant anchors:

- per-chat slot map: `internal/delivery/channels/lark/gateway.go:93`, `internal/delivery/channels/lark/gateway.go:304`
- per-message async task goroutine: `internal/delivery/channels/lark/gateway.go:497`
- pending external-input relay map: `internal/delivery/channels/lark/gateway.go:94`, `internal/delivery/channels/lark/gateway.go:575`
- command-path concurrency cap (`/cc`, `/codex`): `internal/delivery/channels/lark/task_command.go:128`

## What Is Good

1. Clear layering and decoupled composition in bootstrap paths.
2. Lark per-chat slot state machine avoids same-chat race conditions.
3. SSE broadcaster uses non-blocking fan-out with explicit dropped-event signal.
4. Tool execution has a limiter inside ReAct loop (`ToolMaxConcurrent` path).
5. Event ordering wrapper (`SerializingEventListener`) keeps per-run ordering deterministic.

## Findings by Severity

## P0

### P0-1 Duplicate execution risk on resume in multi-instance deployments

Evidence:

- `ResumePendingTasks` scans pending/running then re-dispatches:
  - `internal/delivery/server/app/task_execution_service.go:546`
  - `internal/delivery/server/app/task_execution_service.go:559`
  - `internal/delivery/server/app/task_execution_service.go:625`
- task store status listing has no claim semantics:
  - `internal/infra/task/postgres_store.go:407`
- `SetStatus` uses row lock only for point transition, not scan-and-claim:
  - `internal/infra/task/postgres_store.go:184`
  - `internal/infra/task/postgres_store.go:200`

Impact:

- two instances can resume the same logical task, causing duplicate external side effects.

Recommendation:

- introduce lease-based claim before execution (`owner_id`, `lease_until`, heartbeat/renew).
- use atomic claim SQL (`UPDATE ... WHERE status IN (...) AND (owner_id IS NULL OR lease expired) RETURNING`) or `FOR UPDATE SKIP LOCKED` claim loop.

## P1

### P1-1 No global admission control for task execution

Evidence:

- Web request path spawns one goroutine per accepted task:
  - `internal/delivery/server/http/api_handler_tasks.go:100`
  - `internal/delivery/server/app/task_execution_service.go:196`
- Lark regular chat path also spawns goroutines:
  - `internal/delivery/channels/lark/gateway.go:497`
- `/cc`/`/codex` limit is command-path local, not global:
  - `internal/delivery/channels/lark/task_command.go:128`

Impact:

- burst traffic can cause uncontrolled goroutine growth, model backend saturation, and latency collapse.

Recommendation:

- add process-level admission controller:
  - bounded queue
  - max in-flight execution semaphore
  - per-tenant/session fairness quota
  - clear 429/503 semantics with retry hints

### P1-2 Detached execution context creates orphan-like workload under client churn

Evidence:

- execution context intentionally strips parent cancellation:
  - `internal/delivery/server/app/task_execution_service.go:189`
  - `internal/delivery/server/app/task_execution_service.go:613`

Impact:

- request disconnect does not naturally back off backend compute.

Recommendation:

- keep current detached mode only for explicitly asynchronous jobs, but add:
  - max runtime budget
  - idle watchdog
  - explicit ownership and cancellation policy
  - "detached reason" observability tags

### P1-3 SSE real-time path is instance-local memory, not cluster-wide

Evidence:

- per-instance client registration and local fan-out:
  - `internal/delivery/server/http/sse_handler_stream.go:105`
  - `internal/delivery/server/app/event_broadcaster.go:301`
  - `internal/delivery/server/app/event_broadcaster.go:345`

Impact:

- if execution and SSE connection land on different instances, real-time delivery degrades or disappears unless sticky routing is guaranteed.

Recommendation:

- choose one model explicitly:
  - sticky-session routing by session/run id
  - or cluster event bus (Redis/NATS/Kafka/Postgres LISTEN) for fan-out

## P2

### P2-1 Lark long-lived chat maps do not show explicit global cleanup path

Evidence:

- maps with `LoadOrStore`/`Load` usage:
  - `internal/delivery/channels/lark/gateway.go:93`
  - `internal/delivery/channels/lark/gateway.go:304`
  - `internal/delivery/channels/lark/gateway.go:575`
  - `internal/delivery/channels/lark/gateway.go:584`

Impact:

- cardinality increases with unique chats over long uptime.

Recommendation:

- add TTL + periodic sweeper for inactive chat slot/relay entries.

### P2-2 AI chat session cleanup API exists but is not wired to periodic execution

Evidence:

- cleanup API present:
  - `internal/delivery/channels/lark/ai_chat_coordinator.go:201`
- no runtime call path found for periodic cleanup invocation.

Impact:

- session map growth and stale coordination state.

Recommendation:

- schedule cleanup loop with bounded cadence and metrics.

### P2-3 Scheduler/Timer can run in both server and standalone lark processes

Evidence:

- server mode stages:
  - `internal/delivery/server/bootstrap/server.go:230`
  - `internal/delivery/server/bootstrap/server.go:231`
- lark mode stages:
  - `internal/delivery/server/bootstrap/lark.go:72`
  - `internal/delivery/server/bootstrap/lark.go:73`

Impact:

- if both processes are deployed, proactive jobs may duplicate without leader election.

Recommendation:

- enforce singleton scheduler ownership via distributed lock (for example, DB advisory lock based lease).

### P2-4 Listener serialization can backpressure execution when downstream is slow

Evidence:

- queue size and blocking enqueue:
  - `internal/app/agent/coordinator/event_listener_wrapper.go:15`
  - `internal/app/agent/coordinator/event_listener_wrapper.go:77`

Impact:

- slow downstream listener can throttle core execution path.

Recommendation:

- use tiered event QoS (critical vs diagnostic), and non-blocking drop/degrade for non-critical classes.

## P3

### P3-1 Async history append still adds bounded wait latency when queue is full

Evidence:

- timed wait before returning queue-full:
  - `internal/delivery/server/app/async_event_history_store.go:191`
  - `internal/delivery/server/app/async_event_history_store.go:208`

Impact:

- tail-latency jitter on saturated systems.

Recommendation:

- keep but track explicitly as SLO budget tradeoff; tune append timeout by profile.

## Target Concurrency Model

Recommended end-state model:

1. **Admission Layer**  
Single ingress policy for all task sources (Web, Lark command, Lark normal chat, scheduler/timer):
  - global in-flight cap
  - bounded pending queue
  - per-tenant fairness

2. **Ownership Layer**  
Lease-claimed task ownership:
  - claim
  - heartbeat renew
  - lease expiry takeover
  - idempotency key for side-effecting tool calls

3. **Execution Layer**  
Detached async execution remains, but bounded by:
  - max runtime
  - cancellation policy
  - escalation state transitions

4. **Event Layer**  
Deterministic routing:
  - sticky-route by session/run id, or
  - shared event bus + stateless SSE nodes

5. **Lifecycle Layer**  
All long-lived in-memory maps must have:
  - TTL metadata
  - sweeper job
  - cardinality metrics
  - high-watermark alarms

6. **Control Plane Layer**  
Scheduler/timer single-leader execution with failover lock.

## Phased Remediation Roadmap

### Phase 1 (Fast Risk Reduction)

- Add global execution semaphore + bounded queue for task starts.
- Add metrics:
  - queued_tasks
  - in_flight_tasks
  - rejected_tasks
- Add TTL cleanup loops for Lark slot/relay/session maps.

Success criteria:

- no unbounded goroutine growth under burst tests.
- map cardinalities stabilize under churn tests.

### Phase 2 (Distributed Correctness)

- Implement task claim lease in unified task store.
- Resume path switches from list-and-spawn to claim-and-run.
- Add duplicate-execution guard rails and idempotency keying for side-effect tools.

Success criteria:

- no duplicate task execution in multi-instance chaos/restart tests.

### Phase 3 (Cluster Event Topology)

- Decide and implement one:
  - strict sticky routing, or
  - shared event bus fan-out
- Define failover behavior for live SSE sessions.

Success criteria:

- cross-instance real-time event delivery is deterministic.

### Phase 4 (Operational Hardening)

- Add scheduler/timer single-leader lock.
- Define overload and degradation modes with explicit policy matrix.
- Add SLO dashboards and capacity alerts.

Success criteria:

- stable p95/p99 under synthetic overload and rolling restarts.

## Best-Practice Alignment Notes

The above direction follows common industry patterns:

- staged event-driven backpressure principles (SEDA)
- SRE overload protection and graceful degradation
- lease-based work claiming in distributed workers
- clear control-plane single-leader semantics for schedule execution

This avoids over-orchestration while still providing strong correctness boundaries.
