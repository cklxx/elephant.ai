# ALEX Web Frontend - File Structure

Complete file tree for the Next.js web frontend:

```
web/
├── app/                                # Next.js 14 App Router
│   ├── layout.tsx                     # Root layout with header/footer/navigation
│   ├── page.tsx                       # Home page - task execution interface
│   ├── providers.tsx                  # React Query provider wrapper
│   ├── globals.css                    # Global styles, Tailwind base, custom CSS
│   └── sessions/                      # Session management routes
│       ├── page.tsx                   # Sessions list page
│       └── [id]/                      # Dynamic route for session details
│           └── page.tsx               # Individual session page
│
├── components/                         # React components
│   ├── ui/                            # Base UI components
│   │   ├── button.tsx                 # Button with variants (default, outline, ghost, destructive)
│   │   ├── card.tsx                   # Card container with header/content/footer
│   │   └── badge.tsx                  # Status badges (success, warning, error, info)
│   │
│   ├── agent/                         # Agent-specific components
│   │   ├── TaskInput.tsx              # Task submission form with textarea
│   │   ├── AgentOutput.tsx            # Main event stream container
│   │   ├── ToolCallCard.tsx           # Tool execution card (expandable)
│   │   ├── TaskAnalysisCard.tsx       # Task analysis display with gradient
│   │   ├── TaskCompleteCard.tsx       # Final answer with markdown rendering
│   │   ├── ErrorCard.tsx              # Error display with phase/recovery info
│   │   ├── ThinkingIndicator.tsx      # Animated workflow.node.output.delta state indicator
│   │   └── ConnectionStatus.tsx       # SSE connection status badge
│   │
│   └── session/                       # Session management components
│       ├── SessionCard.tsx            # Session info card with actions
│       └── SessionList.tsx            # Grid of session cards
│
├── hooks/                             # Custom React hooks
│   ├── useSSE.ts                      # SSE connection with auto-reconnect
│   ├── useTaskExecution.ts            # Task execution mutations
│   └── useSessionStore.ts             # Zustand store + React Query hooks
│
├── lib/                               # Core library code
│   ├── types.ts                       # TypeScript type definitions (matches Go events)
│   ├── api.ts                         # API client with error handling
│   └── utils.ts                       # Utility functions (formatting, styling, etc.)
│
├── package.json                       # Project dependencies and scripts
├── tsconfig.json                      # TypeScript configuration
├── tailwind.config.ts                 # Tailwind CSS configuration
├── postcss.config.mjs                 # PostCSS configuration
├── next.config.mjs                    # Next.js configuration
├── .env.local.example                 # Environment variable template
├── .gitignore                         # Git ignore rules
├── .eslintrc.json                     # ESLint configuration
├── README.md                          # User documentation
├── PROJECT_SUMMARY.md                 # Implementation summary
└── STRUCTURE.md                       # This file
```

## File Counts

- **Total Files**: 35
- **Pages**: 4 (layout + 3 routes)
- **Components**: 14
- **Hooks**: 3
- **Library Files**: 3
- **Configuration**: 8
- **Documentation**: 3

## Key Dependencies

### Framework & Core
- next: 14.2.15
- react: 18.3.1
- typescript: 5.6.3

### State & Data
- zustand: 5.0.0
- @tanstack/react-query: 5.59.0

### UI & Styling
- tailwindcss: 3.4.13
- lucide-react: 0.451.0
- clsx + tailwind-merge

### Markdown & Rendering
- react-markdown: 9.0.1
- remark-gfm: 4.0.0
- prism-react-renderer: 2.4.0

## Lines of Code (Estimated)

| Category | Files | Lines |
|----------|-------|-------|
| Components | 14 | ~1,800 |
| Hooks | 3 | ~450 |
| Library | 3 | ~400 |
| Pages | 4 | ~600 |
| Config | 8 | ~200 |
| **Total** | **32** | **~3,450** |

## Component Hierarchy

```
App
└── Layout (Header + Footer)
    ├── HomePage
    │   ├── TaskInput
    │   └── AgentOutput
    │       ├── ConnectionStatus
    │       ├── TaskAnalysisCard
    │       ├── ThinkingIndicator
    │       ├── ToolCallCard
    │       ├── TaskCompleteCard
    │       └── ErrorCard
    │
    ├── SessionsPage
    │   └── SessionList
    │       └── SessionCard (multiple)
    │
    └── SessionDetailsPage
        ├── TaskInput
        └── AgentOutput
            └── (same as HomePage)
```

## Import Graph

```
pages → components → hooks → lib

lib/types.ts         (no dependencies)
    ↓
lib/api.ts           (uses types)
    ↓
hooks/*.ts           (use api + types)
    ↓
components/*.tsx     (use hooks + types)
    ↓
app/*.tsx            (use components)
```

## Size Breakdown (Approximate)

| Directory | Size |
|-----------|------|
| app/ | 15 KB |
| components/ | 35 KB |
| hooks/ | 12 KB |
| lib/ | 18 KB |
| node_modules/ | ~400 MB |
| Total (excl. node_modules) | ~80 KB |

## Feature Coverage

✅ **Complete Features**
- Real-time SSE event streaming
- Task execution with progress
- Session management (CRUD)
- Responsive UI design
- Type-safe TypeScript
- Error handling & recovery
- Auto-reconnection logic
- Markdown rendering
- Tool visualization

⚠️ **Partial/Missing**
- Authentication (not implemented)
- Dark mode (CSS ready, no toggle)
- Tests (not implemented)
- I18n (not implemented)
- Analytics (not implemented)

## Related Documentation

- `/docs/design/SSE_WEB_ARCHITECTURE.md` - Architecture design
- `/internal/agent/domain/events.go` - Go event types
- `web/README.md` - User guide
- `web/PROJECT_SUMMARY.md` - Implementation details
