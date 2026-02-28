# Lark Channel: 禁止 Markdown + 飞书友好格式

**Status**: Draft
**Created**: 2026-02-28

## 问题

当前 LLM 回复在 Lark 通道中以 `text` 消息类型发送，但 LLM 可能生成标准 Markdown 格式（`**bold**`、`## Header`、`- list`、`` ```code``` `` 等），而飞书的 `text` 消息不渲染 Markdown，导致用户看到原始的 Markdown 标记符号。

## 目标

1. **系统提示词层面**：让 LLM 在生成回复时就知道通道是 Lark，避免输出 Markdown
2. **投递层面**：在发送前将残留的 Markdown 转为飞书 `post` 富文本格式（兜底保障）

## 方案设计

### 第一层：系统提示词注入通道格式指令

**设计要点**：沿用现有模式——`preparation/service.go` 已有 `toolMode == "cli"` 时追加文件输出指令的先例（第 269-279 行）。用同样方式基于 channel 注入 Lark 格式指令。

**数据流**：
1. `ContextWindowConfig` 新增 `Channel string` 字段
2. `systemPromptInput` 新增 `Channel string` 字段
3. `composeSystemPrompt()` 中新增 `buildChannelFormattingSection(channel)` section
4. `BuildWindow()` 传递 channel
5. `preparation/service.go` 从 `ctx` 取 channel 传入 config

**Lark 格式指令内容**：
```
# Reply Formatting (Lark Channel)
当前回复通道为飞书 (Lark)，飞书 text 消息不支持 Markdown 渲染。
必须遵守以下格式规则：
- 禁止使用 Markdown 语法：不要使用 **bold**、*italic*、## heading、- list、> quote、[link](url)、```code```
- 使用纯文本格式：用换行分段，用「」或『』做强调，用数字编号代替无序列表
- 代码内容：保持原样但不要用 ``` 围栏，如需展示代码用缩进或直接内联
- 链接：直接贴 URL，不要用 [text](url) 格式
- 结构层次：用中文序号（一、二、三）或数字编号（1. 2. 3.）代替 heading
- 强调内容：用「关键词」或在前面加 → 箭头标注
```

### 第二层：投递层 Markdown → PostContent 转换（兜底）

LLM 不一定 100% 遵守指令，需要在发送前做兜底转换。

**设计**：新增 `markdown_to_post.go`，在现有的 `textContent()` 发送路径前，检测文本是否含 Markdown 标记。如果含有，转换为飞书 `post` 格式发送；否则保持 `text` 格式不变。

**转换规则**：

| Markdown 语法 | 飞书 Post 格式 |
|---|---|
| `**bold**` / `__bold__` | `{"tag":"text","text":"bold","style":["bold"]}` |
| `*italic*` / `_italic_` | `{"tag":"text","text":"italic","style":["italic"]}` |
| `[text](url)` | `{"tag":"a","text":"text","href":"url"}` |
| `` `code` `` | `{"tag":"text","text":"code","style":["bold"]}` (飞书无 inline code，用粗体近似) |
| `## Heading` | 新行 + `{"tag":"text","text":"Heading","style":["bold"]}` |
| `- item` / `* item` | `{"tag":"text","text":"  \u2022 item"}` |
| `1. item` | 保持原样 |
| `> quote` | `{"tag":"text","text":"| quote"}` |
| ` ```code block``` ` | 每行作为独立 text element（保持等宽感） |

**判断是否含 Markdown 的启发规则**：
- 正则检测 `**...**`、`## `、`- `（行首）、`[..](..)`、` ``` ` 等模式
- 只有匹配到 2+ 个模式才判定为含 Markdown（避免误判）
- 门槛低则转 post，对纯文本无副作用

**消息发送路径变更**：

```
当前：reply → ShapeReply7C() → textContent() → dispatch("text", ...)

改为：reply → ShapeReply7C() → smartContent(reply) → dispatch(msgType, content)

smartContent(text):
  if hasMarkdownPatterns(text):
    return "post", buildPostContent(text)
  else:
    return "text", textContent(text)
```

## 涉及文件

### 修改文件

| 文件 | 变更 |
|---|---|
| `internal/domain/agent/ports/agent/context.go` | `ContextWindowConfig` 加 `Channel string` |
| `internal/app/context/manager_prompt.go` | `systemPromptInput` 加 `Channel`；`composeSystemPrompt()` 加入 `buildChannelFormattingSection()` |
| `internal/app/context/manager_prompt_context.go` | 新增 `buildChannelFormattingSection(channel string) string` |
| `internal/app/context/manager_window.go` | `BuildWindow()` 从 ctx 取 channel 传入 input |
| `internal/app/agent/preparation/service.go` | 从 ctx 取 channel 传入 `ContextWindowConfig` |
| `internal/delivery/channels/lark/task_manager.go` | `dispatchResult()` 中使用 `smartContent()` 代替 `textContent()` |
| `internal/delivery/channels/lark/rephrase.go` | 前台 rephrase 提示词加 "不要使用 markdown 格式" |

### 新增文件

| 文件 | 内容 |
|---|---|
| `internal/delivery/channels/lark/markdown_to_post.go` | `hasMarkdownPatterns()` + `buildPostContent()` + `smartContent()` |
| `internal/delivery/channels/lark/markdown_to_post_test.go` | 转换逻辑的单元测试 |

## 实施步骤

1. **系统提示词层**：
   - `ContextWindowConfig` / `systemPromptInput` 加 `Channel` 字段
   - `buildChannelFormattingSection()` 实现
   - `BuildWindow()` 和 `preparation/service.go` 传递 channel
   - 更新 `rephraseForegroundSystemPrompt` 加 "不使用 markdown"

2. **投递层兜底**：
   - 实现 `markdown_to_post.go`
   - 修改 `task_manager.go` 的发送路径
   - 修改 progress listener 和 background listener 的发送路径

3. **测试**：
   - markdown_to_post 单元测试
   - 系统提示词中包含 channel section 的集成测试
   - 手动 Lark 验证

## 不做的事情

- 不改 CLI 和 Web 通道的格式逻辑（它们支持 Markdown）
- 不做完整的 Markdown AST parser（用正则即够，保持简洁）
- 不引入消息卡片 (interactive card)——post 已足够，card 需要更复杂的 schema
- 不改动 `ShapeReply7C()`——它是通用的结构清理，不涉及 Markdown 转换

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| LLM 忽略格式指令继续输出 Markdown | 第二层兜底转换保障 |
| Post 格式转换误判纯文本 | 双模式匹配门槛（2+ markdown pattern） |
| Post 消息对飞书机器人有字数限制 | 与 text 相同（约 4000 字符），超长自动截断已存在 |
| updateMessage 不支持 post 格式 | 飞书 SDK `UpdateMessage` 支持 post msgType，需验证 |
