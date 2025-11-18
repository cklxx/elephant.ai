# Output Design - Dual Format System
> Last updated: 2025-11-18


## Overview

ALEX uses a dual-format output system to optimize information for different audiences:

1. **User Output** - Concise, readable CLI display
2. **LLM Output** - Complete, structured data for reasoning

## Architecture

```
ToolResult
├── Content (string)     → For LLM (complete)
└── Metadata (map)       → For User Display (stats)
```

### User Output (CLI Display)

**Goal**: Show essential information concisely

**Principles**:
- Dynamic progress updates for long-running operations
- Hide verbose logs, show summaries
- Emphasize actionable information
- Use visual indicators (emojis, progress bars)

**Examples**:

```bash
# Subagent - Dynamic Progress
🤖 Subagent: Running 3 tasks (max 2 parallel)
   Progress: 2/3 | Tokens: 1200 | Tool calls: 8
   ✓ Completed: 3/3 tasks | 1500 tokens | 12 tool calls

# File Read - Summary
📄 Read 245 lines
[first 500 chars preview]...

# Search - Match Count
🔍 Found 15 matches (showing first 10)
[preview of matches]
```

### LLM Output (ToolResult.Content)

**Goal**: Provide complete information for reasoning

**Principles**:
- Include ALL relevant data
- Use structured format
- No truncation
- Optimized for parsing

**Examples**:

```
Subagent completed 3/3 tasks (parallel mode)

Task 1 result:
[complete answer from subtask 1]

Task 2 result:
[complete answer from subtask 2]

Task 3 result:
[complete answer from subtask 3]
```

## Implementation

### OutputFormatter (domain layer)

```go
type OutputFormatter struct {
    verbose bool
}

// FormatForUser - Concise CLI display
func (f *OutputFormatter) FormatForUser(toolName, result string, metadata map[string]any) string

// FormatForLLM - Complete information
func (f *OutputFormatter) FormatForLLM(toolName, result string, metadata map[string]any) string
```

### Tool Result Structure

```go
type ToolResult struct {
    CallID   string         // Unique call identifier
    Content  string         // For LLM (complete)
    Error    error          // If execution failed
    Metadata map[string]any // For User Display
}
```

### Metadata Fields (for User Display)

**Subagent**:
- `total_tasks`: int
- `success_count`: int
- `failure_count`: int
- `total_tokens`: int
- `total_tool_calls`: int
- `results`: JSON array of full results

**File Operations**:
- `line_count`: int
- `file_size`: int
- `truncated`: bool

**Search**:
- `match_count`: int
- `files_searched`: int

## Subagent Specific Design

### User Display (Concurrent-Safe Output)

```bash
# Initial
🤖 Subagent: Running 5 tasks (max 3 parallel)

# During execution (each completion on new line - safe for concurrent output)
   ✓ [1/5] Task 1 | 250 tokens | 3 tools
   ✓ [2/5] Task 2 | 180 tokens | 2 tools
   ❌ [3/5] Task 3: timeout error
   ✓ [4/5] Task 4 | 320 tokens | 4 tools
   ✓ [5/5] Task 5 | 210 tokens | 3 tools

# Final summary
   ━━━ Completed: 4/5 tasks | Total: 960 tokens, 12 tool calls
```

**Note**: No dynamic line updates (`\r`) to avoid conflicts when multiple subagents run concurrently. Each event gets its own line.

### LLM Output (Concise)

Only includes:
- Task completion summary
- Individual task results (full answers)
- Errors if any

Does NOT include:
- Token counts (not relevant for LLM reasoning)
- Progress updates (real-time info)
- Implementation details

## Benefits

1. **Reduced Token Usage**: LLM only receives essential information
2. **Better UX**: Users see clean, actionable output
3. **Separation of Concerns**: Different formats for different needs
4. **Flexibility**: Easy to add tool-specific formatting

## Verbose Mode

Set `ALEX_VERBOSE=1` to see full output in CLI:

```bash
ALEX_VERBOSE=1 ./alex "your task"
```

In verbose mode:
- `FormatForUser()` returns same as `FormatForLLM()`
- All tool output shown completely
- Useful for debugging

## Future Enhancements

- [ ] Streaming user output for long operations
- [ ] Configurable output formats (JSON, plain text)
- [ ] Tool-specific user formatters
- [ ] Output filtering based on tool category
