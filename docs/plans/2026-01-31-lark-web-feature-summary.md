# Plan: Summarize Lark Agent Features & Event Flows + Web Delivery

**Created:** 2026-01-31
**Status:** Draft

---

## Goal

Produce a concise technical summary of:
1. Lark channel — features, event flow, and integration points.
2. Web delivery — SSE streaming, API surface, and frontend event pipeline.

This document serves as a living reference for onboarding and architecture reviews.

---

## 1 · Lark Agent Summary

### 1.1 Core Files

| File | Responsibility |
|------|----------------|
| `internal/channels/lark/gateway.go` | Message dispatch, dedup (LRU 2048/10 min), session management (fresh/stable), auto chat context, plan review, attachment upload, emoji reactions |
| `internal/channels/lark/config.go` | Config struct (AppID, AppSecret, BaseDomain, SessionMode, ReactEmoji, ShowToolProgress, AutoChatContext, PlanReview) |
| `internal/channels/lark/progress_listener.go` | Real-time tool progress in Lark (rate-limited 2 s flush, respects 5 QPS) |
| `internal/channels/lark/chat_context.go` | Fetches last N chat messages and formats as chronological context |
| `internal/channels/lark/plan_review_store.go` / `plan_review_postgres.go` | Approval-gate persistence (Postgres, upsert, TTL expiry) |
| `internal/channels/lark/emoji_reactions.go` | Random start/end emoji picker; configurable pool |
| `internal/channels/base.go` | Shared gateway: per-session mutex, context builder, presets, timeout |
| `internal/channels/executor.go` | `AgentExecutor` interface (`EnsureSession`, `ExecuteTask`) |
| `internal/tools/builtin/larktools/chat_history.go` | `lark_chat_history` tool — queries chat history from within an agent task |
| `internal/server/bootstrap/lark_gateway.go` | Bootstrap: starts gateway goroutine, dual tool modes (CLI/Web), registers EventBroadcaster |

### 1.2 Message Event Flow

```
Lark user sends message
  → Lark SDK event dispatcher (P2MessageReceiveV1)
  → handleMessage(): extract sender, chat_id, content; filter by type/chat_type
  → Dedup check (LRU cache, 10-min TTL)
  → Memory ID derivation (SHA1(chatID))
  → Per-chat session lock
  → EnsureSession() or ResetSession()
  → [if group + enabled] Auto chat context fetch (last N messages)
  → [if enabled] Load pending plan review from Postgres
  → [async] Emoji reaction start
  → [if enabled] Attach progress listener
  → coordinator.ExecuteTask() (ReAct loop)
      ├─ Tool progress events → progressListener → Lark message update
      └─ TaskResult (answer + attachments)
  → [if StopReason == "await_user_input"] Save plan review
  → Build reply text + attachment summary
  → Send/reply (group: reply to msg; P2P: new message)
  → Upload attachments (image → uploadImage; file → uploadFile)
  → [async] Emoji reaction end
  → Session save
```

### 1.3 Key Features

- **Message dedup** — LRU (2048 entries, 10-min TTL).
- **Session modes** — `fresh` (new per message) or `stable` (per chat).
- **Auto chat context** — Injects recent group messages as context before execution.
- **Plan review** — Approval gate with Postgres persistence and TTL.
- **Real-time progress** — In-place Lark message updates, rate-limited.
- **Emoji reactions** — Configurable start/end reaction pool.
- **Attachment upload** — Images via `uploadImage`; files via `uploadFile` with MIME-type mapping.
- **Chat history tool** — `lark_chat_history` available inside agent tasks.

---

## 2 · Web Delivery Summary

### 2.1 Backend — SSE & API

| File | Responsibility |
|------|----------------|
| `internal/server/http/router.go` | Route registration (Go 1.22+ method patterns) |
| `internal/server/http/api_handler_tasks.go` | `POST /api/tasks` (create), `GET /api/tasks` (list), cancel |
| `internal/server/http/api_handler_sessions.go` | Session CRUD, fork, share, persona |
| `internal/server/http/sse_handler_stream.go` | SSE lifecycle: headers, channel (100), register, replay, heartbeat (30 s), dedup |
| `internal/server/http/sse_render.go` | Event → JSON serialization, streaming delta computation |
| `internal/server/http/sse_render_attachments.go` | Inline data → cached URLs, LRU (512), force-include on final |
| `internal/server/http/sse_render_payload.go` | Payload sanitization |

**SSE replay modes:** `full` (all history), `session` (session-only), `none` (live only).

**Key API surface:**

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/tasks` | Create task (async, returns run_id) |
| GET | `/api/sse` | SSE event stream |
| GET/POST/DELETE | `/api/sessions[/{id}]` | Session CRUD |
| POST | `/api/sessions/{id}/fork` | Fork session |
| POST | `/api/sessions/{id}/share` | Create share token |
| GET/PUT | `/api/sessions/{id}/persona` | User persona |

### 2.2 Frontend — SSE Event Pipeline

| File | Responsibility |
|------|----------------|
| `web/hooks/useSSE/useSSE.ts` | Main orchestrator: composes sub-hooks, event clamping (max 1000), final-event merging |
| `web/hooks/useSSE/useSSEConnection.ts` | EventSource lifecycle, two-phase reconnect (fast exponential backoff → slow 60 s) |
| `web/hooks/useSSE/useSSEDeduplication.ts` | Two-tier dedup (event ID + seq per run) |
| `web/hooks/useSSE/useSSEEventBuffer.ts` | Microtask-batched event flush |
| `web/hooks/useSSE/useStreamingAnswerBuffer.ts` | Streaming delta accumulation |
| `web/lib/events/sseClient.ts` | EventSource factory → EventPipeline → agentEventBus |
| `web/lib/events/eventPipeline.ts` | Schema validation → event registry → bus emission |

### 2.3 End-to-End Event Flow

```
User submits task (ConversationPageContent)
  → useTaskExecution: POST /api/tasks → { run_id, session_id }
  → useSSE connects: GET /api/sse?session_id=...&replay=session
      Backend:
        1. Set SSE headers, create channel (100)
        2. Register with EventBroadcaster
        3. Send "connected" event
        4. Replay history from storage
        5. Start listening for live events
      Agent (async):
        coordinator.ExecuteTaskAsync() → ReAct loop
          → emits workflow.node.started, .output.delta, .tool.started, .tool.completed, .result.final
        EventBroadcaster.Broadcast() → per-client channels
      SSE render:
        1. Allowlist filter (25+ event types)
        2. Dedup by event_id/seq
        3. Normalize attachments (data URI → cache URL, LRU 512)
        4. Compute streaming delta
        5. Write SSE line, flush
      Frontend:
        EventSource.onmessage → sseClient → EventPipeline.process()
          → agentEventBus → useSSE listener
          → useSSEEventBuffer: microtask batch flush
          → useSSEDeduplication: skip seen IDs
          → useStreamingAnswerBuffer: accumulate deltas
          → setEventState (React re-render)
          → ConversationEventStream: sort, partition, render
```

### 2.4 Key Features

- **SSE allowlist** — Only 25+ whitelisted event types reach the frontend; `react:*` nodes suppressed.
- **Two-phase reconnect** — Fast backoff (1 s → 30 s max) then slow retry (60 s).
- **Attachment normalization** — Inline data URIs converted to cache-backed URLs; LRU (512); force-include on terminal event.
- **Streaming deltas** — `result.final` with `is_streaming=true` sends only the diff vs. previous.
- **Event clamping** — Max 1000 events in memory; consecutive `output.delta` events merged.
- **Session management** — CRUD, fork, share, user persona, replay modes.
- **LLM selection** — Per-request model override via localStorage → SelectionResolver.
- **Observability** — Metrics on connection count, duration, message size, errors.

---

## 3 · Cross-Cutting Patterns

| Pattern | Lark | Web |
|---------|------|-----|
| **Deduplication** | LRU cache (2048, 10 min) | Two-tier: event_id + seq per run (both backend & frontend) |
| **Streaming** | In-place Lark message updates (rate-limited) | SSE with delta computation + microtask buffer |
| **Attachments** | Upload via Lark API (image_key / file upload) | Inline → cache URL normalization, LRU (512) |
| **Session** | Fresh or stable mode per chat | Session CRUD, fork, share, replay modes |
| **Approval gates** | Plan review with Postgres persistence | Plan review via event flow (await_user_input) |
| **Context injection** | Auto chat context (last N group messages) | Session replay + context compression diagnostics |
| **Reconnection** | N/A (Lark SDK manages WS) | Two-phase: fast backoff + slow 60 s retry |

---

## Next Steps

- [ ] Review and finalize with team.
- [ ] Optionally extract into a standalone architecture doc if this grows.
