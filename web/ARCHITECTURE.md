# ALEX Web Frontend - Architecture Overview

Visual guide to the frontend architecture and data flow.

---

## System Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     Browser (User)                            │
│  ┌────────────────────────────────────────────────────────┐  │
│  │               ALEX Web Frontend (Next.js)              │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │              Pages (App Router)                   │  │  │
│  │  │  - Home (/)                                       │  │  │
│  │  │  - Sessions (/sessions)                           │  │  │
│  │  │  - Session Details (/sessions/[id])               │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │           Components Layer                        │  │  │
│  │  │  - TaskInput                                      │  │  │
│  │  │  - AgentOutput                                    │  │  │
│  │  │  - ToolCallCard                                   │  │  │
│  │  │  - SessionList                                    │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │              Hooks Layer                          │  │  │
│  │  │  - useSSE (SSE connection)                        │  │  │
│  │  │  - useTaskExecution (React Query)                 │  │  │
│  │  │  - useSessionStore (Zustand)                      │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │              Library Layer                        │  │  │
│  │  │  - API Client (fetch wrapper)                     │  │  │
│  │  │  - Type Definitions (TypeScript)                  │  │  │
│  │  │  - Utilities (formatting, styling)                │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
                            ↕ HTTP / SSE
┌──────────────────────────────────────────────────────────────┐
│                   ALEX Backend (Go)                           │
│  ┌────────────────────────────────────────────────────────┐  │
│  │               HTTP/SSE API Layer                       │  │
│  │  - POST /api/tasks                                     │  │
│  │  - GET /api/sessions                                   │  │
│  │  - GET /api/sse?session_id=xxx                         │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           ReactEngine + EventSystem                    │  │
│  │  (internal/agent/domain/)                              │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

---

## Data Flow Diagram

### Task Execution Flow

```
User Input → TaskInput Component
     ↓
useTaskExecution Hook (React Query)
     ↓
API Client: POST /api/tasks
     ↓
Backend: Create Task + Session
     ↓
Response: { task_id, session_id }
     ↓
useSSE Hook: Connect to /api/sse?session_id=xxx
     ↓
EventSource (Browser API)
     ↓
Backend: Stream Events
     ↓
useSSE: Receive Events
     ↓
AgentOutput: Render Event Cards
     ↓
User sees real-time progress
```

### Event Stream Flow

```
Backend Event Emitted
     ↓
EventSource receives event
     ↓
useSSE processes event
     ↓
Add to events array
     ↓
React re-renders AgentOutput
     ↓
Route to appropriate card:
  - task_analysis → TaskAnalysisCard
  - thinking → ThinkingIndicator
  - tool_call_start → ToolCallCard (running)
  - tool_call_complete → ToolCallCard (complete)
  - task_complete → TaskCompleteCard
  - error → ErrorCard
     ↓
Auto-scroll to bottom
```

---

## Component Hierarchy

```
App
└── RootLayout
    ├── Header (Navigation)
    └── Main Content
        ├── HomePage (/)
        │   ├── Hero Section
        │   ├── TaskInput
        │   │   └── Textarea + Submit Button
        │   └── AgentOutput
        │       ├── ConnectionStatus
        │       └── Event Cards
        │           ├── TaskAnalysisCard
        │           ├── ThinkingIndicator
        │           ├── ToolCallCard
        │           ├── TaskCompleteCard
        │           └── ErrorCard
        │
        ├── SessionsPage (/sessions)
        │   ├── Header + New Session Button
        │   └── SessionList
        │       └── SessionCard (multiple)
        │           ├── Session Info
        │           └── Actions (Fork, Delete)
        │
        └── SessionDetailsPage (/sessions/[id])
            ├── Back Button
            ├── Session Info Card
            ├── TaskInput
            ├── AgentOutput (same as HomePage)
            └── Task History
```

---

## State Management Architecture

### Global State (Zustand)

```typescript
// useSessionStore
{
  currentSessionId: string | null,
  sessionHistory: string[],
  setCurrentSession: (id) => void,
  clearCurrentSession: () => void,
  addToHistory: (id) => void
}
```

Persisted to localStorage as `alex-session-storage`.

### Server State (React Query)

```typescript
// Query Keys
['tasks']              // All tasks
['task', taskId]       // Single task
['sessions']           // All sessions
['session', sessionId] // Single session

// Mutations
createTask()           // POST /api/tasks
cancelTask()           // POST /api/tasks/:id/cancel
deleteSession()        // DELETE /api/sessions/:id
forkSession()          // POST /api/sessions/:id/fork
```

### Local State (useState)

```typescript
// Component-level state
const [events, setEvents] = useState<AnyAgentEvent[]>([]);
const [isConnected, setIsConnected] = useState(false);
const [task, setTask] = useState('');
```

---

## SSE Connection Lifecycle

```
Component Mount
     ↓
useSSE(sessionId)
     ↓
Create EventSource
     ↓
Register event listeners:
  - onopen
  - task_analysis
  - thinking
  - tool_call_start
  - tool_call_complete
  - task_complete
  - error
  - onerror
     ↓
Connected (isConnected = true)
     ↓
Receive events → update state
     ↓
On Error:
  ├── Close connection
  ├── Wait (exponential backoff)
  ├── Retry (max 5 attempts)
  └── If max reached: show error
     ↓
Component Unmount
     ↓
Close EventSource
     ↓
Cleanup
```

---

## API Integration Points

### REST API Calls

| Method | Endpoint | Purpose | Hook |
|--------|----------|---------|------|
| POST | `/api/tasks` | Create task | useTaskExecution |
| GET | `/api/tasks/:id` | Get status | useTaskStatus |
| POST | `/api/tasks/:id/cancel` | Cancel task | useCancelTask |
| GET | `/api/sessions` | List sessions | useSessions |
| GET | `/api/sessions/:id` | Get details | useSessionDetails |
| DELETE | `/api/sessions/:id` | Delete session | useDeleteSession |
| POST | `/api/sessions/:id/fork` | Fork session | useForkSession |

### SSE Connection

```javascript
// Create connection
const eventSource = new EventSource(
  `${API_URL}/api/sse?session_id=${sessionId}`
);

// Listen to events
eventSource.addEventListener('tool_call_start', (e) => {
  const event = JSON.parse(e.data);
  // Handle event
});

// Handle errors
eventSource.onerror = (err) => {
  // Reconnect logic
};
```

---

## Type System Flow

```
Go Events (internal/agent/domain/events.go)
     ↓
JSON over HTTP/SSE
     ↓
TypeScript Types (lib/types.ts)
     ↓
React Components
```

### Type Mapping Example

**Go:**
```go
type TaskAnalysisEvent struct {
    BaseEvent
    ActionName string
    Goal       string
}
```

**TypeScript:**
```typescript
interface TaskAnalysisEvent extends AgentEvent {
  event_type: 'task_analysis';
  action_name: string;
  goal: string;
  timestamp: string;
  agent_level: 'core' | 'subagent';
}
```

---

## Styling Architecture

### Tailwind Configuration

```
tailwind.config.ts (Design tokens)
     ↓
globals.css (Base styles + Custom CSS)
     ↓
Components (Tailwind classes)
     ↓
lib/utils.ts (cn() for class merging)
```

### Color System

| Category | Color | Hex | Usage |
|----------|-------|-----|-------|
| Primary | Blue | #2563eb | Links, buttons |
| Success | Green | #16a34a | Success states |
| Warning | Yellow | #eab308 | Warnings |
| Error | Red | #dc2626 | Errors |
| Muted | Gray | #6b7280 | Secondary text |

### Tool Colors

| Tool Type | Color | Border | Background |
|-----------|-------|--------|------------|
| File | Blue | border-blue-200 | bg-blue-50 |
| Shell | Purple | border-purple-200 | bg-purple-50 |
| Search | Green | border-green-200 | bg-green-50 |
| Web | Orange | border-orange-200 | bg-orange-50 |
| Think | Gray | border-gray-200 | bg-gray-50 |
| Task | Cyan | border-cyan-200 | bg-cyan-50 |

---

## Error Handling Flow

```
Error Occurs
     ↓
Where?
  ├── API Call → APIError
  │   ├── Network error (fetch failed)
  │   ├── HTTP error (4xx, 5xx)
  │   └── Parse error (invalid JSON)
  │
  ├── SSE Connection → Connection error
  │   ├── onerror event
  │   ├── Retry with backoff
  │   └── Max retries reached
  │
  └── Component Error → Error boundary
      └── Display error message
     ↓
User sees error
  ├── Toast notification
  ├── Error card
  └── Retry button (if recoverable)
```

---

## Performance Optimizations

### React Query Caching

```typescript
// Cache configuration
{
  staleTime: 60 * 1000,  // 1 minute
  retry: 1,              // Retry once on failure
}
```

### Event Buffering

```typescript
// SSE events buffered in useState
const [events, setEvents] = useState<AnyAgentEvent[]>([]);

// Batch updates to prevent excessive re-renders
setEvents((prev) => [...prev, newEvent]);
```

### Auto-scroll Optimization

```typescript
// Ref to bottom element
const bottomRef = useRef<HTMLDivElement>(null);

// Smooth scroll on new events
useEffect(() => {
  bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
}, [events]);
```

---

## Security Considerations

### API Security

- CORS validation on backend
- Environment-based API URL
- No sensitive data in localStorage
- CSP headers (recommended)

### SSE Security

- Session ID validation
- Connection timeout
- Max reconnect attempts
- Rate limiting (backend)

### Input Validation

- Task input sanitization
- XSS prevention (React handles)
- No eval() or innerHTML
- Markdown renderer (safe mode)

---

## Build & Deployment

### Development

```bash
npm run dev → Next.js dev server
  ↓
Hot reload enabled
  ↓
TypeScript checking
  ↓
Tailwind JIT compilation
  ↓
localhost:3000
```

### Production

```bash
npm run build → Next.js production build
  ↓
TypeScript compilation
  ↓
Code splitting
  ↓
CSS optimization
  ↓
Static optimization
  ↓
.next/ output
  ↓
npm start → Production server
```

---

## Monitoring Points

### Frontend Metrics

- Page load time
- Time to interactive
- Core Web Vitals
- Bundle size
- API response times
- SSE connection uptime

### Error Tracking

- API failures
- SSE disconnections
- Component errors
- Network timeouts
- Parse errors

### User Actions

- Task submissions
- Session creations
- Page views
- Button clicks
- Error occurrences

---

## Future Architecture Plans

### Phase 1: Authentication Layer

```
Add auth middleware
     ↓
JWT token management
     ↓
Protected routes
     ↓
User-specific sessions
```

### Phase 2: WebSocket Support

```
Upgrade SSE → WebSocket
     ↓
Bidirectional communication
     ↓
Real-time collaboration
     ↓
Live cursor sharing
```

### Phase 3: Offline Support

```
Service Worker
     ↓
IndexedDB caching
     ↓
Offline queue
     ↓
Sync on reconnect
```

---

## Conclusion

The ALEX Web Frontend follows a clean, layered architecture:

1. **Pages** - Route handling (App Router)
2. **Components** - UI elements (reusable)
3. **Hooks** - Business logic (custom)
4. **Library** - Core utilities (API, types, utils)

This architecture enables:
- Type safety
- Code reusability
- Easy testing
- Performance
- Scalability

---

**Architecture Version**: 1.0
**Last Updated**: 2025-10-02
**Status**: Production Ready
