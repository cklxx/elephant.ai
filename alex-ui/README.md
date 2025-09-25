# ALEX Terminal UI

A high-performance TypeScript Terminal UI for ALEX (Agile Light Easy Xpert Code Agent) built with React, Ink, and modern TypeScript patterns.

## Features

- **Stream-Native Interface**: Real-time streaming responses with visual indicators
- **Tool Call Visualization**: Live monitoring of tool execution with status, progress, and results
- **Intelligent Input**: Multi-line support, command history, and syntax highlighting
- **Responsive Layout**: Adaptive terminal UI with header, sidebar, and footer components
- **Type-Safe Architecture**: Comprehensive TypeScript coverage with Zod validation
- **State Management**: Zustand-powered stores with persistence and error handling
- **WebSocket Integration**: Full-duplex communication with automatic reconnection
- **Error Recovery**: Graceful error handling with user-friendly recovery mechanisms

## Quick Start

### Prerequisites

- Node.js 18+
- ALEX Go backend server running on localhost:8080

### Installation

```bash
# Install dependencies
npm install

# Build the project
npm run build

# Run in development mode
npm run dev

# Start the CLI
npm start
```

### Usage

```bash
# Basic usage
alex-ui

# Custom server URLs
alex-ui --api-url http://localhost:3000/api/v1 --ws-url ws://localhost:3000/ws

# Disable WebSocket (HTTP only)
alex-ui --no-websocket

# Debug mode
alex-ui --debug
```

## Architecture

### Project Structure

```
src/
├── app/                 # App root and providers
│   ├── App.tsx         # Main application component
│   └── providers/      # Context providers
├── components/         # UI components
│   ├── chat/          # Message display and streaming
│   ├── input/         # Command input and history
│   ├── layout/        # Layout components
│   └── common/        # Reusable components
├── services/          # External service integration
│   ├── api-client.ts  # HTTP API client
│   ├── websocket-client.ts # WebSocket client
│   └── connection-manager.ts # Connection orchestration
├── stores/           # Zustand state management
│   ├── app-store.ts  # Global app state
│   ├── session-store.ts # Session management
│   ├── message-store.ts # Chat and messages
│   └── ui-store.ts   # UI preferences
├── types/           # TypeScript definitions
├── utils/           # Utility functions
└── test/           # Test suites
```

### Core Components

#### MessageList & MessageItem
- Renders conversation history with role-based styling
- Supports syntax highlighting for code blocks
- Handles tool calls and results display
- Auto-scrolling and pagination

#### CommandInput
- Real-time input with cursor management
- Command history navigation (↑/↓)
- Multi-line support with Ctrl+M submit
- Input validation and sanitization

#### StreamingIndicator
- Visual feedback for AI processing phases
- Progress bars for long-running operations
- Estimated time remaining display
- Phase-specific icons and colors

#### ToolCallDisplay
- Live tool execution monitoring
- Input/output visualization
- Status tracking (pending → running → completed/failed)
- Duration and performance metrics

### State Management

#### App Store
- Connection status and error handling
- Available tools registry
- Global notifications

#### Session Store
- Session creation, loading, and persistence
- Session list management
- Configuration updates

#### Message Store
- Conversation thread management
- Tool call tracking
- Streaming state coordination
- Message sending and receiving

#### UI Store
- Input state and history
- Display preferences (timestamps, metadata, syntax highlighting)
- Layout state (sidebar, header visibility)
- Error state management

### Communication Layer

#### API Client
- RESTful HTTP client for session management
- Tool discovery and health checks
- Configurable base URL and headers
- Request/response type safety

#### WebSocket Client
- Real-time streaming communication
- Automatic reconnection with exponential backoff
- Event-based message handling
- Connection state management

#### Connection Manager
- Orchestrates API and WebSocket initialization
- Health monitoring and reconnection
- Graceful error handling and fallbacks

## Key Features Implementation

### Streaming Responses

The UI handles streaming responses in phases:

1. **Thinking**: AI analyzes the request
2. **Acting**: Tools are executed
3. **Observing**: Results are processed
4. **Responding**: Final response generation

Each phase has visual indicators, progress tracking, and estimated completion times.

### Tool Call Visualization

Tool calls are tracked through their lifecycle:

```typescript
interface ToolCall {
  id: string
  name: string
  input: Record<string, any>
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  result?: any
  error?: string
  started_at: string
  completed_at?: string
  duration_ms?: number
}
```

The UI provides:
- Real-time status updates
- Input/output display
- Error handling and retry mechanisms
- Performance metrics

### Error Handling

Multi-layered error handling:

1. **Network Errors**: Automatic retry with exponential backoff
2. **Validation Errors**: Input sanitization and user feedback
3. **API Errors**: Graceful degradation and error display
4. **UI Errors**: Error boundaries with recovery options

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+C` | Exit application |
| `Ctrl+L` | Clear screen/errors |
| `Ctrl+N` | New session |
| `Ctrl+S` | Save session |
| `Ctrl+B` | Toggle sidebar |
| `Ctrl+H` | Toggle header |
| `↑/↓` | Navigate command history |
| `Ctrl+M` | Submit multiline input |

## Development

### Scripts

```bash
npm run dev          # Development mode with hot reload
npm run build        # Production build
npm run test         # Run test suite
npm run test:ui      # Run tests with UI
npm run test:coverage # Generate coverage report
npm run lint         # ESLint
npm run lint:fix     # Fix linting issues
npm run format       # Prettier formatting
npm run typecheck    # TypeScript type checking
```

### Testing

Comprehensive test suite covering:

- **Unit Tests**: Utilities, formatters, validators
- **Store Tests**: State management logic
- **Component Tests**: UI component behavior
- **Integration Tests**: Service layer functionality

```bash
# Run all tests
npm test

# Run specific test files
npm test utils.test.ts

# Generate coverage report
npm run test:coverage
```

### Code Quality

- **TypeScript**: Strict type checking with comprehensive coverage
- **ESLint**: Code quality and consistency rules
- **Prettier**: Automatic code formatting
- **Zod**: Runtime type validation for external data
- **Vitest**: Fast unit testing with mocking support

## Integration with ALEX Backend

### Required API Endpoints

```typescript
// Session Management
POST   /api/v1/sessions          # Create session
GET    /api/v1/sessions          # List sessions
GET    /api/v1/sessions/:id      # Load session
DELETE /api/v1/sessions/:id      # Delete session

// Chat
POST   /api/v1/chat              # Send message

// Tools
GET    /api/v1/tools             # Get available tools

// Health
GET    /api/v1/health            # Health check
```

### WebSocket Events

```typescript
// Client → Server
{
  "type": "message",
  "data": {
    "message": "Your request",
    "stream": true
  }
}

// Server → Client
{
  "type": "stream_chunk",
  "data": {
    "type": "content",
    "content": "Streaming response..."
  }
}

{
  "type": "stream_chunk",
  "data": {
    "type": "tool_call",
    "tool_call": {
      "id": "call_123",
      "name": "file_read",
      "input": { "file_path": "/path/to/file" }
    }
  }
}
```

## Performance Considerations

- **Lazy Loading**: Components and stores initialized on demand
- **Memoization**: React.memo and useMemo for expensive computations
- **Virtual Scrolling**: Efficient handling of large message histories
- **Debounced Input**: Reduced API calls during typing
- **Connection Pooling**: Reused connections for HTTP requests
- **State Persistence**: Local storage for user preferences

## Accessibility

- **Keyboard Navigation**: Full keyboard accessibility
- **Screen Reader Support**: Semantic HTML and ARIA labels
- **High Contrast**: Terminal-appropriate color schemes
- **Focus Management**: Proper focus handling for modal dialogs
- **Error Announcements**: Screen reader notifications for errors

## Contributing

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/new-component`
3. **Make changes with tests**: Ensure test coverage for new features
4. **Run quality checks**: `npm run lint && npm run typecheck && npm test`
5. **Submit a pull request**: With clear description and test coverage

## License

MIT License - see LICENSE file for details.

## Roadmap

- [ ] Plugin system for custom components
- [ ] Themeable UI with color customization
- [ ] Advanced search and filtering
- [ ] Session export/import functionality
- [ ] Performance monitoring dashboard
- [ ] Offline mode support
- [ ] Mobile-responsive design
- [ ] Voice input support