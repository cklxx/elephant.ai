# Smart Tool Display Optimization Summary

## Problem

之前的工具输出展示采用统一策略：
- 默认只显示简短摘要
- 需要 VERBOSE 模式才能看到完整内容
- 未考虑不同工具的用户需求差异

**问题案例：**
- `code_execute`: 用户看不到执行的代码和完整输出
- `todo_update`: 用户看不到更新后的完整任务列表
- `file_read`: 显示大量内容给用户（但主要是给LLM看的）

## Solution

**核心思想：从用户视角出发，不同工具展示不同内容量**

### 工具分类策略

| 类型 | 工具 | 展示策略 | 原因 |
|------|------|----------|------|
| **完整展示** | `code_execute` | 代码 + 完整输出 | 调试和验证需要看到所有信息 |
| | `todo_read/update` | 完整任务列表 | 用户需要看到所有任务状态 |
| | `git_*` | 完整Git输出 | Git操作结果重要且通常简洁 |
| **智能摘要** | `grep/ripgrep` | 匹配数 + 前3-5条 | 大量结果时显示代表性样本 |
| | `list_files` | 文件数 + 前5-10个 | 避免长列表刷屏 |
| | `bash` | 短输出完整显示，长输出摘要 | 根据长度智能判断 |
| **仅统计** | `file_read` | 仅行数 | 内容主要供LLM分析 |
| **仅状态** | `file_write/edit` | ✓ 成功确认 | 用户只需知道操作成功 |
| | `web_search/fetch` | ✓ 完成状态 | 结果主要供LLM使用 |

## Implementation

### 核心函数

`cmd/alex/stream_output.go:180-309`

```go
func (h *StreamingOutputHandler) printSmartToolOutput(toolName, result string) {
    switch toolName {
    case "code_execute":
        // ALWAYS show full output
        h.printFullOutput("Execution Result", result, ...)

    case "todo_read", "todo_update":
        // ALWAYS show full task list
        h.printFullOutput("Task List", result, ...)

    case "grep", "ripgrep", "code_search":
        // Show count + preview
        fmt.Printf("  → %d matches\n", matchCount)
        // Show first 3-5 matches + "... and N more"

    case "file_read":
        // Just show line count
        fmt.Printf("  → %d lines read\n", lines)

    // ... 其他工具类型
    }
}
```

## Examples

### Before (统一简短摘要)

```
⏺ ▶️code_execute(language=python, ...)
  → success

⏺ 📋todo_update(...)
  → 3 tasks updated
```

### After (智能展示)

```
⏺ ▶️code_execute(language=python, code=print('Hello')...)
    Success in 25ms:
    Hello from code_execute!
    Line 0
    Line 1
    Line 2

⏺ 📋todo_update(...)
    Updated: 0 in progress, 1 pending, 2 completed (3 total)

    Pending:
      - Demo task 3: Verify todo functionality

    Recently Completed:
      - Demo task 1: Read current todo list
      - Demo task 2: Create sample tasks
```

## Benefits

### 1. 用户体验优化
- ✅ 关键信息不遗漏（代码执行、任务列表完整展示）
- ✅ 减少信息噪音（文件读取只显示行数）
- ✅ 大数据智能摘要（搜索结果分页显示）

### 2. 信息密度合理
- 重要工具：完整详细
- 中等重要：摘要+样本
- 辅助工具：仅状态/统计

### 3. 保持灵活性
- VERBOSE 模式仍然可用（`ALEX_VERBOSE=1`）
- 不同场景不同策略
- 易于扩展新工具类型

## Testing Results

### Code Execute ✅
```bash
./alex "Execute Python code: print('Hello'); for i in range(3): print(f'Line {i}')"
```
**Output:** 完整显示代码和所有输出行

### Todo Tools ✅
```bash
./alex "Read and update todo list"
```
**Output:** 完整的任务列表，包含所有状态分组

### Search Tools ✅
```bash
./alex "Search for 'func' in all go files"
```
**Output:** 103 matches，显示前3条 + "... and 100 more" 提示

### File Operations ✅
```bash
./alex "Read main.go"
```
**Output:** "→ 120 lines read" （不显示内容）

## Documentation

详细设计文档：`docs/SMART_TOOL_DISPLAY.md`

## Commit Message

```
feat: implement smart tool display based on user needs

不同工具展示不同内容量：
- code_execute/todo: 完整展示（用户需要看到所有信息）
- grep/list: 智能摘要（显示代表性样本）
- file_read: 仅统计（内容供LLM分析）
- file_write: 仅状态（确认操作成功）

核心改进：
- printSmartToolOutput() 根据工具类型选择展示策略
- printFullOutput() 格式化完整输出
- 保留 VERBOSE 模式用于详细输出

Closes #N/A
```
