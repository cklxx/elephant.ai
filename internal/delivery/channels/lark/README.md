# Lark Gateway

Bridges Lark bot messages into the agent runtime. Supports two operating modes depending on configuration.

## Message Routing

Every incoming Lark message flows through `handleMessage` → `handleMessageWithOptions`:

```
Lark WS event
  │
  ├── parseIncomingMessage (dedup, mention filter, content extract)
  ├── AI Chat Coordinator (multi-bot session handling, if configured)
  │
  ├── /stop, /new, /reset, /model, /notice, /usage  →  command handlers
  ├── natural status query ("做到哪了")              →  handleNaturalTaskStatusQuery
  │
  ├── ConversationProcessEnabled?
  │     YES → handleViaConversationProcess  (chat+worker mode, see below)
  │     NO  → legacy single-process mode:
  │             ├── slotRunning? → inject inputCh / btw fork
  │             └── slotIdle?    → launchWorkerGoroutine (foreground task)
  │
  └── AttentionGate (auto-ack non-urgent, if configured)
```

---

## Chat + Worker Mode (`ConversationProcessEnabled=true`)

Two concurrent goroutine roles per chat:

```
User ──▶ Chat Process ──▶ instant reply (~1-3s)
              │
              ├── dispatch_worker ──▶ Worker Process (background, minutes)
              ├── stop_worker     ──▶ cancel Worker
              └── direct reply    ──▶ text / progress / chat
```

### Chat Process

| Aspect | Detail |
|--------|--------|
| Entry | `handleViaConversationProcess` |
| LLM | lightweight call, 8s timeout, 300 max tokens, temp 0.3 |
| Tools | `dispatch_worker` (start/inject task), `stop_worker` (cancel task) |
| Context | worker snapshot + recent chat history (5 rounds) |
| Responsibility | instant reply, task routing, stop control, progress reporting |

**System prompt rules** (`conversationSystemPrompt`):
1. Casual chat / status query → direct reply
2. Tool operations needed → `dispatch_worker` + short confirmation
3. Keep replies under 100 chars, natural tone
4. Running task + user asks progress → answer from snapshot
5. Running task + user sends supplement → `dispatch_worker` (injects via `inputCh`)
6. User asks to stop → `stop_worker`

### Worker Process

| Aspect | Detail |
|--------|--------|
| Entry | `spawnWorker` → `launchWorkerGoroutine` → `runTask` |
| Runtime | full ReAct Agent with tool execution |
| Input | receives injected messages via `inputCh` (buffered, cap 16) |
| Progress | tool events recorded to slot via `slotProgressRecorder` (ring buffer, max 8) |
| Cleanup | goroutine resets slot phase on exit; drains or discards pending inputs |

### Chat ↔ Worker Interaction

| Action | Mechanism |
|--------|-----------|
| Start task | `dispatch_worker` tool → `spawnWorkerInSlotMap` → `launchWorkerGoroutineForSlotMap` |
| Inject message | `dispatch_worker` on running worker → `spawnOrInjectWorker` → `inputCh` send |
| Stop task | `stop_worker` tool → `chatSlotMap.stopAll` / `stopByTaskID` (`intentionalCancelToken` + `cancel()`) |
| Read status | `snapshotWorker` → `workerSnapshot.StatusSummary()` (includes recent tool progress) |

---

## Slot State Machine

Per-chat state tracked in `sessionSlot` (stored in `activeSlots` sync.Map):

```
idle ──(new task)──▶ running ──(task completes)──▶ idle
                        │
                        ├──(await_user_input)──▶ awaitingInput ──(user reply)──▶ running
                        └──(stop_worker / /stop / /new)──▶ idle
```

**Slot fields:**
- `phase` — idle / running / awaitingInput
- `inputCh` — user input channel (non-nil when running)
- `taskCancel` — context cancel func for the running task
- `taskToken` / `intentionalCancelToken` — prevents stale cancellation side effects
- `recentProgress` — ring buffer (max 8) of tool event descriptions
- `sessionID` / `lastSessionID` — session continuity across turns

---

## Key Types

| Type | File | Role |
|------|------|------|
| `sessionSlot` | `gateway.go` | per-chat mutable state (phase, inputCh, cancel, progress) |
| `workerSnapshot` | `worker_snapshot.go` | read-only point-in-time copy for Chat LLM context |
| `slotProgressRecorder` | `slot_progress_recorder.go` | `EventListener` decorator: tool events → slot ring buffer |
| `launchWorkerGoroutine` | `gateway_handlers.go` | shared goroutine launcher (used by both handleMessage and spawnWorker) |
| `incomingMessage` | `gateway_handlers.go` | parsed Lark message fields |

---

## Event Listener Chain

Configured in `setupListeners` (`task_manager_exec.go`):

```
base eventListener (gateway-level)
  └─ slotProgressRecorder (tool events → slot.recentProgress)
       └─ progressListener (tool progress → Lark message, if ShowToolProgress)
            └─ backgroundProgressListener (periodic progress updates)
                 └─ planClarifyListener (plan review messages)
                      └─ preanalysisEmojiReactionListener (emoji feedback)
                           └─ toolFailureGuardListener (abort after N failures)
```

---

## Configuration

| Config | Default | Description |
|--------|---------|-------------|
| `ConversationProcessEnabled` | `false` | Enable chat+worker mode |
| `BtwEnabled` | `false` | Enable btw fork mode (legacy path, non-conversation) |
| `ShowToolProgress` | `false` | Show per-tool progress messages in chat |
| `BackgroundProgressEnabled` | `true` | Periodic background progress notifications |
| `SlowProgressSummaryEnabled` | `true` | Summarize slow tasks |
| `ToolFailureAbortThreshold` | `6` | Abort task after N consecutive tool failures |
| `ActiveSlotTTL` | `6h` | Stale slot cleanup threshold |
