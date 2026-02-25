# Plan: Optimize Agent Teams Lark Notification UX + Monitoring

Date: 2026-02-24
Status: Draft (ready for implementation)
Owner: Codex + cklxx

## 1. Goal

For Agent Teams running in Lark:
- Make progress/notification display easier to scan in group/DM chats.
- Build monitoring that can quickly detect delivery regressions, event storms, and UX overload.
- Minimize user cognitive load while keeping control/approval confidence.

## 2. Constraints and Current Base

Existing event chain already supports typed envelopes and listener composition:
- Gateway + listener chain: `internal/delivery/channels/lark/gateway.go`, `internal/delivery/channels/lark/task_manager.go`
- Progress listener: `internal/delivery/channels/lark/progress_listener.go`
- Background progress listener: `internal/delivery/channels/lark/background_progress_listener.go`
- Plan/clarify listener: `internal/delivery/channels/lark/plan_clarify_listener.go`
- Event translation: `internal/app/agent/coordinator/workflow_event_translator.go`
- Event types: `internal/domain/agent/types/events.go`
- Broadcaster metrics/drop signals: `internal/delivery/server/app/event_broadcaster.go`

Current strengths:
- Dedup and rate-limit already exist.
- Background progress + input-request bridge already exist.
- Typed events and IDs (`session_id`, `run_id`, `task_id`) are available for correlation.

Current gaps:
- Display narrative is fragmented (progress, clarify, background updates are separate text flows).
- Monitoring lacks a channel-specific “cognitive load” layer.
- No explicit policy of “what should notify vs silently update”.

## 3. Design Principles (Low Cognitive Load First)

1. Quiet by default: update existing message first, send new message only on milestone/blocker.
2. One spine per task: one primary progress thread, avoid parallel noisy updates.
3. Progressive disclosure: default short summary, expand details only on demand.
4. Action-oriented interruptions only: only block user for approvals/input/time-sensitive failures.
5. Deterministic structure: every state transition maps to a stable display behavior.

## 4. UX Model for Lark

### 4.1 Message hierarchy

Use 3 layers only:
- Layer A (Primary status): single editable progress message per foreground task.
- Layer B (Milestone alerts): short replies for key transitions only.
- Layer C (Action required): explicit approval/input card or numbered options.

### 4.2 Event-to-display policy

Classify events into:
- `silent_update`: tool start/progress/delta output -> edit Layer A only.
- `milestone`: plan finalized, subflow completed, background task completed -> one Layer B message.
- `blocking`: external input requested, approval required, task failed unrecoverably -> Layer C message.

### 4.3 Agent Team specific presentation

For subagent/team tasks:
- Group by `task_id` and render one concise section:
  - `任务: <description>`
  - `状态: running/completed/failed`
  - `最近动作: <friendly phrase>`
  - `耗时 + tokens + merge status`
- Suppress per-subtask chatter unless failure or user asked for detail.
- Keep one “team summary” line in main progress:
  - `Team 进度: 3/5 完成, 1 失败, 1 进行中`

### 4.4 Final output protocol

Final reply always follows:
- Result summary (2-5 lines)
- What changed / artifacts
- Pending risks or user decision needed
- Optional “show details” pointer (not full transcript by default)

## 5. Monitoring Architecture

## 5.1 Observability layers

1) Delivery health:
- send/update success rate (Lark API)
- notification latency p50/p95/p99
- retry/failure distribution by event type

2) Stream health:
- dropped events (`workflow.stream.dropped`)
- no-client events
- dedup hit ratio
- queue saturation rate

3) UX load health (new):
- avg outgoing messages per task
- avg progress edits per task
- blocking prompts per task
- user follow-up clarification rate
- “status query” frequency (proxy for confusion)

4) Outcome health:
- task completion rate
- median time-to-final-answer
- approval turnaround time
- background task completion/merge success rates

## 5.2 Correlation fields (mandatory)

Every notification metric/log should include:
- `chat_id`, `session_id`, `run_id`, `task_id`, `event_type`, `node_kind`

## 5.3 Alert policy

P1 alerts:
- send/update failure rate > 5% for 5m
- stream dropped events spike (>= 3x baseline)
- blocking prompts timeout rate > threshold

P2 alerts:
- messages per task > configured budget
- status-query frequency > baseline (cognitive overload signal)

## 6. Implementation Plan

### Phase 0: Event policy normalization
- Add event notification policy mapper (`silent_update|milestone|blocking`) near Lark listener composition.
- File targets:
  - `internal/delivery/channels/lark/task_manager.go`
  - `internal/app/agent/coordinator/workflow_event_translator.go`

### Phase 1: Unified display composer
- Introduce a small `lark notification composer` that all listeners call.
- Keep one source of truth for message format + throttling.
- File targets:
  - `internal/delivery/channels/lark/progress_listener.go`
  - `internal/delivery/channels/lark/background_progress_listener.go`
  - `internal/delivery/channels/lark/plan_clarify_listener.go`

### Phase 2: Monitoring instrumentation
- Add channel-specific metrics hooks for Lark notify pipeline.
- Expose dashboard-friendly counters/histograms for cognitive load.
- File targets:
  - `internal/infra/observability/instrumentation.go`
  - `internal/infra/observability/metrics.go`
  - `internal/delivery/channels/lark/gateway.go`
  - `internal/delivery/server/app/event_broadcaster.go`

### Phase 3: Controlled rollout
- Feature flags:
  - `lark.notification_policy_v2`
  - `lark.notification_compose_v2`
  - `lark.notification_metrics_v2`
- Rollout 10% -> 50% -> 100% with weekly compare.

## 7. Acceptance Criteria

Functional:
- Foreground tasks keep one primary progress message.
- Background tasks produce <= 1 periodic status update per configured interval.
- Blocking events always produce actionable prompt.

Reliability:
- send/update success >= 99%
- no event-loss regression vs baseline

Cognitive load:
- median outgoing messages per task reduced by >= 30%
- status-query frequency reduced by >= 20%
- no drop in task completion or user approval confidence

## 8. Risks and Mitigations

Risk: Over-suppression hides useful detail.
- Mitigation: keep opt-in detail command (`/status detail`, `/task list`).

Risk: Listener overlap creates duplicate notifications.
- Mitigation: central policy gate + idempotency key per (`task_id`, `event_type`, `node_id`).

Risk: Monitoring overhead affects runtime.
- Mitigation: sample high-frequency events, aggregate by window.

## 9. Next Step

Start with Phase 0 + Phase 1 in one PR:
- establish policy mapper
- route current listeners through unified composer
- keep existing behavior as fallback behind flag
