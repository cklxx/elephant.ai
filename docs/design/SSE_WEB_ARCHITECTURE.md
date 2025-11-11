# ALEX SSE Service Architecture & Web Interface Design

## 目录

1. [架构概览](#架构概览)
2. [SSE 服务设计](#sse-服务设计)
3. [Web 前端设计 (Next.js)](#web-前端设计)
4. [数据流与事件系统](#数据流与事件系统)
5. [实现细节](#实现细节)
6. [部署方案](#部署方案)

---

## 架构概览

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      Web Browser (Next.js)                   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   UI Layer  │  │ Event Stream │  │  State Manager   │   │
│  │  (React)    │  │   (SSE)      │  │  (Zustand/Jotai) │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            ↕ SSE / HTTP
┌─────────────────────────────────────────────────────────────┐
│                   ALEX SSE Server (Go)                       │
│  ┌─────────────────────────────────────────────────────────┤
│  │              HTTP/SSE API Layer                          │
│  │  ┌──────────┐  ┌──────────┐  ┌─────────────────────┐   │
│  │  │   SSE    │  │   REST   │  │   WebSocket (opt)   │   │
│  │  │ Handler  │  │  Handler │  │      Handler        │   │
│  │  └──────────┘  └──────────┘  └─────────────────────┘   │
│  └─────────────────────────────────────────────────────────┤
│  │            Service Layer (Application)                   │
│  │  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │  │ Session Manager │  │  Event Broadcasting Service │  │
│  │  └─────────────────┘  └─────────────────────────────┘  │
│  │  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │  │ Task Dispatcher │  │   Auth & Rate Limiter       │  │
│  │  └─────────────────┘  └─────────────────────────────┘  │
│  └─────────────────────────────────────────────────────────┤
│  │             Domain Layer (Business Logic)                │
│  │  ┌─────────────────────────────────────────────────────┤
│  │  │   ReactEngine   │   ToolRegistry  │  EventSystem   │ │
│  │  │  (react_engine) │   (tools/)      │  (events.go)   │ │
│  │  └─────────────────────────────────────────────────────┤
│  └─────────────────────────────────────────────────────────┤
│  │               Infrastructure Layer                        │
│  │  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │  │  LLM Clients │  │   Session   │  │   Message    │   │
│  │  │  (llm/)      │  │   Store     │  │   Queue      │   │
│  │  └──────────────┘  └─────────────┘  └──────────────┘   │
│  └─────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────┘
```

---

## SSE 服务设计

### 1. 核心架构原则

遵循 ALEX 现有的**六边形架构 (Hexagonal Architecture)**：

```
Domain (Pure Logic) - ReactEngine, EventSystem
    ↓ depends on
Ports (Interfaces) - SSEBroadcaster, SessionManager
    ↑ implemented by
Adapters (Infrastructure) - HTTP/SSE Server, Event Bus
```

### 2. SSE 服务层次结构

#### 2.1 HTTP/SSE API Layer

**位置**: `internal/server/http/`

```go
// internal/server/http/sse_handler.go
package http

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "alex/internal/agent/domain"
    "alex/internal/server/ports"
)

type SSEHandler struct {
    broadcaster ports.SSEBroadcaster
    sessionMgr  ports.ServerSessionManager
}

// HandleSSEStream handles SSE connection for real-time event streaming
func (h *SSEHandler) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    sessionID := r.URL.Query().Get("session_id")
    if sessionID == "" {
        http.Error(w, "session_id required", http.StatusBadRequest)
        return
    }

    // Create event channel for this client
    clientChan := make(chan domain.AgentEvent, 100)

    // Register client with broadcaster
    h.broadcaster.RegisterClient(sessionID, clientChan)
    defer h.broadcaster.UnregisterClient(sessionID, clientChan)

    // Stream events
    flusher, _ := w.(http.Flusher)
    for {
        select {
        case event := <-clientChan:
            // Serialize event to SSE format
            data := h.serializeEvent(event)
            fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.EventType(), data)
            flusher.Flush()

        case <-r.Context().Done():
            return
        }
    }
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event domain.AgentEvent) string {
    // Use existing event types from domain/events.go
    // Serialize to JSON
}
```

#### 2.2 REST API Endpoints

**位置**: `internal/server/http/api_handler.go`

```go
// API Endpoints
// POST   /api/tasks              - Create and execute new task
// GET    /api/tasks/:id          - Get task status
// POST   /api/tasks/:id/cancel   - Cancel running task
// GET    /api/sessions           - List sessions
// GET    /api/sessions/:id       - Get session details
// DELETE /api/sessions/:id       - Delete session
// POST   /api/sessions/:id/fork  - Fork session to new branch
```

#### 2.3 Event Broadcasting Service

**位置**: `internal/server/app/event_broadcaster.go`

```go
// internal/server/app/event_broadcaster.go
package app

import (
    "sync"
    "alex/internal/agent/domain"
)

type EventBroadcaster struct {
    // sessionID -> []clientChannels
    clients map[string][]chan domain.AgentEvent
    mu      sync.RWMutex
}

func NewEventBroadcaster() *EventBroadcaster {
    return &EventBroadcaster{
        clients: make(map[string][]chan domain.AgentEvent),
    }
}

// Implements domain.EventListener
func (b *EventBroadcaster) OnEvent(event domain.AgentEvent) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    // Extract session ID from event context
    sessionID := extractSessionID(event)

    // Broadcast to all clients subscribed to this session
    if clients, ok := b.clients[sessionID]; ok {
        for _, ch := range clients {
            select {
            case ch <- event:
            default:
                // Client buffer full, skip
            }
        }
    }
}

func (b *EventBroadcaster) RegisterClient(sessionID string, ch chan domain.AgentEvent) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.clients[sessionID] = append(b.clients[sessionID], ch)
}

func (b *EventBroadcaster) UnregisterClient(sessionID string, ch chan domain.AgentEvent) {
    b.mu.Lock()
    defer b.mu.Unlock()

    clients := b.clients[sessionID]
    for i, client := range clients {
        if client == ch {
            b.clients[sessionID] = append(clients[:i], clients[i+1:]...)
            close(ch)
            break
        }
    }
}
```

#### 2.4 Server Coordinator

**位置**: `internal/server/app/server_coordinator.go`

```go
// internal/server/app/server_coordinator.go
package app

import (
    "context"
    "alex/internal/agent/app"
    "alex/internal/agent/domain"
)

type ServerCoordinator struct {
    agentCoordinator *app.AgentCoordinator
    broadcaster      *EventBroadcaster
    sessionStore     ports.SessionStore
}

// ExecuteTaskAsync executes task asynchronously and streams events via SSE
func (s *ServerCoordinator) ExecuteTaskAsync(ctx context.Context, task string, sessionID string) error {
    // Set broadcaster as event listener
    _, err := s.agentCoordinator.ExecuteTask(ctx, task, sessionID, s.broadcaster)
    return err
}
```

### 3. 项目目录结构

```
internal/
├── server/                      # 新增 server 模块
│   ├── http/                    # HTTP/SSE handlers
│   │   ├── sse_handler.go
│   │   ├── api_handler.go
│   │   ├── middleware.go
│   │   └── router.go
│   ├── app/                     # Server application layer
│   │   ├── server_coordinator.go
│   │   ├── event_broadcaster.go
│   │   ├── session_manager.go
│   │   └── task_dispatcher.go
│   ├── ports/                   # Server-specific interfaces
│   │   ├── sse_broadcaster.go
│   │   └── server_session_manager.go
│   └── adapters/                # Infrastructure adapters
│       ├── redis_session.go     # Redis session store (optional)
│       └── memory_session.go    # In-memory session store
├── agent/                       # 现有 agent 模块 (不变)
│   ├── domain/
│   ├── app/
│   └── ports/
└── ...

cmd/
├── alex/                        # CLI (现有)
│   └── main.go
└── alex-server/                 # 新增 Server 入口
    └── main.go
```

### 4. Server 启动入口

**位置**: `cmd/alex-server/main.go`

```go
// cmd/alex-server/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    serverHTTP "alex/internal/server/http"
    serverApp "alex/internal/server/app"
    "alex/internal/agent/app"
    // ... other imports
)

func main() {
    // Load config
    cfg := loadConfig()

    // Initialize dependencies (reuse existing factories)
    container := initializeContainer(cfg)

    // Create server coordinator
    broadcaster := serverApp.NewEventBroadcaster()
    serverCoordinator := serverApp.NewServerCoordinator(
        container.AgentCoordinator,
        broadcaster,
        container.SessionStore,
    )

    // Setup HTTP router
    router := serverHTTP.NewRouter(serverCoordinator, broadcaster, healthChecker, nil, runtimeCfg.Environment)

    // Create HTTP server
    srv := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    // Graceful shutdown
    go func() {
        sigint := make(chan os.Signal, 1)
        signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
        <-sigint

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        srv.Shutdown(ctx)
    }()

    log.Printf("ALEX Server listening on :8080")
    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("Server error: %v", err)
    }
}
```

---

## Web 前端设计

### 1. Next.js 技术栈

```json
{
  "framework": "Next.js 14 (App Router)",
  "language": "TypeScript",
  "styling": "Tailwind CSS + shadcn/ui",
  "state": "Zustand / Jotai",
  "data-fetching": "React Query (TanStack Query)",
  "markdown": "react-markdown + remark-gfm",
  "code-highlight": "prism-react-renderer",
  "terminal": "@xterm/xterm (optional)"
}
```

### 2. 前端目录结构

```
web/                              # Next.js 项目根目录
├── app/                          # App Router
│   ├── layout.tsx
│   ├── page.tsx                  # 首页
│   ├── sessions/
│   │   ├── page.tsx              # Sessions 列表
│   │   └── [id]/
│   │       └── page.tsx          # Session 详情
│   └── api/                      # API Routes (proxy to Go server)
│       └── sse/route.ts
├── components/
│   ├── agent/
│   │   ├── TaskInput.tsx         # 任务输入框
│   │   ├── AgentOutput.tsx       # Agent 输出显示
│   │   ├── ToolCallCard.tsx      # 工具调用卡片
│   │   └── StreamingText.tsx     # 流式文本显示
│   ├── session/
│   │   ├── SessionList.tsx
│   │   └── SessionCard.tsx
│   └── ui/                        # shadcn/ui components
│       ├── button.tsx
│       ├── card.tsx
│       └── ...
├── hooks/
│   ├── useSSE.ts                 # SSE connection hook
│   ├── useTaskExecution.ts       # Task execution logic
│   └── useSessionStore.ts        # Session state management
├── lib/
│   ├── api.ts                    # API client
│   ├── sse-client.ts             # SSE client wrapper
│   └── types.ts                  # TypeScript types
└── stores/
    ├── agentStore.ts             # Zustand store for agent state
    └── sessionStore.ts           # Zustand store for sessions
```

### 3. 核心组件设计

#### 3.1 SSE Connection Hook

**文件**: `web/hooks/useSSE.ts`

```typescript
// web/hooks/useSSE.ts
import { useEffect, useRef, useState } from 'react';
import type { AgentEvent } from '@/lib/types';

export function useSSE(sessionId: string | null) {
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!sessionId) return;

    const eventSource = new EventSource(
      `/api/sse?session_id=${sessionId}`
    );

    eventSource.onopen = () => {
      setIsConnected(true);
      console.log('SSE connected');
    };

    // Listen to specific event types from domain/events.go
    const eventTypes = [
      'task_analysis',
      'iteration_start',
      'thinking',
      'think_complete',
      'tool_call_start',
      'tool_call_complete',
      'task_complete',
      'error',
    ];

    eventTypes.forEach((type) => {
      eventSource.addEventListener(type, (e) => {
        const event = JSON.parse(e.data) as AgentEvent;
        setEvents((prev) => [...prev, event]);
      });
    });

    eventSource.onerror = () => {
      setIsConnected(false);
      eventSource.close();
    };

    eventSourceRef.current = eventSource;

    return () => {
      eventSource.close();
    };
  }, [sessionId]);

  return { events, isConnected };
}
```

#### 3.2 Agent Output Component

**文件**: `web/components/agent/AgentOutput.tsx`

```tsx
// web/components/agent/AgentOutput.tsx
import { useSSE } from '@/hooks/useSSE';
import { TaskAnalysisCard } from './TaskAnalysisCard';
import { ToolCallCard } from './ToolCallCard';
import { ThinkingIndicator } from './ThinkingIndicator';

interface Props {
  sessionId: string;
}

export function AgentOutput({ sessionId }: Props) {
  const { events, isConnected } = useSSE(sessionId);

  return (
    <div className="space-y-4">
      {/* Connection status */}
      <div className="flex items-center gap-2">
        <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-gray-400'}`} />
        <span className="text-sm text-gray-600">
          {isConnected ? 'Connected' : 'Disconnected'}
        </span>
      </div>

      {/* Event stream */}
      {events.map((event, idx) => (
        <EventCard key={idx} event={event} />
      ))}
    </div>
  );
}

function EventCard({ event }: { event: AgentEvent }) {
  switch (event.event_type) {
    case 'task_analysis':
      return <TaskAnalysisCard event={event} />;

    case 'thinking':
      return <ThinkingIndicator />;

    case 'tool_call_start':
      return <ToolCallCard event={event} status="running" />;

    case 'tool_call_complete':
      return <ToolCallCard event={event} status="complete" />;

    case 'task_complete':
      return <TaskCompleteCard event={event} />;

    case 'error':
      return <ErrorCard event={event} />;

    default:
      return null;
  }
}
```

#### 3.3 Tool Call Card Component

**文件**: `web/components/agent/ToolCallCard.tsx`

```tsx
// web/components/agent/ToolCallCard.tsx
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';

interface Props {
  event: ToolCallEvent;
  status: 'running' | 'complete' | 'error';
}

export function ToolCallCard({ event, status }: Props) {
  const iconMap = {
    file_read: '📖',
    file_write: '✍️',
    bash: '🔧',
    web_search: '🔍',
    think: '💭',
  };

  return (
    <Card className="p-4">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <span className="text-2xl">{iconMap[event.tool_name] || '⚙️'}</span>
          <div>
            <p className="font-semibold">{event.tool_name}</p>
            <p className="text-sm text-gray-500">
              {status === 'running' && 'Running...'}
              {status === 'complete' && `Completed in ${event.duration}ms`}
              {status === 'error' && 'Failed'}
            </p>
          </div>
        </div>
        <Badge variant={status === 'complete' ? 'success' : 'default'}>
          {status}
        </Badge>
      </div>

      {/* Arguments */}
      {event.arguments && (
        <div className="mt-3">
          <p className="text-sm font-medium mb-1">Arguments:</p>
          <SyntaxHighlighter language="json" className="text-xs">
            {JSON.stringify(event.arguments, null, 2)}
          </SyntaxHighlighter>
        </div>
      )}

      {/* Result */}
      {event.result && (
        <div className="mt-3">
          <p className="text-sm font-medium mb-1">Result:</p>
          <pre className="bg-gray-100 p-2 rounded text-xs overflow-x-auto">
            {event.result}
          </pre>
        </div>
      )}
    </Card>
  );
}
```

#### 3.4 Task Execution Hook

**文件**: `web/hooks/useTaskExecution.ts`

```typescript
// web/hooks/useTaskExecution.ts
import { useMutation } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';

export function useTaskExecution() {
  return useMutation({
    mutationFn: async ({ task, sessionId }: { task: string; sessionId?: string }) => {
      const response = await apiClient.post('/api/tasks', {
        task,
        session_id: sessionId,
      });
      return response.data;
    },
  });
}

// Usage in component
function TaskInput() {
  const { mutate: executeTask, isPending } = useTaskExecution();

  const handleSubmit = (task: string) => {
    executeTask({ task, sessionId: currentSessionId });
  };

  return (
    <form onSubmit={(e) => {
      e.preventDefault();
      handleSubmit(taskInput);
    }}>
      <input
        type="text"
        value={taskInput}
        onChange={(e) => setTaskInput(e.target.value)}
        disabled={isPending}
      />
      <button type="submit" disabled={isPending}>
        {isPending ? 'Executing...' : 'Execute'}
      </button>
    </form>
  );
}
```

### 4. UI/UX 设计要点

#### 4.1 页面布局

```
┌─────────────────────────────────────────────────────────┐
│  Header: ALEX - AI Programming Agent                    │
│  [New Session] [Sessions] [Settings]                    │
├─────────────────────────────────────────────────────────┤
│ Sidebar          │  Main Content Area                   │
│ ┌─────────────┐  │  ┌────────────────────────────────┐ │
│ │ Sessions    │  │  │  Task Input                    │ │
│ │             │  │  │  [Enter your task...]          │ │
│ │ • Session 1 │  │  │  [Execute]                     │ │
│ │ • Session 2 │  │  └────────────────────────────────┘ │
│ │ • Session 3 │  │                                      │
│ │             │  │  Agent Output Stream                │
│ │             │  │  ┌────────────────────────────────┐ │
│ │             │  │  │ 🎯 Task Analysis               │ │
│ │             │  │  │ Analyzing repository...        │ │
│ │             │  │  └────────────────────────────────┘ │
│ │             │  │  ┌────────────────────────────────┐ │
│ │             │  │  │ 🔧 bash: ls -la               │ │
│ │             │  │  │ Status: completed (120ms)      │ │
│ │             │  │  │ Result: [files...]             │ │
│ │             │  │  └────────────────────────────────┘ │
│ │             │  │  ┌────────────────────────────────┐ │
│ │             │  │  │ 💭 Agent is thinking...        │ │
│ │             │  │  └────────────────────────────────┘ │
│ └─────────────┘  │                                      │
└─────────────────────────────────────────────────────────┘
```

#### 4.2 颜色与图标映射

```typescript
// Tool category colors (matching CLI output)
const toolColors = {
  file: 'text-blue-600',      // File operations
  shell: 'text-purple-600',   // Shell/bash
  search: 'text-green-600',   // Search/grep
  web: 'text-orange-600',     // Web search/fetch
  think: 'text-gray-600',     // Thinking/analysis
  task: 'text-cyan-600',      // Task management
};

// Event type colors
const eventColors = {
  task_analysis: 'bg-purple-50 border-purple-200',
  tool_call_start: 'bg-blue-50 border-blue-200',
  tool_call_complete: 'bg-green-50 border-green-200',
  error: 'bg-red-50 border-red-200',
};
```

---

## 数据流与事件系统

### 1. 完整事件流

```
User Input (Web)
    → HTTP POST /api/tasks
        → ServerCoordinator.ExecuteTaskAsync()
            → AgentCoordinator.ExecuteTask(ctx, task, sessionID, EventBroadcaster)
                → ReactEngine.SolveTask() [emits events]
                    → EventBroadcaster.OnEvent()
                        → Broadcast to all SSE clients
                            → SSE Stream to Browser
                                → useSSE hook receives event
                                    → State update
                                        → UI re-render
```

### 2. 事件类型映射 (Go → TypeScript)

**Go 事件定义** (`internal/agent/domain/events.go`):

```go
// Already exists in codebase
type TaskAnalysisEvent struct { ... }
type ToolCallStartEvent struct { ... }
type ToolCallCompleteEvent struct { ... }
// etc.
```

**TypeScript 类型定义** (`web/lib/types.ts`):

```typescript
// web/lib/types.ts
export interface AgentEvent {
  event_type: string;
  timestamp: string;
  agent_level: 'core' | 'subagent';
}

export interface TaskAnalysisEvent extends AgentEvent {
  event_type: 'task_analysis';
  action_name: string;
  goal: string;
}

export interface ToolCallStartEvent extends AgentEvent {
  event_type: 'tool_call_start';
  iteration: number;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
}

export interface ToolCallCompleteEvent extends AgentEvent {
  event_type: 'tool_call_complete';
  call_id: string;
  tool_name: string;
  result: string;
  error?: string;
  duration: number;
}

export interface TaskCompleteEvent extends AgentEvent {
  event_type: 'task_complete';
  final_answer: string;
  total_iterations: number;
  total_tokens: number;
  stop_reason: string;
  duration: number;
}

export type AnyAgentEvent =
  | TaskAnalysisEvent
  | ToolCallStartEvent
  | ToolCallCompleteEvent
  | TaskCompleteEvent
  | ErrorEvent;
```

---

## 实现细节

### 1. SSE 实现关键点

#### 1.1 连接保活 (Keep-Alive)

```go
// Send heartbeat every 30 seconds
ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

for {
    select {
    case event := <-clientChan:
        // Send event
    case <-ticker.C:
        // Send heartbeat comment
        fmt.Fprintf(w, ": heartbeat\n\n")
        flusher.Flush()
    case <-r.Context().Done():
        return
    }
}
```

#### 1.2 错误重连 (Frontend)

```typescript
// web/hooks/useSSE.ts
const reconnectInterval = useRef<NodeJS.Timeout>();

eventSource.onerror = () => {
  setIsConnected(false);
  eventSource.close();

  // Exponential backoff reconnection
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
  reconnectInterval.current = setTimeout(() => {
    setReconnectAttempts((prev) => prev + 1);
    // Re-establish connection
  }, delay);
};
```

### 2. 性能优化

#### 2.1 Event Batching

```go
// Batch events within 100ms window
type EventBatcher struct {
    events []domain.AgentEvent
    timer  *time.Timer
}

func (b *EventBatcher) Add(event domain.AgentEvent) {
    b.events = append(b.events, event)

    if b.timer == nil {
        b.timer = time.AfterFunc(100*time.Millisecond, b.Flush)
    }
}

func (b *EventBatcher) Flush() {
    // Send batched events as single SSE message
    // ...
}
```

#### 2.2 Frontend Virtualization

```tsx
// Use react-window for large event lists
import { FixedSizeList } from 'react-window';

<FixedSizeList
  height={600}
  itemCount={events.length}
  itemSize={100}
  width="100%"
>
  {({ index, style }) => (
    <div style={style}>
      <EventCard event={events[index]} />
    </div>
  )}
</FixedSizeList>
```

### 3. 安全性

#### 3.1 CORS 配置

```go
// internal/server/http/middleware.go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")

        // Allow specific origins in production
        allowedOrigins := []string{
            "http://localhost:3000",
            "https://alex.yourdomain.com",
        }

        if contains(allowedOrigins, origin) {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Credentials", "true")
        }

        next.ServeHTTP(w, r)
    })
}
```

#### 3.2 Rate Limiting

```go
// Use golang.org/x/time/rate
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
}

func (r *RateLimiter) GetLimiter(clientID string) *rate.Limiter {
    r.mu.Lock()
    defer r.mu.Unlock()

    if limiter, exists := r.limiters[clientID]; exists {
        return limiter
    }

    limiter := rate.NewLimiter(rate.Limit(10), 20) // 10 req/s, burst 20
    r.limiters[clientID] = limiter
    return limiter
}
```

---

## 部署方案

### 1. Docker Compose 部署

```yaml
# docker-compose.yml
version: '3.8'

services:
  alex-server:
    build:
      context: .
      dockerfile: Dockerfile.server
    ports:
      - "8080:8080"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ALEX_MODEL=${ALEX_MODEL}
      - REDIS_URL=redis:6379
    depends_on:
      - redis
    volumes:
      - ./sessions:/data/sessions

  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
    depends_on:
      - alex-server

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

volumes:
  redis_data:
```

### 2. Dockerfile

```dockerfile
# Dockerfile.server
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /alex-server ./cmd/alex-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /alex-server /usr/local/bin/
EXPOSE 8080
CMD ["alex-server"]
```

```dockerfile
# web/Dockerfile
FROM node:20-alpine AS builder

WORKDIR /app
COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
RUN npm ci --production

EXPOSE 3000
CMD ["npm", "start"]
```

### 3. Kubernetes 部署 (可选)

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alex-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: alex-server
  template:
    metadata:
      labels:
        app: alex-server
    spec:
      containers:
      - name: alex-server
        image: alex-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: alex-secrets
              key: openai-api-key
---
apiVersion: v1
kind: Service
metadata:
  name: alex-server-service
spec:
  selector:
    app: alex-server
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

---

## 总结

### 核心设计原则

1. **保持架构一致性**: 遵循现有的六边形架构，不引入不必要的复杂性
2. **事件驱动通信**: 复用现有的 `domain.AgentEvent` 系统
3. **最小侵入性**: Server 层作为独立模块，不修改 Agent 核心逻辑
4. **类型安全**: Go 和 TypeScript 类型严格对应

### 实施路径

**Phase 1: SSE Server 基础**
- [ ] 实现 `internal/server/` 模块
- [ ] 创建 `EventBroadcaster`
- [ ] 实现 SSE handler
- [ ] 编写单元测试

**Phase 2: REST API**
- [ ] 实现任务执行 API
- [ ] 实现会话管理 API
- [ ] 添加认证中间件

**Phase 3: Web 前端**
- [ ] Next.js 项目初始化
- [ ] 实现 SSE client hooks
- [ ] 构建核心 UI 组件
- [ ] 集成 API client

**Phase 4: 部署与优化**
- [ ] Docker 化
- [ ] 性能优化 (batching, virtualization)
- [ ] 监控与日志
- [ ] 生产环境部署

### 下一步行动

建议先从 **Phase 1** 开始，创建基础的 SSE server 架构，然后逐步迭代。所有实现都应该：

1. 遵循项目 `CLAUDE.md` 中的原则
2. 编写完整的单元测试
3. 保持代码简洁清晰
4. 文档完整

---

**设计文档版本**: v1.0
**创建时间**: 2025-10-02
**作者**: Claude Code
**状态**: Initial Design
