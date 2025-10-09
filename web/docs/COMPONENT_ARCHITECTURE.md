# Component Architecture Diagram

## Overview

This document provides visual diagrams of the research console-style UI architecture.

## Component Hierarchy

```
page.tsx (Home hero landing)
  └─ HomeContent
     ├─ HeroSection (CTA → /conversation)
     ├─ HighlightCards
     └─ SummaryTiles

conversation/page.tsx (Research console workspace)
  └─ ConversationPageContent
     ├─ SessionSidebar
     │   ├─ ConnectionStatus
     │   └─ SessionHistory (pinned & recent)
     ├─ ConversationStream
     │   ├─ Header (language switch + timeline status)
     │   ├─ TerminalOutput (event cards, plan approval, tool statuses)
     │   └─ TaskInput (textarea + submit button)
     └─ GuidanceSidebar
         ├─ QuickstartButtons
         └─ TimelineOverview
```

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      EVENT STREAM FLOW                           │
└─────────────────────────────────────────────────────────────────┘

SSE Connection (/api/sse)
        │
        ▼
┌──────────────────┐
│ useSSE Hook      │
│ - Connect        │
│ - Reconnect      │
│ - Parse events   │
└────────┬─────────┘
         │
         ▼
┌──────────────────────────────────────────┐
│      events: AnyAgentEvent[]             │
└────────┬─────────────────────────────────┘
         │
         ├────────────────────────┐
         │                        │
         ▼                        ▼
┌────────────────────┐   ┌────────────────────┐
│ useTimelineSteps   │   │ useToolOutputs     │
│                    │   │                    │
│ Converts events to │   │ Converts events to │
│ timeline steps     │   │ tool outputs       │
└────────┬───────────┘   └────────┬───────────┘
         │                        │
         ▼                        ▼
┌────────────────────┐   ┌────────────────────┐
│ ResearchTimeline   │   │   WebViewport      │
│                    │   │                    │
│ Displays steps     │   │ Displays outputs   │
└────────────────────┘   └────────────────────┘
```

## Plan Approval State Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                   PLAN APPROVAL STATE FLOW                       │
└─────────────────────────────────────────────────────────────────┘

        User submits task
                │
                ▼
        ┌───────────────────┐
        │ Task created      │
        │ requires_approval │
        └─────────┬─────────┘
                  │
                  ▼
        ┌───────────────────┐
        │ SSE: research_plan│
        │ event arrives     │
        └─────────┬─────────┘
                  │
                  ▼
        ┌───────────────────────────┐
        │ usePlanApproval           │
        │ state: 'awaiting_approval'│
        └─────────┬─────────────────┘
                  │
                  ▼
        ┌─────────────────────────┐
        │ ResearchPlanCard shows  │
        │ [APPROVE|MODIFY|CANCEL] │
        └─────────┬───────────────┘
                  │
    ┌─────────────┼─────────────┐
    │             │             │
APPROVE        MODIFY        CANCEL
    │             │             │
    ▼             ▼             ▼
┌────────┐  ┌──────────┐  ┌─────────┐
│POST    │  │Edit plan │  │POST     │
│approve │  │inline    │  │reject   │
└───┬────┘  └────┬─────┘  └────┬────┘
    │            │              │
    │            ▼              ▼
    │      ┌──────────┐    ┌─────────┐
    │      │POST      │    │Task     │
    │      │approve   │    │cancelled│
    │      │modified  │    └─────────┘
    │      └────┬─────┘
    │           │
    └───────────┴────────────┐
                             ▼
                    ┌──────────────────┐
                    │Execution starts  │
                    │Timeline activates│
                    └──────────────────┘
```

## Layout Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                    MANUS UI LAYOUT                               │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Connection Status Bar                                           │
│  [●] Connected | 234 events (45KB) | Session: abc123...         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Research Plan Card (if awaiting approval)                       │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Goal: Optimize database queries                          │  │
│  │ Steps: [1. Analyze, 2. Index, 3. Test]                   │  │
│  │ [APPROVE & START] [MODIFY PLAN] [CANCEL]                 │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌───────────────────────────────┬─────────────────────────────────┐
│  LEFT PANE (2/3 width)        │  RIGHT PANE (1/3 width)         │
│                               │                                 │
│  ┌─────────────────────────┐  │  ┌────────────────────────────┐ │
│  │ [Timeline][Events][Doc] │  │  │  Computer View             │ │
│  └─────────────────────────┘  │  │                            │ │
│                               │  │  ┌──────────────────────┐  │ │
│  Tab Content:                 │  │  │ web_fetch            │  │ │
│                               │  │  │ URL: example.com     │  │ │
│  Timeline View:               │  │  │ [Screenshot] [HTML]  │  │ │
│  ┌─────────────────────────┐  │  │  │                      │  │ │
│  │ ▶ Iteration 1/5         │  │  │  │ [Screenshot Preview] │  │ │
│  │   Tools: bash, file_read│  │  │  │                      │  │ │
│  │   Duration: 2.5s        │  │  │  └──────────────────────┘  │ │
│  │   Tokens: 1500          │  │  │                            │ │
│  │                         │  │  │  [◄] [1/5] [►] [⛶]         │ │
│  │ ✓ Iteration 2/5         │  │  └────────────────────────────┘ │
│  │   (collapsed)           │  │                                 │
│  │                         │  │                                 │
│  │ ⏸ Iteration 3/5         │  │                                 │
│  │   (pending)             │  │                                 │
│  └─────────────────────────┘  │                                 │
│                               │                                 │
└───────────────────────────────┴─────────────────────────────────┘
```

## Component Communication

```
┌─────────────────────────────────────────────────────────────────┐
│                  COMPONENT COMMUNICATION                         │
└─────────────────────────────────────────────────────────────────┘

Props Flow (Top-down):
─────────────────────
page.tsx
  │
  ├─► ConsoleAgentOutput
  │     │
  │     ├─► events: AnyAgentEvent[]
  │     ├─► sessionId: string
  │     ├─► taskId: string
  │     └─► isConnected: boolean
  │
  └─► ResearchPlanCard
        │
        ├─► plan: ResearchPlan
        ├─► onApprove: () => void
        ├─► onModify: (plan) => void
        └─► onReject: (reason) => void

Callback Flow (Bottom-up):
──────────────────────────
ResearchPlanCard
  │ onApprove()
  ▼
usePlanApproval
  │ handleApprove()
  ▼
API Call: POST /api/plans/approve
  │
  ▼
Toast Notification: "Plan approved"
  │
  ▼
Timeline: Execution starts

ResearchPlanCard
  │ onReject(reason)
  ▼
usePlanApproval
  │ handleReject(reason)
  ▼
Toast Notification: "Plan rejected"
  │
  ▼
Plan remains editable for revision


Hook Dependencies:
──────────────────
useSSE → events → useTimelineSteps → ResearchTimeline
                  useToolOutputs → WebViewport
                  usePlanApproval → ResearchPlanCard
```

## State Management

```
┌─────────────────────────────────────────────────────────────────┐
│                     STATE MANAGEMENT                             │
└─────────────────────────────────────────────────────────────────┘

Server State (React Query):
────────────────────────────
┌───────────────────────┐
│ useTaskExecution      │  ← POST /api/tasks
│ (mutation)            │
└───────────────────────┘

┌───────────────────────┐
│ usePlanApproval       │  ← POST /api/plans/approve
│ (mutation)            │
└───────────────────────┘

Client State (Zustand):
───────────────────────
┌───────────────────────┐
│ useSessionStore       │  ← Session history
│ - currentSessionId    │
│ - history             │
└───────────────────────┘

┌───────────────────────┐
│ useAgentStreamStore   │  ← Event stream management
│ - events              │
│ - memoryStats         │
└───────────────────────┘

Derived State (useMemo):
────────────────────────
┌───────────────────────┐
│ useTimelineSteps      │  ← Computed from events
│ - steps[]             │
└───────────────────────┘

┌───────────────────────┐
│ useToolOutputs        │  ← Computed from events
│ - outputs[]           │
└───────────────────────┘

Local State (useState):
───────────────────────
┌───────────────────────┐
│ ConsoleAgentOutput      │
│ - activeTab           │
│ - documentViewMode    │
└───────────────────────┘

┌───────────────────────┐
│ WebViewport           │
│ - currentIndex        │
│ - isFullscreen        │
└───────────────────────┘
```

## Error Handling

```
┌─────────────────────────────────────────────────────────────────┐
│                      ERROR HANDLING                              │
└─────────────────────────────────────────────────────────────────┘

API Errors:
───────────
useTaskExecution
  │ onError
  ▼
Toast.error("Failed to execute task", error.message)

SSE Connection Errors:
──────────────────────
useSSE
  │ error state
  ▼
ConnectionStatus (shows error + reconnect button)
  │
  ▼
Toast.error("Connection lost", "Attempting to reconnect...")

Plan Approval Errors:
─────────────────────
usePlanApproval
  │ onError
  ▼
Toast.error("Plan approval failed", error.message)
  │
  ▼
State resets to 'awaiting_approval' (allow retry)

Execution Errors:
─────────────────
SSE: error event
  │
  ▼
Timeline: Step marked as 'error' status
  │
  ▼
Show error details in expanded view
```

## Summary

This architecture provides:
- ✅ Clear separation of concerns
- ✅ Unidirectional data flow
- ✅ Centralized state management
- ✅ Error handling at all levels
- ✅ Type safety throughout
- ✅ Composable components
