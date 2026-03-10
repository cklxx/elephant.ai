# Leader Agent Capability Gap Analysis

## Executive Summary

The codebase already has meaningful infrastructure for the "leader agent" story, but support is uneven across the four pillars.

- Strongest today: `(3) Proactive follow-up` and `(4) Coordination`.
- Partial but real: `(1) Continuous ownership`.
- Weakest: `(2) Attention gating`.

The biggest theme is that the repo has several strong building blocks, but they are not yet unified into a single durable "leader owns the work until done" product loop. The current implementation is strongest at orchestrating active execution, background work, and team delegation. It is weaker at sustained ownership across channels, policy-based attention control, and automatic follow-up against external humans.

## Pillar 1: Continuous Ownership

### Existing capability

- Runtime sessions have explicit lifecycle state, heartbeats, stall detection, and parent/child relationships in `internal/runtime/session/session.go` and `internal/runtime/hooks/stall_detector.go`.
- The runtime leader subscribes to `stalled`, `needs_input`, and `child_completed` events and can inject unblock guidance or escalate after repeated failures in `internal/runtime/leader/leader.go`.
- Background tasks are tracked with lifecycle state, dependencies, merge state, and workspace mode inside `internal/domain/agent/react/background_dispatch.go`.
- The Lark channel persists per-chat task records, keeps progress messages alive, and can recover missed completion signals through task-store polling in:
  - `internal/delivery/channels/lark/task_store.go`
  - `internal/delivery/channels/lark/task_store_local.go`
  - `internal/delivery/channels/lark/background_progress_listener.go`
  - `internal/delivery/channels/lark/background_progress_flush.go`
- The domain model already defines a much stronger unified task abstraction with leases, transitions, bridge checkpoints, dependencies, and session/chat queries in `internal/domain/task/store.go`.
- Bridge-resume logic exists for orphaned external-agent work in `internal/infra/external/bridge/resumer.go`, which is a real ownership/resilience primitive.

### What this means

The system can already keep track of active execution, recover some interrupted background work, and keep pushing work forward when a task is in flight. That is real "ownership during execution."

### Gaps

- There is no single product-wide owner-of-record store wired end to end. The stronger `internal/domain/task` model exists, but live bootstraps still mostly use local channel/server stores instead of the unified domain store:
  - Lark uses `buildLarkTaskStore()` with local memory/file stores in `internal/delivery/server/bootstrap/lark_gateway.go`.
  - The HTTP server uses `NewInMemoryTaskStore()` in `internal/delivery/server/bootstrap/server.go`.
  - `internal/delivery/taskadapters/lark_adapter.go` exists, but it is not the main runtime wiring path.
- Lark task persistence is chat-scoped and retention-limited, not a durable cross-channel ownership ledger.
- Ownership logic is concentrated around running tasks, not around long-lived commitments. There is no unified notion of:
  - task SLA / due date
  - next follow-up time
  - owner / delegated owner / watcher set
  - blocked-on-person-X state
  - resolution criteria
- The runtime leader is reactive to stalls and child completion, but it is not yet a general planner/executor that continuously re-opens unfinished obligations.

### Verdict

`Partial support.` The repo supports execution-time ownership well enough to be credible, but it does not yet fully implement durable, cross-channel continuous ownership.

## Pillar 2: Attention Gating

### Existing capability

- There is a real notification center with channel routing, priority levels, and delivery history in `internal/app/notification/notification.go`.
- The Lark channel has several summarization/filtering mechanisms:
  - group discussion summarization in `internal/infra/lark/summary/group.go`
  - slow-progress summaries in `internal/delivery/channels/lark/slow_progress_summary_listener.go`
  - team completion summaries in `internal/delivery/channels/lark/background_progress_summary.go`
  - user-input request formatting and narrowing in `internal/delivery/channels/lark/input_request_listener.go`
- Context compression and history summarization exist in:
  - `internal/app/context/manager_compress.go`
  - `internal/app/agent/preparation/history.go`
- The runtime leader can escalate stalled or blocked sessions through `EventHandoffRequired` in `internal/runtime/leader/leader.go`.

### What this means

The codebase can summarize noisy execution and it has the plumbing to route urgent notifications. It can also convert some internal runtime states into human-facing prompts or escalations.

### Gaps

- The strongest gap is policy enforcement. `runtime.proactive.attention.*` is defined in config, defaulted in load/merge, but I did not find business-side consumers that actually enforce:
  - max daily notifications
  - minimum interval between notifications
  - quiet hours
  - priority threshold

  Relevant files:
  - `internal/shared/config/types.go`
  - `internal/shared/config/load.go`
  - `internal/shared/config/proactive_merge.go`
  - `docs/reference/config-field-governance-analysis.md`

- Attention gating is currently mostly "summarize and notify," not "decide whether this deserves interruption now."
- Escalation exists at the runtime-event layer, but there is no broad policy engine that scores interrupt-worthiness across tasks, channels, and reminders.
- `startRuntimeCompletionNotifier()` only sends notifications for `completed` and `failed` events in `internal/delivery/server/bootstrap/hooks_bridge.go`; it does not consume `handoff_required`, which weakens the actual escalation path.
- Summarization is fragmented by subsystem rather than unified into one attention queue or inbox.

### Verdict

`Weak / partial support.` There is plenty of summarization and delivery plumbing, but not yet a strong attention-governor that decides when to stay silent, summarize, escalate, or interrupt.

## Pillar 3: Proactive Follow-up

### Existing capability

- The proactive scheduler is a substantial implemented subsystem in `internal/app/scheduler/scheduler.go`.
- It supports:
  - cron-like triggers
  - persisted jobs
  - cooldown / concurrency control
  - recovery behavior
  - OKR-derived dynamic triggers
  - calendar reminder registration
  - heartbeat registration
- Concrete trigger paths exist in:
  - `internal/app/scheduler/calendar_trigger.go`
  - `internal/app/scheduler/heartbeat_trigger.go`
- Reminder generation exists as a reusable pipeline in `internal/app/reminder/pipeline.go`.
- The README positioning around proactive operation is grounded in actual scheduling/runtime code, not just copy.

### What this means

This is the most convincing pillar after coordination. The system can already wake itself up, run periodic work, check calendars, and perform heartbeat-based attention scans.

### Gaps

- The current follow-up model is scheduler-driven, not commitment-driven. It can run periodic prompts, but it does not yet maintain a first-class "I am waiting on Alice for update X by time Y" follow-up queue.
- The reminder pipeline is confirmation-centric. It is useful for reminder composition, but it is not the same as autonomous status chase.
- There is no general built-in "auto chase an external stakeholder until resolved" workflow with backoff, stop conditions, and escalation.
- Scheduler leader-lock config exists, but bootstrap explicitly passes no distributed lock in `internal/delivery/server/bootstrap/scheduler.go`. That limits safe single-leader behavior in multi-instance deployments.
- The system can remind "me" or run internal checks, but there is less evidence of robust external follow-up loops against other participants across Lark/email/task systems.

### Verdict

`Strong partial support.` The core proactive infrastructure is real and useful, but it is not yet a generalized autonomous follow-up manager for external commitments.

## Pillar 4: Coordination

### Existing capability

- Multi-agent/team orchestration is the clearest implementation match to the leader-agent narrative.
- Team templates, role definitions, staged execution, and collaboration context are defined in config and taskfile paths:
  - `internal/shared/config/types.go`
  - `internal/infra/tools/builtin/orchestration/team_runner.go`
- Team runtime bootstrap creates per-team runtime state, role bindings, CLI capability selection, event logs, and tmux role panes in:
  - `internal/infra/teamruntime/bootstrap.go`
  - `internal/infra/teamruntime/types.go`
  - `internal/infra/teamruntime/recorder.go`
- Team run records are durably captured in `internal/infra/external/teamrun/file_recorder.go`.
- Parent/child session relationships are explicit in `internal/runtime/session/session.go`.
- The runtime leader can react when child sessions complete and decide the next orchestration step in `internal/runtime/leader/leader.go`.
- There is explicit evaluation coverage for leader/team orchestration expectations:
  - `internal/infra/integration/agent_teams_leader_e2e_test.go`
  - `evaluation/agent_eval/datasets/leader_agent_e2e_rubric.yaml`

### What this means

This is the most mature pillar. The repo clearly supports delegation, staged dependency ordering, context inheritance, and mixed-agent execution. The architecture already treats coordination as a first-class concern.

### Gaps

- Coordination is strongest for internal/external agent teams, weaker for human coordination.
- I did not find a unified multi-party alignment layer that tracks:
  - who has been informed
  - who still owes input
  - what decision is blocked on which stakeholder
  - what thread/document/task holds the current source of truth
- Runtime `handoff_required` events are generated, but the downstream operator/human escalation path looks thin. It is not yet a full coordination loop.
- Team runtime artifacts are durable, but they are not clearly surfaced back into a product-level coordination dashboard or ownership view.

### Verdict

`Strong support.` Coordination among agent workers is already a real product capability. Coordination among humans and mixed human-agent teams is still thinner than the positioning implies.

## Cross-Cutting Gaps

- No unified durable task/obligation ledger is actually wired as the main path, despite `internal/domain/task` defining the right abstraction.
- No strong attention policy engine consumes `runtime.proactive.attention.*`.
- Escalation signals exist, but operator/human handoff handling is incomplete downstream.
- Proactive logic is mostly timer/scheduler based, not obligation/state-machine based.
- Cross-channel continuity exists in pieces, but not yet as one coherent "leader agent owns this until it is truly done" loop.

## Bottom Line

The current codebase already supports a credible "leader agent" story if positioned carefully:

- Best-supported claims today:
  - multi-agent delegation and orchestration
  - proactive scheduled checks and reminders
  - progress tracking and recovery for active/background execution
  - summarization of noisy work into human-readable updates

- Claims that are not fully backed yet:
  - always-on continuous ownership across channels and long-lived obligations
  - robust attention gating with quiet-hours / threshold / interruption policy
  - autonomous status chasing of external humans until closure
  - complete multi-party human coordination and escalation loops

If the product wants the positioning to be fully true, the highest-leverage next step is to wire the unified task/obligation model as the central source of truth, then layer attention policy and follow-up state machines on top of it.
