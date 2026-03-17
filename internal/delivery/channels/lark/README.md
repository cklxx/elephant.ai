# Lark Gateway — Dual-Process Architecture

## Overview

When `ConversationProcessEnabled=true`, the Lark gateway operates in a **dual-process** model:

```
User ──▶ Chat Process ──▶ instant reply
              │
              ├── dispatch_worker ──▶ Worker Process (background)
              ├── stop_worker     ──▶ cancel Worker
              └── direct reply    ──▶ text/progress answer
```

## Chat Process (Conversation Process)

- **Entry**: `handleViaConversationProcess`
- **LLM**: lightweight, fast (~8s timeout, 300 max tokens)
- **Tools**: `dispatch_worker` (start/inject), `stop_worker` (cancel)
- **Context**: worker snapshot (phase, task desc, elapsed, recent progress) + recent chat history (5 rounds)
- **Responsibility**: instant user replies, routing tasks to Worker, stopping Worker

## Worker Process

- **Entry**: `spawnWorker` → `launchWorkerGoroutine` → `runTask`
- **Runtime**: full ReAct Agent with tool execution
- **Communication**: receives injected messages via `inputCh`
- **Progress**: tool events recorded to slot via `slotProgressRecorder`

## Interaction Model

| Chat → Worker | Mechanism |
|---|---|
| Start task | `dispatch_worker` tool → `spawnWorker` |
| Inject message | `dispatch_worker` on running worker → `inputCh` send |
| Stop task | `stop_worker` tool → `stopWorkerFromConversation` (cancel + intentionalCancelToken) |
| Read status | `snapshotWorker` → `workerSnapshot.StatusSummary()` |

## Slot State Machine

```
idle ──(dispatch_worker)──▶ running ──(task done)──▶ idle
                               │
                               ├──(await_user_input)──▶ awaitingInput ──(user reply)──▶ running
                               └──(stop_worker / /stop)──▶ idle
```

## Key Types

- **`sessionSlot`**: per-chat state — phase, inputCh, taskCancel, recentProgress ring buffer
- **`workerSnapshot`**: read-only point-in-time copy of slot state for Chat LLM context
- **`slotProgressRecorder`**: `EventListener` decorator that records tool events into slot

## Configuration

| Config | Description |
|---|---|
| `ConversationProcessEnabled` | Enable dual-process mode |
| `BtwEnabled` | Enable btw fork mode (non-conversation path) |
| `ShowToolProgress` | Show tool progress messages |
| `BackgroundProgressEnabled` | Background progress notifications |
