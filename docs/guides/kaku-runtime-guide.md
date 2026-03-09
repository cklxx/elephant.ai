# Kaku Runtime 操作手册

Updated: 2026-03-09
Status: Verified（实测通过）

Kaku 是 elephant.ai 的 CLI runtime 基础设施。它是一个基于 WezTerm 的 GUI terminal emulator，通过 `kaku cli` 接口暴露完整的 pane 控制能力，作为多 agent session 的执行容器。

---

## 1. 核心概念

```
Kaku GUI（用户可见）
  └── Window
        └── Tab（可多个，每个代表一个工作上下文）
              └── Pane（可分割，每个运行一个 process）
                    └── Member CLI（claude / codex / kimi / bash）
```

| Kaku 概念 | Runtime 概念 | 说明 |
|---|---|---|
| Tab | 工作空间 / Project | 一个任务集合的可视容器 |
| Pane | Session | 一个 member 的执行单元 |
| `split-pane` | StartSession | 创建执行容器 |
| `send-text` | InjectInput（文本） | 向 pane 注入内容 |
| `send-text --no-paste $'\r'` | InjectInput（提交） | 触发 Enter 提交 |
| `get-text` | CaptureOutput | 读取 pane 当前屏幕内容 |
| `kill-pane` | StopSession | 终止 session |
| `activate-pane` | FocusSession | 聚焦到某个 session |

---

## 2. kaku cli 完整命令参考

### 查看状态

```bash
# 列出所有 window / tab / pane
kaku cli list

# 输出格式：
# WINID TABID PANEID WORKSPACE SIZE   TITLE  CWD
#     0     2      5 default   128x50 ~      file:///Users/.../elephant.ai
```

### 创建 Pane

```bash
# 从已有 pane 拆分（推荐方式）
kaku cli split-pane --pane-id <ID> --right  --percent 30 --cwd <DIR> -- bash -l
kaku cli split-pane --pane-id <ID> --bottom --percent 65 --cwd <DIR> -- bash -l

# 在新 tab 创建（注意：spawn 的 pane 如果 process 立即退出会消失）
kaku cli spawn --new-window false --cwd <DIR> -- bash -l
# 返回值：新建的 pane ID
```

### 注入文本 / 命令

```bash
# 注入普通文本（paste 模式，不触发 Enter）
kaku cli send-text --pane-id <ID> "some text"

# 触发 Enter 提交（--no-paste + $'\r'，必须两者都加）
kaku cli send-text --no-paste --pane-id <ID> $'\r'

# 注入一行命令并执行（文本 + 换行两步）
kaku cli send-text --pane-id <ID> "ls -la"
kaku cli send-text --no-paste --pane-id <ID> $'\r'
```

> **关键规则**：`\n` 不等于 Enter。CC 等 interactive UI 必须用 `--no-paste $'\r'` 才能提交。

### 读取输出

```bash
# 获取 pane 当前屏幕内容（全屏快照）
kaku cli get-text --pane-id <ID>

# 实用技巧：只看最后 N 行
kaku cli get-text --pane-id <ID> | tail -20
```

### 管理 Pane / Tab

```bash
# 激活（聚焦）pane
kaku cli activate-pane --pane-id <ID>

# 关闭 pane
kaku cli kill-pane --pane-id <ID>

# 设置 tab 标题
kaku cli set-tab-title --tab-id <ID> "标题"

# 缩放 pane（全屏/取消全屏）
kaku cli zoom-pane --pane-id <ID>

# 调整 pane 大小
kaku cli adjust-pane-size --pane-id <ID> --direction Right --amount 10
```

---

## 3. 启动 Member CLI

### Claude Code（CC）—— Interactive 对话模式

```bash
# Step 1: 创建 pane
PANE=$(kaku cli split-pane --pane-id <PARENT> --bottom --percent 70 \
  --cwd /path/to/project -- bash -l)

# Step 2: 启动 CC（必须 unset CLAUDECODE 防止嵌套报错）
kaku cli send-text --pane-id $PANE "unset CLAUDECODE && claude --dangerously-skip-permissions"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# Step 3: 等待 CC 启动（出现 ❯ 提示符）
sleep 5

# Step 4: 发送消息
kaku cli send-text --pane-id $PANE "你的任务描述"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# Step 5: 读取响应
sleep 15
kaku cli get-text --pane-id $PANE | tail -30
```

CC interactive 模式的提示符特征：`❯`（等待输入时）

### Claude Code —— 单次 Prompt 模式

```bash
PANE=$(kaku cli split-pane --pane-id <PARENT> --bottom --percent 70 \
  --cwd /path/to/project -- bash -l)

kaku cli send-text --pane-id $PANE \
  'unset CLAUDECODE && claude --dangerously-skip-permissions -p "你的任务"'
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

### Codex

```bash
PANE=$(kaku cli split-pane --pane-id <PARENT> --bottom --percent 70 \
  --cwd /path/to/project -- bash -l)

kaku cli send-text --pane-id $PANE \
  'codex exec --full-auto -- "你的任务"'
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

### 普通 Shell 命令

```bash
PANE=$(kaku cli split-pane --pane-id <PARENT> --right --percent 30 \
  --cwd /path/to/project -- bash -l)

kaku cli send-text --pane-id $PANE "git log --oneline -10"
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

---

## 4. 标准布局模式

### 双 pane（主 + 监控）

```
┌─────────────────────────┬──────────────┐
│                         │              │
│   Member（CC/Codex）     │   Monitor    │
│   pane: 执行任务          │   pane: 状态  │
│                         │              │
└─────────────────────────┴──────────────┘
```

```bash
PARENT=5   # 当前对话所在 pane

# 主执行 pane（左，70%）
EXEC=$(kaku cli split-pane --pane-id $PARENT --right --percent 30 \
  --cwd /path/to/project -- bash -l)

# 监控 pane（右，30%）
MON=$(kaku cli split-pane --pane-id $PARENT --right --percent 30 \
  --cwd /path/to/project -- bash -l)
```

### 三 pane（当前对话 + CC + 监控）

```
┌──────────────┬───────────────────┐
│              │                   │
│  Claude Code │   当前对话（CC）    │
│  pane        │   pane (pane 5)   │
│              │                   │
├──────────────┴───────────────────┤
│         Monitor pane             │
└──────────────────────────────────┘
```

```bash
# 从当前 pane 底部拆出大 pane 跑 CC
CC=$(kaku cli split-pane --pane-id 5 --bottom --percent 65 \
  --cwd /path/to/project -- bash -l)

# 从侧边拆出监控 pane
MON=$(kaku cli split-pane --pane-id 5 --right --percent 35 \
  --cwd /path/to/project -- bash -l)
```

---

## 5. 检测 CC 状态

CC interactive 模式的屏幕状态特征：

| 屏幕特征 | CC 状态 | Kaku runtime 事件 |
|---|---|---|
| `❯ ` 空提示符 | 等待输入 | `needs_input` |
| `⏺` 开头的行正在滚动 | 正在执行 | `heartbeat` |
| `❯ ` 重新出现（执行完毕后） | 完成本轮 | `completed`（当前 turn） |
| `esc to interrupt` 消失 | session 结束 | `completed` |
| bash 提示符（`$`）出现 | CC 进程退出 | `completed` / `failed` |

读取状态的轮询写法：

```bash
while true; do
  OUTPUT=$(kaku cli get-text --pane-id $CC_PANE | tail -5)
  if echo "$OUTPUT" | grep -q '^\$'; then
    echo "CC session ended"
    break
  fi
  if echo "$OUTPUT" | grep -q '^❯ $'; then
    echo "CC waiting for input"
  fi
  sleep 3
done
```

---

## 6. 关键约束与已知问题

### CLAUDECODE 嵌套保护

CC 检测到 `CLAUDECODE` 环境变量时拒绝启动：

```
Error: Claude Code cannot be launched inside another Claude Code session.
```

**必须在启动命令前 unset：**

```bash
unset CLAUDECODE && claude --dangerously-skip-permissions
```

### send-text 换行行为

| 用法 | 效果 |
|---|---|
| `send-text "text\n"` | 插入文字 + 换行字符，**不提交** |
| `send-text --no-paste $'\r'` | 触发真实 Enter 键，**提交** |
| `send-text --no-paste $'\n'` | 换行字符，在 CC UI 中**不提交** |

### spawn 的 pane 生命周期

`kaku cli spawn` 创建的 pane 如果命令立即退出（如 `bash -l` 没有 stdin 保持），pane 会关闭。建议始终用 `split-pane`，从已有的存活 pane 拆分。

### get-text 是屏幕快照

`get-text` 返回的是当前可见屏幕内容，不是完整历史。长输出会被截断（只保留屏幕可见区域）。监控时用 `tail -N` 读最后几行。

---

## 7. 与 Runtime 的集成

Kaku pane 控制直接映射到 MemberAdapter 接口：

```go
type KakuPanel struct {
    paneID int
    binary string // "/Applications/Kaku.app/Contents/MacOS/kaku"
}

func (p *KakuPanel) InjectText(text string) error {
    // kaku cli send-text --pane-id <ID> "<text>"
}

func (p *KakuPanel) Submit() error {
    // kaku cli send-text --no-paste --pane-id <ID> $'\r'
}

func (p *KakuPanel) CaptureOutput() (string, error) {
    // kaku cli get-text --pane-id <ID>
}

func (p *KakuPanel) Kill() error {
    // kaku cli kill-pane --pane-id <ID>
}
```

P0 Runtime Skeleton 无需实现 pty 管理，直接封装 `kaku cli` shell 调用即可。用户在 Kaku GUI 中实时可见所有执行过程。

---

## 8. Hooks 事件流（P1）

### notify_runtime.sh 配置

ClaudeCodeAdapter 会在启动 CC 前注入环境变量，并期望 CC 的 settings.json 配置了如下 hooks：

```json
{
  "hooks": {
    "PostToolUse": [{
      "hooks": [{ "type": "command",
        "command": "/path/to/scripts/cc_hooks/notify_runtime.sh",
        "async": true }]
    }],
    "Stop": [{
      "hooks": [{ "type": "command",
        "command": "/path/to/scripts/cc_hooks/notify_runtime.sh",
        "async": true }]
    }]
  }
}
```

### 事件映射表

| CC Hook | RuntimeHooksHandler 转化 | 含义 |
|---|---|---|
| PostToolUse | `EventHeartbeat` | CC 正在工作 |
| PreToolUse | `EventHeartbeat` | CC 准备用工具 |
| Stop (end_turn) | `EventCompleted` | 成功完成 |
| Stop (error) | `EventFailed` | 错误退出 |

### 手动测试 hooks endpoint

```bash
# 测试 heartbeat
curl -X POST "http://localhost:8080/api/hooks/runtime?session_id=rs-test" \
  -H "Content-Type: application/json" \
  -d '{"hook_event_name":"PostToolUse","tool_name":"Bash"}'

# 测试 completed
curl -X POST "http://localhost:8080/api/hooks/runtime?session_id=rs-test" \
  -H "Content-Type: application/json" \
  -d '{"hook_event_name":"Stop","stop_reason":"end_turn","answer":"done"}'
```

---

## 9. alex runtime CLI（P3）

### Session 生命周期命令

```bash
# 启动（使用当前 pane 作为父 pane，CC 在新 pane 中运行）
alex runtime session start \
  --member claude_code \
  --goal "统计 Go 文件数量" \
  --work-dir . \
  --parent-pane-id $KAKU_PANE_ID

# 列出所有 session
alex runtime session list

# 只看运行中
alex runtime session list --state running

# 查看详情（JSON）
alex runtime session status rs-abc12345

# 注入文本（解封 stalled session）
alex runtime session inject --id rs-abc12345 --message "请继续"

# 停止
alex runtime session stop --id rs-abc12345
```

### 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `KAKU_PANE_ID` | — | 自动传入 `--parent-pane-id` |
| `KAKU_STORE_DIR` | `~/.kaku/sessions` | 持久化目录 |

---

## 10. Stall 检测与 Leader Agent（P2）

### Stall 检测流程

```
StallDetector（每 60s 扫描）
  → ScanStalled(threshold=60s)
  → bus.Publish(EventStalled)
  → LeaderAgent.handleStall()
     → LLM prompt: "session stalled for X — inject/fail/escalate?"
     → INJECT <message>  → rt.InjectText()
     → FAIL <reason>     → rt.MarkFailed()
     → ESCALATE          → bus.Publish(EventHandoffRequired)
```

### 查看 session 事件历史

```bash
# 所有事件
cat ~/.kaku/sessions/rs-abc12345.events.jsonl | jq .

# 只看 stall 事件
cat ~/.kaku/sessions/rs-abc12345.events.jsonl | jq 'select(.type == "stalled")'

# 监控实时事件流
tail -f ~/.kaku/sessions/*.events.jsonl | jq -r '"\(.type) \(.session_id)"'
```

---

## 11. 多 Session 编排（DependencyScheduler）

```go
// 创建 engine
engine := scheduler.NewEngine(rt, rt.Bus(), parentPaneID)

// 注册 sessions（串行）
ids, _ := engine.Schedule(ctx, []scheduler.SessionSpec{
    {Member: session.MemberClaudeCode, Goal: "phase 1"},
})
engine.Schedule(ctx, []scheduler.SessionSpec{
    {Member: session.MemberCodex, Goal: "phase 2", DependsOn: ids},
})

// 开始调度（阻塞直到所有完成）
engine.Run(ctx)
```
