# Smart Tool Display Strategy

## Overview

智能工具展示系统基于用户视角，为不同类型的工具提供最优的输出展示策略。

## Design Philosophy

**从用户需求出发，每个工具展示恰当的信息量：**

- 用户需要看到的内容 → 完整展示
- 用户只需要确认的操作 → 简要状态
- 大量数据结果 → 智能摘要 + 可选详细输出

## Tool Display Strategies

### 1. Full Output Tools (完整展示)

**适用场景：** 用户需要看到完整内容才能理解结果

#### code_execute
```
⏺ ▶️code_execute(language=python, code=print('Hello')...)
    Success in 25ms:
    Hello from code_execute!
    Line 0
    Line 1
    Line 2
```

**理由：** 代码执行的完整输出对调试和验证至关重要

#### todo_read / todo_update
```
⏺ 📋todo_read()
    Todo List:

    # Task List

    ## In Progress
    - [▶] Task 1

    ## Pending
    - [ ] Task 2
    - [ ] Task 3

    3 tasks
```

**理由：** 任务列表需要完整展示，用户需要看到所有任务状态

### 2. Summary with Preview (摘要+预览)

**适用场景：** 结果数量多，展示代表性样本

#### grep / ripgrep / code_search / find
```
⏺ 🔍grep(pattern=func, path=.)
  → 103 matches
    ./main.go:10:func main() {
    ./utils.go:5:func helper() {
    ./service.go:20:func NewService() {
    ... and 100 more (use ALEX_VERBOSE=1 for full output)
```

**展示规则：**
- ≤ 5 matches: 全部显示
- 6-10 matches: 显示前5个 + "... and N more"
- > 10 matches: 显示前3个 + 提示使用VERBOSE模式

#### list_files
```
⏺ 📁list_files(path=cmd/alex)
  → 10 files/directories
    [FILE] cli.go (4540 bytes)
    [FILE] config.go (3111 bytes)
    [FILE] container.go (2976 bytes)
    [FILE] cost.go (7373 bytes)
    [FILE] main.go (1702 bytes)
    ... and 5 more
```

**展示规则：**
- ≤ 10 files: 全部显示
- > 10 files: 显示前5个 + 统计

### 3. Smart Bash Output (智能命令输出)

**适用场景：** 命令输出可能很长，需要智能判断

```
⏺ 💻bash(command=ls -la)
    total 48
    drwxr-xr-x  10 user  staff   320 Jan 15 10:30 .
    drwxr-xr-x   5 user  staff   160 Jan 14 09:15 ..
    -rw-r--r--   1 user  staff  1234 Jan 15 10:30 file1.txt
```

**展示规则：**
- ≤ 300 chars: 完整显示
- > 300 chars:
  - 默认模式：显示行数统计 + 提示VERBOSE
  - VERBOSE模式：完整显示

### 4. Status Only (仅状态)

**适用场景：** 用户只需要知道操作成功

#### file_write
```
⏺ ✍️file_write(path=config.yaml, content=...)
  → ✓ file written
```

#### file_edit
```
⏺ ✏️file_edit(path=main.go, old=..., new=...)
  → ✓ file edited
```

#### web_search
```
⏺ 🌐web_search(query=golang best practices)
  → ✓ search completed
```

#### web_fetch
```
⏺ 📡web_fetch(url=https://example.com)
  → ✓ content fetched
```

**理由：** 这些操作的详细内容主要供LLM使用，用户只需知道操作成功

### 5. Statistical Summary (统计摘要)

**适用场景：** 内容供LLM分析，用户只需统计信息

#### file_read
```
⏺ 📄file_read(path=main.go)
  → 120 lines read
```

**理由：** 文件内容主要供LLM分析，用户只需知道读取了多少行

### 6. Thinking Output (思考输出)

```
⏺ 💭think()
  → Analyzing the problem: need to refactor the authentication module to support OAuth2...
```

**展示规则：**
- ≤ 100 chars: 完整显示
- > 100 chars: 截断为97 chars + "..."

## Verbose Mode

设置 `ALEX_VERBOSE=1` 启用详细模式：

```bash
export ALEX_VERBOSE=1
./alex "your task"
```

**效果：**
- 搜索工具显示所有匹配项
- Bash长输出完整展示
- 默认工具显示完整结果

## Implementation

### Code Location

`cmd/alex/stream_output.go:180-309` - `printSmartToolOutput()`

### Key Functions

```go
func (h *StreamingOutputHandler) printSmartToolOutput(toolName, result string)
```

根据工具类型智能选择展示策略

```go
func (h *StreamingOutputHandler) printFullOutput(label, content string, color lipgloss.Color)
```

格式化打印完整输出

## Benefits

1. **用户体验优化**
   - 关键信息不遗漏（code_execute, todo完整展示）
   - 减少信息噪音（file_read只显示行数）
   - 大数据智能摘要（search结果分页）

2. **可读性提升**
   - 输出层次清晰
   - 重要信息突出
   - 合理的信息密度

3. **灵活性**
   - VERBOSE模式提供完整输出选项
   - 不同工具不同策略
   - 易于扩展新工具类型

## Future Enhancements

- [ ] 支持工具级别的display hint (在tool definition中指定)
- [ ] 更智能的内容截断算法（保留重要行）
- [ ] 用户自定义展示配置
- [ ] 交互式查看完整输出（按键展开）
