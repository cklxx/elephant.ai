# Event Display Architecture

## Problem Analysis

Current implementation has:
1. **Mixed concerns**: Event filtering, grouping, and rendering all in one component
2. **Unclear hierarchy**: Subagent threads, tool calls, node events interleaved without clear structure
3. **State management issues**: useRef for tracking rendered items causes synchronization problems
4. **Inconsistent grouping**: Subagent threads grouped by parent_task_id but not clearly presented

## Design Principles

1. **Clear Hierarchy**: User Input → Core Agent Execution → Subagent Executions
2. **Progressive Disclosure**: Show high-level progress first, details on demand
3. **Consistent Patterns**: Same lifecycle (started → running → completed) for all operations
4. **Stream-based**: Events append naturally, no complex reordering

## Event Taxonomy

### Level 1: System Events (Internal, skip display)
- `workflow.lifecycle.updated`
- `workflow.diagnostic.*`
- `workflow.node.started` (prepare node)

### Level 2: User-Facing Core Events (Main stream)
- `workflow.input.received` - User message
- `workflow.node.output.delta` - Streaming AI output
- `workflow.node.output.summary` - AI output complete
- `workflow.tool.started` → `workflow.tool.completed` - Tool execution
- `workflow.node.started/completed` - Node lifecycle (show as running state)
- `workflow.result.final` - Task complete

### Level 3: Subagent Events (Aggregated in cards)
- Same as Level 2 but `agent_level: "subagent"`
- Grouped by `parent_task_id` into cards
- Shown inside `AgentCard` component

## Component Architecture

```
ConversationEventStream
├── partitionEvents() → Categorize events into:
│   ├── mainStream: Core agent events (user facing)
│   ├── subagentGroups: Map<parentTaskId, SubagentThread[]>
│   ├── pendingTools: Running tools
│   └── pendingNodes: Running nodes
│
├── buildDisplayEntries() → Create unified timeline:
│   ├── EventEntry: { kind: "event", event }
│   ├── SubagentGroupEntry: { kind: "subagentGroup", parentTaskId, threads }
│   └── ClarifyTimelineEntry: { kind: "clarifyTimeline", groups }
│
└── Render by entry.kind:
    ├── "event" → EventLine
    ├── "subagentGroup" → SubagentCardGroup
    └── "clarifyTimeline" → ClarifyTimeline
```

## Key Decisions

### 1. Event Filtering Strategy
- **Skip**: Internal diagnostics, prepare node, tool.started (merged into completed)
- **Show**: User input, AI output, tool completed, node running state, final result
- **Aggregate**: All subagent events into cards

### 2. Tool Call Presentation
- Started + Completed = Single card with duration
- Only Started = Running state with spinner
- Progress events update the running card

### 3. Node Lifecycle Presentation
- Started + Completed = Hidden (internal)
- Only Started = "Running: node_id" with spinner
- Shows which node is currently active

### 4. Subagent Grouping
- Group by `parent_task_id` (the task that spawned subagents)
- All subagents with same parent shown together
- Positioned at earliest subagent event timestamp
- Each subagent = one AgentCard

### 5. Timeline Construction
- Events sorted by timestamp
- Subagent groups inserted at their first event time
- No complex interleaving - append as received

## Rendering Rules

### Main Stream Event Mapping
| Event Type | Display |
|------------|---------|
| `workflow.input.received` | User message bubble |
| `workflow.node.output.delta` | Streaming AI text |
| `workflow.node.output.summary` | AI message complete |
| `workflow.tool.completed` | Tool output card (with paired started) |
| `workflow.result.final` | Task completion with stats |
| Pending tool (started only) | Running spinner + tool name |
| Pending node (started only) | Running spinner + node name |

### Subagent Card Mapping
| Subagent Event | Display in Card |
|----------------|-----------------|
| `workflow.tool.completed` | Tool output |
| `workflow.result.final` | Final answer with stats |
| `workflow.node.output.summary` | Subagent summary |

## State Management

- **No useRef for rendered tracking**: Causes sync issues
- **Pure functions only**: Same input → Same output
- **Events append only**: Never modify historical display
- **Pending state in Maps**: For started-but-not-completed items

## Implementation Plan

1. **Simplify partitionEvents()**: Clear separation of concerns
2. **Create unified buildDisplayEntries()**: Single timeline construction
3. **Simplify render**: Switch on entry.kind
4. **Remove complex matching**: No more thread-to-event key matching
5. **Add tests**: Cover all event type combinations

## Files to Modify

- `ConversationEventStream.tsx` - Main component
- `EventLine/index.tsx` - Event line rendering
- Tests - Comprehensive coverage

## Migration Path

1. Keep existing tests passing
2. Refactor partitionEvents
3. Refactor buildDisplayEntries
4. Simplify render
5. Add new test cases
6. Remove deprecated code
