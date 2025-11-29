# Research Console Interaction Patterns for ALEX Web UI

## Overview

This document describes the research console interaction patterns implemented in the ALEX web UI.

## Implemented Features

### 1. Plan Approval Flow
- Plan approval is auto-submitted via `usePlanApproval` (no dedicated UI)
- Research plans are accepted immediately to keep execution flowing
- API integration for plan submission remains in place

### 2. Real-Time Timeline
- TimelineStepList with step highlighting
- Auto-scroll to active step with manual focus override
- Visual indicators: pending → active → complete/error

### 3. Computer View
- WebViewport carousel for tool outputs
- Support for: web_fetch, bash, file operations
- Fullscreen mode for detailed inspection
- Syntax-highlighted code and terminal output

### 4. Document Canvas
- Multiple view modes: Default | Reading | Compare
- Markdown rendering with syntax highlighting
- Side-by-side comparison view
- Clean reading mode with focused content

### 5. Unified Feedback
- Toast notifications (Sonner library)
- Modal dialogs for confirmations
- No browser alerts
- Auto-dismiss after 5s (errors persist)

## File Structure

```
web/
├── components/
│   ├── agent/
│   │   ├── ConsoleAgentOutput.tsx      # Main integration component
│   │   ├── TimelineStepList.tsx         # Step-by-step timeline
│   │   ├── WebViewport.tsx           # Tool output inspector
│   │   ├── DocumentCanvas.tsx        # Document viewer
│   │   └── VirtualizedEventList.tsx  # Event stream
│   └── ui/
│       ├── tabs.tsx                  # Tab navigation
│       ├── toast.tsx                 # Toast notifications
│       └── dialog.tsx                # Modal dialogs
├── hooks/
│   ├── usePlanApproval.ts           # Plan approval logic
│   ├── useToolOutputs.ts            # Parse events to tool outputs
│   └── useTimelineSteps.ts          # Parse events to timeline
└── lib/
    ├── api.ts                       # API client with plan approval
    └── types.ts                     # TypeScript definitions
```

## Accessibility

All components are fully keyboard-accessible with:
- ARIA attributes (role, aria-label, aria-selected)
- Focus indicators (ring-2, ring-primary)
- Keyboard shortcuts (Tab, Enter, Escape, Arrow keys)
- Screen reader support

## Performance

- VirtualizedEventList for large event streams
- useMemo for expensive computations
- Lazy loading strategy ready (see docs)
- Code splitting for research console UI bundle

## Backend Requirements

New API endpoints needed:
- POST /api/plans/approve

New SSE events needed:
- research_plan
- step_started
- step_completed
- browser_info

See full API spec in this document.
