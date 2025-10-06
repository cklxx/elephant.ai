# Manus-Style Interaction Patterns for ALEX Web UI

## Overview

This document describes the Manus-inspired interaction patterns implemented in the ALEX web UI.

## Implemented Features

### 1. Plan Approval Flow
- ResearchPlanCard with approve/modify/cancel actions
- usePlanApproval hook for state management
- Inline plan editing with validation
- API integration for plan submission

### 2. Real-Time Timeline
- ResearchTimeline with step highlighting
- Auto-scroll to active step
- Expandable step details (tools, duration, tokens)
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
│   │   ├── ManusAgentOutput.tsx      # Main integration component
│   │   ├── ResearchPlanCard.tsx      # Plan approval UI
│   │   ├── ResearchTimeline.tsx      # Step-by-step timeline
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
- Focus indicators (ring-2, ring-blue-500)
- Keyboard shortcuts (Tab, Enter, Escape, Arrow keys)
- Screen reader support

## Performance

- VirtualizedEventList for large event streams
- useMemo for expensive computations
- Lazy loading strategy ready (see docs)
- Code splitting for Manus UI bundle

## Backend Requirements

New API endpoints needed:
- POST /api/plans/approve

New SSE events needed:
- research_plan
- step_started
- step_completed
- browser_snapshot

See full API spec in this document.
