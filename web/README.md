# ALEX Web Frontend

Modern Next.js web interface for the ALEX AI Programming Agent.

## Features

- Real-time event streaming via Server-Sent Events (SSE)
- Task execution with live progress updates
- Session management (view, fork, delete)
- Responsive design with Tailwind CSS
- Type-safe TypeScript implementation
- Automatic reconnection with exponential backoff
- Tool call visualization with color-coded cards
- Markdown rendering for final answers

## Tech Stack

- **Framework**: Next.js 14 (App Router)
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **State Management**: Zustand
- **Data Fetching**: TanStack Query (React Query)
- **Markdown**: react-markdown + remark-gfm
- **Icons**: lucide-react

## Getting Started

### Prerequisites

- Node.js 20+ and npm
- ALEX backend server running (see main README)

### Installation

1. Install dependencies:

```bash
npm install
```

2. Configure environment variables:

```bash
cp .env.local.example .env.local
```

Edit `.env.local` and set:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### Development

Run the development server:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

### Production Build

```bash
npm run build
npm start
```

## Project Structure

```
web/
├── app/                        # Next.js App Router pages
│   ├── layout.tsx             # Root layout with navigation
│   ├── page.tsx               # Home page (task execution)
│   ├── sessions/
│   │   ├── page.tsx           # Sessions list
│   │   └── [id]/page.tsx      # Session details
│   ├── providers.tsx          # React Query provider
│   └── globals.css            # Global styles
├── components/
│   ├── agent/                 # Agent-specific components
│   │   ├── TaskInput.tsx      # Task input form
│   │   ├── AgentOutput.tsx    # Event stream display
│   │   ├── ToolCallCard.tsx   # Tool execution card
│   │   ├── TaskAnalysisCard.tsx
│   │   ├── TaskCompleteCard.tsx
│   │   ├── ErrorCard.tsx
│   │   ├── ThinkingIndicator.tsx
│   │   └── ConnectionStatus.tsx
│   ├── session/               # Session management components
│   │   ├── SessionList.tsx
│   │   └── SessionCard.tsx
│   └── ui/                    # Base UI components
│       ├── button.tsx
│       ├── card.tsx
│       └── badge.tsx
├── hooks/
│   ├── useSSE.ts              # SSE connection hook
│   ├── useTaskExecution.ts    # Task execution hook
│   └── useSessionStore.ts     # Session state management
├── lib/
│   ├── types.ts               # TypeScript type definitions
│   ├── api.ts                 # API client
│   └── utils.ts               # Utility functions
└── package.json
```

## Key Components

### SSE Connection Hook

The `useSSE` hook manages Server-Sent Events connections with automatic reconnection:

```typescript
const { events, isConnected, isReconnecting, error } = useSSE(sessionId);
```

Features:
- Automatic reconnection with exponential backoff
- Event buffering
- Connection status tracking
- Max retry limit (default: 5)

### Task Execution

Submit tasks to the ALEX agent:

```typescript
const { mutate: executeTask, isPending } = useTaskExecution();

executeTask({ task: 'Add dark mode toggle', session_id: sessionId });
```

### Event Stream Display

The `AgentOutput` component renders events in real-time:

- Task analysis with goals
- Tool calls with arguments and results
- Thinking indicators
- Task completion with final answers
- Error displays

## Event Types

All events correspond to Go types in `internal/agent/domain/events.go`:

- `task_analysis` - Initial task analysis
- `iteration_start` - ReAct iteration start
- `thinking` - LLM is generating response
- `think_complete` - LLM response received
- `tool_call_start` - Tool execution begins
- `tool_call_stream` - Streaming tool output
- `tool_call_complete` - Tool execution complete
- `iteration_complete` - Iteration complete
- `task_complete` - Task finished
- `error` - Error occurred

## Tool Icons and Colors

Tools use the unified console palette:

- **File & web operations** (primary accent): file_read, file_write, file_edit, web_search, web_fetch
- **Shell execution** (amber accent): bash, code_execute
- **Indexing & organization** (primary accent): grep, ripgrep, find, todo_read, todo_update
- **Think** (muted neutral): think

## API Integration

The frontend communicates with the ALEX backend via:

- **REST API**: Task and session management
- **SSE**: Real-time event streaming

### API Endpoints

- `POST /api/tasks` - Create and execute task
- `GET /api/tasks/:id` - Get task status
- `POST /api/tasks/:id/cancel` - Cancel task
- `GET /api/sessions` - List sessions
- `GET /api/sessions/:id` - Get session details
- `DELETE /api/sessions/:id` - Delete session
- `POST /api/sessions/:id/fork` - Fork session
- `GET /api/sse?session_id=xxx` - SSE event stream

## Styling

The project uses Tailwind CSS with custom design tokens:

- Color scheme matches CLI output colors
- Responsive breakpoints for mobile/tablet/desktop
- Custom scrollbar styles
- Markdown prose styles

## Error Handling

- Network errors show user-friendly messages
- SSE disconnections trigger automatic reconnection
- Failed API calls display error alerts
- Recoverable vs non-recoverable errors indicated

## Performance Optimizations

- React Query caching for API responses
- Automatic scroll to latest event
- Debounced event updates
- Optimistic UI updates for actions

## Contributing

Follow the main ALEX project contribution guidelines.

## License

See main ALEX project LICENSE.
