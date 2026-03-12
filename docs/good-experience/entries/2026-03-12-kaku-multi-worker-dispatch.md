# 2026-03-12 - Kaku CLI 多 Worker 并发调度

## Context
- 目标：利用 kaku-cli 在 tmux 中实现 1-leader + N-worker 的多 pane 并发调度，提升批量任务吞吐。
- 场景：leader pane 负责任务派发与状态巡查，worker pane 各自执行独立任务（代码修改、文档撰写等）。
- 约束：worker 之间无共享状态，leader 通过 tmux pane 文本交互完成协调。

## What Worked

### 调度架构：Leader + Worker 分离
- Leader pane 持有任务队列，负责循环巡查各 worker pane 状态并派发下一个任务。
- Worker pane 仅执行单个任务，完成后回到空闲提示符等待下一次派发。
- 分离职责使 leader 逻辑简单：轮询 → 检测空闲 → 派发 → 轮询。

### Pane 管理：get-text / send-text
- 使用 `kaku cli get-text --pane <id>` 获取 worker pane 当前屏幕文本，判断执行状态。
- 使用 `kaku cli send-text --pane <id> "<prompt>"` 向 worker pane 发送任务指令。
- 巡查频率按需调整，避免过于频繁造成干扰。

### 任务池设计
- 将待执行任务组织为有序列表（本次 5 个任务），leader 按顺序循环派发。
- 每次巡查发现空闲 worker 时从池中取出下一个任务派发，直到任务池耗尽。
- 任务粒度保持独立，避免跨 worker 依赖导致阻塞。

### Stall 检测：识别空闲提示符
- Worker 完成任务后会回到 shell 提示符或 claude-code 的 idle 提示。
- Leader 通过 `get-text` 返回的文本匹配提示符模式（如 `$`、`❯`、`>`）判断 worker 是否空闲。
- 对长时间无输出变化的 pane 也视为 stall，可触发重派或人工介入。

## Lessons Learned

### send-text 需要 no-paste 模式发送回车
- tmux `send-keys` 默认使用 bracket paste 模式，多行文本会被粘贴而非逐行执行。
- 解决：使用 `kaku cli send-text` 的 no-paste 选项，确保末尾回车被正确发送为 Enter 键事件。
- 踩坑表现：worker 收到文本但不执行，看起来像"粘贴了但没按回车"。

### Context 不足时任务堆积
- 当 worker pane 的 claude-code 会话 context 接近上限时，新任务派发后可能无法正常启动或产出质量下降。
- 表现：worker 反复要求确认、输出截断、或静默失败。
- 缓解：leader 在检测到异常输出模式时标记该 worker 为 degraded，跳过派发或提示人工重启会话。

## Reusable Rule
- 多 worker 并发调度模式：
  1. Leader 与 worker 分离，leader 仅做巡查+派发，不执行业务。
  2. 通过 pane 文本内容做状态检测，避免引入额外 IPC。
  3. send-text 必须使用 no-paste 模式确保回车生效。
  4. 监控 worker context 容量，及时标记 degraded worker 避免任务浪费。
  5. 任务粒度保持独立，不跨 worker 建立依赖。

## Metadata
- id: good-2026-03-12-kaku-multi-worker-dispatch
- tags: [kaku, cli, multi-worker, dispatch, tmux, concurrency]
- links: []
