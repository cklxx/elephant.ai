# 2026-03-03 Lark Terminal Delivery Architecture Refactor (Event-Driven + Outbox)

Updated: 2026-03-03 22:00  
Status: Implemented (Phase 0-2 baseline)

## 0. Implementation Snapshot (2026-03-03)

Implemented in this wave:
1. Added `DeliveryIntent` + `DeliveryOutboxStore` abstractions with local memory/file implementation.
2. Added outbox worker (`delivery_worker.go`) with claim/send/retry/dead-letter state transitions.
3. Refactored `task_manager.dispatchResult` to build terminal intents and route by `delivery_mode` (`direct|shadow|outbox`).
4. Added bootstrap/runtime YAML config support for delivery mode and worker tunables.
5. Wired gateway startup to initialize outbox store and start worker in `outbox` mode.
6. Added unit/integration tests for outbox dedupe, retry/dead/replay, and worker delivery behavior.

Not yet implemented in this wave:
1. Unified progress/attachment delivery into multi-intent type taxonomy (current baseline keeps terminal intent + attachment send in one delivery step).
2. Dedicated metrics family (`delivery_intent_*`) and alert wiring.
3. Operator-facing replay command endpoint/CLI.

## 1. Background and Problem Statement

Today, Lark terminal replies (final answer / failure / await-user-input prompts) are sent synchronously in `task_manager.runTask -> dispatchResult`, tightly coupled to the task execution context.  
When the execution context times out or is canceled (for example, `ReplyTimeout`), terminal delivery can be canceled as a side effect, causing "task completed but no final message received."

The core issue is not "can we send once," but that **terminal delivery lacks an independent reliable delivery semantic**:
1. Delivery lifecycle is coupled to execution lifecycle.
2. There is no unified delivery state machine (`pending/sent/failed/retrying/dead`).
3. There is no explicit idempotency key and replay mechanism.

## 2. Target Architecture

Adopt a three-stage model: **Domain Event + Delivery Intent Outbox + Async Delivery Worker**.

1. Domain/Coordinator emits terminal events (for example, `workflow.result.final`).
2. Delivery Intent Builder converts events into `DeliveryIntent` (terminal text, attachments, chat_id, idempotency key, sequence).
3. Persist intent and task status in the same transactional boundary (or equivalent durable boundary).
4. Delivery Worker asynchronously claims intents and delivers to Lark with retry policy.
5. Delivery result is written back; over retry limit goes to dead-letter and can be replayed.

## 3. Key Design Points

### 3.1 Reliability Boundary

- Decouple task execution success/failure from message-delivered status:
  execution completion only guarantees intent persistence; delivery success is owned by Worker.
- Worker must not rely on task execution context; it uses its own timeout and retry budget.

### 3.2 Idempotency and Ordering

- Recommended idempotency key: `lark:{chat_id}:{run_id}:{event_type}:{sequence}`.
- Require monotonic sequence within same `chat_id + run_id`; `final` event has highest priority.
- Replay uses idempotency keys for dedupe, ensuring at-least-once delivery with business idempotency.

### 3.3 Retry Policy

- Exponential backoff + jitter + max retry count.
- Retry only retriable failures (429/5xx/network instability); fail fast for semantic 4xx errors with alerting.
- Maintain retry budget to avoid blast-radius amplification.

### 3.4 Observability

Required signals:
1. `delivery_intent_pending_total`
2. `delivery_intent_retry_total`
3. `delivery_intent_dead_total`
4. `terminal_delivery_latency_ms` (event creation to successful delivery)
5. Traceable logs by `chat_id/run_id/intent_id`

## 4. Mapping to Current Code (Proposed)

### New Files

1. `internal/delivery/channels/lark/delivery_outbox_store.go`
   - Intent model, delivery mode constants, and outbox interface.
2. `internal/delivery/channels/lark/delivery_outbox_local.go`
   - Persistence interface and implementation (file store first, DB later).
3. `internal/delivery/channels/lark/delivery_worker.go`
   - Poll/claim, deliver, retry, write-back.
4. `internal/delivery/channels/lark/delivery_worker_test.go`
   - Coverage for idempotency, retry, dead-letter, replay.

### Refactors

1. `internal/delivery/channels/lark/task_manager.go`
   - Change `dispatchResult` from direct-send to enqueue-intent.
2. `internal/app/agent/coordinator/workflow_event_translator_react.go`
   - Ensure terminal envelope fields are complete (`answer/stop_reason/attachments/seq`).
3. `internal/delivery/server/app/event_broadcaster.go` (optional)
   - Unified terminal event ingress if a shared delivery pipeline is desired.

## 5. Phased Rollout Plan

### Phase 0 (Completed)
- Minimum stopgap: send terminal messages with detached context.

### Phase 1 (Low Risk)
- Introduce `DeliveryIntent` model and Outbox Store.
- Terminal path does "enqueue intent + still direct-send" (shadow mode).
- Acceptance: intent generation count matches terminal direct-send count 1:1.

### Phase 2 (Cutover)
- Switch terminal path to Worker async delivery; keep direct-send as feature-flagged fallback.
- Acceptance: `terminal_delivery_missing_rate` drops significantly with no visible user regression.

### Phase 3 (Unified Delivery)
- Bring progress edits and attachment sends into the intent mechanism (multi-type intents).
- Add dead-letter replay command.

### Phase 4 (Convergence)
- Remove legacy direct-send primary path, keep emergency bypass.
- Sync docs, runbook, and alert strategy.

## 6. Testing and Acceptance

### Unit Tests
1. Intent can still be delivered after task context cancellation.
2. Duplicate enqueue with same idempotency key sends only once.
3. 429/5xx retries; 4xx does not retry.
4. Over max retries transitions to dead-letter.

### Integration Tests
1. Inject intermittent Lark API failures and verify eventual terminal delivery.
2. Restart process and verify unfinished intents can resume and deliver.
3. Under high chat concurrency, terminal ordering remains correct.

### Runtime SLO/Signals
1. `terminal_delivery_latency_p95` < 5s (example target)
2. `terminal_delivery_missing_rate` approaches 0
3. Dead-letter queue is observable and replayable

## 7. Risks and Rollback

Risks:
1. During dual-write (direct-send + outbox), duplicates may occur.
2. Worker failures may accumulate pending backlog.

Mitigations:
1. Idempotency keys + sender-side dedupe.
2. Monitor pending backlog and auto-scale worker.
3. Keep feature flag for one-step rollback to direct-send.

## 8. Conclusion

The incident exposed a missing **terminal delivery semantic**, not a single code bug.  
Recommended direction is a reliable event-driven delivery architecture:

**event emission (traceable) -> durable intent persistence (recoverable) -> async delivery (retryable) -> status write-back (auditable).**
