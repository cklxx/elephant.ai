# Subagent Tool Improvements

## Overview

Enhanced the `subagent` tool to prevent misuse and recursive calls.

## Changes Made

### 1. Improved Tool Description

**Added Clear Usage Guidelines**:

```
⚠️ IMPORTANT USAGE GUIDELINES:
- ❌ DO NOT use for simple, quick tasks (file operations, single searches, basic analysis)
- ❌ DO NOT use when main agent can complete task in 1-2 iterations
- ✅ ONLY use for truly complex research requiring multiple independent investigations
- ✅ ONLY use when each subtask is substantial (>5 steps) and parallel execution saves significant time
- ✅ Each subtask should be completely independent and take >30 seconds
```

**When to Use**:
- Comprehensive research requiring multiple deep investigations
- Large-scale code analysis across multiple modules
- Parallel data gathering from different sources
- Complex comparative analysis requiring separate detailed studies

**When NOT to Use**:
- Simple file operations → Use `file_read`, `file_write`, etc.
- Single web searches → Use `web_search` directly
- Quick analysis → Use `think` or direct tools
- Tasks completable in <5 tool calls

**Examples Added**:

✅ **GOOD** (complex parallel research):
```json
{
  "subtasks": [
    "Comprehensive analysis of React 18 features, best practices, and migration guide",
    "Complete Vue 3 Composition API research with real-world examples",
    "In-depth Svelte framework study including compiler and reactivity model"
  ],
  "mode": "parallel"
}
```

❌ **BAD** (use direct tools instead):
```json
{
  "subtasks": [
    "Read README.md",        // ❌ Use file_read directly
    "List project files",    // ❌ Use list_files directly
    "Search for 'main'"      // ❌ Use grep directly
  ]
}
```

### 2. Prevented Recursive Subagent Calls

**Problem**: Subagent could call itself recursively, causing infinite loops.

**Solution**: Context-based recursion detection.

**Implementation**:

```go
// Context key for nested subagent detection
type subagentCtxKey struct{}

func isNestedSubagent(ctx context.Context) bool {
    return ctx.Value(subagentCtxKey{}) != nil
}

func markSubagentContext(ctx context.Context) context.Context {
    return context.WithValue(ctx, subagentCtxKey{}, true)
}

// In Execute():
if isNestedSubagent(ctx) {
    return &ports.ToolResult{
        CallID:  call.ID,
        Content: "Error: Subagent cannot call subagent recursively. Use direct tools instead.",
        Error:   fmt.Errorf("recursive subagent call not allowed"),
    }, nil
}

// In executeParallel/executeSerial:
ctx = markSubagentContext(ctx)  // Mark before delegating
```

**How It Works**:
1. Parent subagent marks context before delegating tasks
2. Child agent receives marked context
3. If child tries to call subagent, check fails
4. Returns clear error message instead of recursing

**Benefits**:
- Prevents infinite recursion
- Clear error message to LLM
- Guides LLM to use direct tools
- No global state needed (context-scoped)

## Files Modified

- ✅ `internal/tools/builtin/subagent.go`
  - Enhanced description (60+ lines)
  - Added recursion detection (20 lines)
  - Total changes: ~80 lines

## Testing

### Build Verification
```bash
make build  # ✅ Success
```

### Manual Test Cases

#### Test 1: Simple Task (Should NOT use subagent)
```bash
./alex "list files in directory"
```
**Expected**: Uses `list_files` directly, NOT subagent

#### Test 2: Recursive Call Prevention
```bash
# If subagent tries to call subagent
```
**Expected**:
```
Error: Subagent cannot call subagent recursively. Use direct tools instead.
```

#### Test 3: Complex Research (Appropriate use)
```bash
./alex "深度对比分析 React, Vue, Svelte 三个框架"
```
**Expected**: May use subagent for parallel research (appropriate)

## Benefits

### ✅ Prevents Misuse
- Clear guidelines discourage simple tasks
- Examples show appropriate vs inappropriate usage
- LLM understands when to use subagent

### ✅ Prevents Infinite Recursion
- Context-based detection
- No global state pollution
- Clean error messages

### ✅ Better Resource Usage
- Avoids unnecessary parallel execution
- Reduces token consumption
- Faster execution for simple tasks

### ✅ Improved LLM Decision Making
- Explicit DO/DON'T guidelines
- Concrete examples
- Clear error feedback

## Impact

**Before**:
- LLM might use subagent for simple file reads
- Risk of infinite recursion
- Unclear when to use subagent
- Wasted resources on trivial parallel tasks

**After**:
- Clear usage guidelines
- Recursion prevented
- LLM knows when subagent is appropriate
- Efficient resource usage

## Future Enhancements

### Potential Additions
1. **Cost Threshold**: Only allow subagent if estimated tokens > N
2. **Complexity Detection**: Auto-analyze if task is complex enough
3. **Usage Metrics**: Track subagent vs direct tool usage
4. **Adaptive Limits**: Adjust max_workers based on system load

---

**Status**: ✅ **COMPLETE**

**Implementation Date**: 2025-10-01

**Changes**:
- Enhanced description with clear guidelines
- Prevented recursive calls with context detection
- Build successful

**Implemented By**: Claude (with cklxx)
