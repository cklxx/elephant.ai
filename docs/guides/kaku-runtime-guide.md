# Kaku Runtime 操作手册

Updated: 2026-03-10

Kaku 是基于 WezTerm 的 GUI terminal，通过 `kaku cli` 控制 pane，作为多 agent session 的执行容器。

## 核心概念

```
Kaku GUI → Window → Tab（工作上下文）→ Pane（执行单元）→ Member CLI
```

| Kaku 操作 | Runtime 含义 |
|---|---|
| `split-pane` | 创建 session |
| `send-text` | 注入文本 |
| `send-text --no-paste $'\r'` | 提交（Enter） |
| `get-text` | 捕获屏幕输出 |
| `kill-pane` | 终止 session |

## kaku cli 命令参考

### 查看状态

```bash
kaku cli list   # 列出所有 window/tab/pane
```

### 创建 Pane

```bash
# 从已有 pane 拆分（推荐）
kaku cli split-pane --pane-id <ID> --right  --percent 30 --cwd <DIR> -- bash -l
kaku cli split-pane --pane-id <ID> --bottom --percent 65 --cwd <DIR> -- bash -l

# 新 tab（注意：进程立即退出的 pane 会消失）
kaku cli spawn --new-window false --cwd <DIR> -- bash -l
```

### 注入与提交

```bash
kaku cli send-text --pane-id <ID> "命令文本"         # 注入文本
kaku cli send-text --no-paste --pane-id <ID> $'\r'   # 提交 Enter
```

> `\n` 不等于 Enter。Interactive UI（CC 等）必须用 `--no-paste $'\r'` 提交。

### 读取输出

```bash
kaku cli get-text --pane-id <ID>            # 全屏快照
kaku cli get-text --pane-id <ID> | tail -20 # 最后 N 行
```

### 管理

```bash
kaku cli activate-pane --pane-id <ID>       # 聚焦
kaku cli kill-pane --pane-id <ID>           # 关闭
kaku cli set-tab-title --tab-id <ID> "标题"  # 设置标题
kaku cli zoom-pane --pane-id <ID>           # 全屏切换
```

## 启动 Member CLI

### Claude Code (Interactive)

```bash
PANE=$(kaku cli split-pane --pane-id <PARENT> --bottom --percent 70 \
  --cwd /path/to/project -- bash -l)

kaku cli send-text --pane-id $PANE "unset CLAUDECODE && claude --dangerously-skip-permissions"
kaku cli send-text --no-paste --pane-id $PANE $'\r'
sleep 5  # 等待 ❯ 提示符

kaku cli send-text --pane-id $PANE "任务描述"
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

### Claude Code (单次 Prompt)

```bash
kaku cli send-text --pane-id $PANE \
  'unset CLAUDECODE && claude --dangerously-skip-permissions -p "任务"'
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

### Codex

```bash
kaku cli send-text --pane-id $PANE 'codex exec --full-auto -- "任务"'
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

## 布局预置

```bash
KAKU_PANE_ID=5 bash scripts/kaku/layout.sh 4grid --cwd /path/to/project
# → TOP_LEFT=5  TOP_RIGHT=12  BOT_LEFT=13  BOT_RIGHT=14
```

| 名称 | 布局 |
|---|---|
| `4grid` | 2x2 四方格 |
| `2h` | 上下两行 |
| `2v` | 左右两列 |
| `3col` | 三列 |
| `1+2` | 主窗口 + 底部双监控 |

## CC 状态检测

| 屏幕特征 | CC 状态 |
|---|---|
| `❯ ` 空提示符 | 等待输入 |
| `⏺` 行滚动 | 正在执行 |
| `❯ ` 重新出现 | 完成本轮 |
| bash `$` 出现 | CC 退出 |

## 关键约束

- **CLAUDECODE 嵌套**：CC 检测到 `CLAUDECODE` env 时拒绝启动。必须 `unset CLAUDECODE` 后再启动。
- **send-text 换行**：`send-text "text\n"` 插入换行但不提交；`--no-paste $'\r'` 才是真 Enter。
- **get-text 是快照**：只返回当前屏幕，不是完整历史。

## Hooks 事件流

CC settings.json 配置 hooks：

```json
{
  "hooks": {
    "PostToolUse": [{"hooks": [{"type": "command",
      "command": "/path/to/scripts/cc_hooks/notify_runtime.sh", "async": true}]}],
    "Stop": [{"hooks": [{"type": "command",
      "command": "/path/to/scripts/cc_hooks/notify_runtime.sh", "async": true}]}]
  }
}
```

| CC Hook | Runtime 事件 |
|---|---|
| PostToolUse | `EventHeartbeat` |
| Stop (end_turn) | `EventCompleted` |
| Stop (error) | `EventFailed` |

## alex runtime CLI

```bash
alex runtime session start --member claude_code --goal "任务" --work-dir . --parent-pane-id $KAKU_PANE_ID
alex runtime session list [--state running]
alex runtime session status <id>
alex runtime session inject --id <id> --message "继续"
alex runtime session stop --id <id>
```

| 变量 | 说明 |
|---|---|
| `KAKU_PANE_ID` | 自动传入 `--parent-pane-id` |
| `KAKU_STORE_DIR` | 持久化目录（默认 `~/.kaku/sessions`） |

## Stall 检测

StallDetector 每 60s 扫描，触发 `EventStalled` → LeaderAgent 决策（inject/fail/escalate）。

```bash
# 查看事件
cat ~/.kaku/sessions/<id>.events.jsonl | jq .
tail -f ~/.kaku/sessions/*.events.jsonl | jq -r '"\(.type) \(.session_id)"'
```

## 多 Session 编排

```go
engine := scheduler.NewEngine(rt, rt.Bus(), parentPaneID)
ids, _ := engine.Schedule(ctx, []scheduler.SessionSpec{
    {Member: session.MemberClaudeCode, Goal: "phase 1"},
})
engine.Schedule(ctx, []scheduler.SessionSpec{
    {Member: session.MemberCodex, Goal: "phase 2", DependsOn: ids},
})
engine.Run(ctx)
```
