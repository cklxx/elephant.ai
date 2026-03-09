# Kaku Runtime 集成测试通用方案

## 核心原则

**起点永远是对话**。所有测试从 `POST /api/dev/inject` 注入一条飞书消息开始，
不直接调用内部 API。通过三层验证确认系统行为符合预期：

```
[1] 注入对话消息
       ↓
[2] 观察 Agent 执行轨迹（日志）
       ↓
[3] 核验 Kaku GUI 界面结果
```

---

## 基础设施

### 端口

| 服务 | 地址 | 用途 |
|---|---|---|
| HTTP API | `http://localhost:8080` | 对外 API，包含 `/api/hooks/runtime` |
| Debug 服务 | `http://localhost:9090` | 包含 `/api/dev/inject`（本地测试专用） |

### 注入接口（inject endpoint）

```bash
# 通用注入函数
inject() {
  local TEXT="$1"
  local TIMEOUT="${2:-120}"
  curl -s -X POST "http://localhost:9090/api/dev/inject" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\": \"$TEXT\",
      \"chat_type\": \"p2p\",
      \"timeout_seconds\": $TIMEOUT
    }" | jq .
}

# 示例
inject "帮我数一下项目里有多少个 .go 文件"
```

inject 响应格式：
```json
{
  "replies": [
    {"method": "AddReaction",  "emoji": "OnIt"},
    {"method": "ReplyMessage", "content": "项目共有 32315 个 .go 文件"}
  ],
  "duration_ms": 8200,
  "error": ""
}
```

### 日志监控（三条命令，开三个 pane）

```bash
# Pane A：Agent 执行轨迹（所有 INFO 日志）
tail -f ~/alex-service.log | grep -v "EventBroadcaster"

# Pane B：Runtime bus 事件（只看 kaku runtime 事件）
tail -f ~/alex-service.log | grep "runtime_bus_event"

# Pane C：HTTP 请求日志（看 inject 调用和 hooks 回调）
tail -f /Users/bytedance/code/elephant.ai/logs/requests/*.log 2>/dev/null \
  || tail -f ~/alex-service.log | grep -E "POST|GET|http"
```

---

## Pane 生命周期管理

### 规则
- **测试前**：检查并关闭上次遗留的测试 pane（避免窗口污染）
- **测试中**：每个测试用一个独立 tab（`kaku cli spawn`）
- **测试后**：用例完成后立即 kill 测试 pane

### 标准初始化与清理

```bash
# ===== 测试前：清理旧测试 pane =====
# 列出所有 pane，找到标题包含 "Test" 或 "kaku" 的（上次遗留）
kaku cli list

# 批量 kill 测试遗留 pane（替换 pane ID）
for PANE in 13 14 15 16 17 18 19 20; do
  kaku cli kill-pane --pane-id $PANE 2>/dev/null && echo "killed $PANE" || true
done

# ===== 测试中：创建独立 tab =====
TEST_PANE=$(kaku cli spawn --window-id 0 \
  --cwd /Users/bytedance/code/elephant.ai -- bash -l)
kaku cli set-tab-title --pane-id $TEST_PANE "TC-1 基础任务"
echo "Test pane: $TEST_PANE"

# ===== 测试后：清理 =====
kaku cli kill-pane --pane-id $TEST_PANE
```

### Pane 分配推荐（4grid 布局）

```
┌──────────────────┬──────────────────┐
│   TL (当前 CC)   │   TR (测试 pane) │
│  - 本次对话       │  - CC session 在此运行
├──────────────────┼──────────────────┤
│  BL (日志监控)   │  BR (inject 脚本)│
│  tail -f logs    │  curl 注入命令   │
└──────────────────┴──────────────────┘
```

```bash
# 一键创建 4grid 测试布局
KAKU_PANE_ID=$KAKU_PANE_ID bash scripts/kaku/layout.sh 4grid \
  --cwd /Users/bytedance/code/elephant.ai
# 输出：TOP_LEFT=X TOP_RIGHT=Y BOT_LEFT=Z BOT_RIGHT=W

# 左下角 pane 开日志
kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/alex-service.log | grep -E 'runtime_bus_event|TaskExecution'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'
```

---

## 测试用例模板

每个测试用例遵循统一结构：**注入 → 等待 → 三层验证 → 清理**。

```
TC-N：<测试名称>
├── 前提条件
├── Step 1: 注入对话消息（inject）
├── Step 2: 等待执行
├── Step 3a: 验证 inject 响应（同步 reply）
├── Step 3b: 验证日志轨迹（log trace）
├── Step 3c: 验证 Kaku GUI（界面结果）
└── Step 4: 清理 pane
```

---

## TC-1：基础任务 — CC 启动并完成

**场景**：注入一个简单任务，CC 在 Kaku pane 执行完后状态变 `completed`，hooks 事件可见。

### Step 1：测试前清理

```bash
# 清理上次遗留 pane（kaku cli list 查出 ID 再 kill）
kaku cli list
kaku cli kill-pane --pane-id <旧 pane id> 2>/dev/null || true

# 创建测试 tab
TEST_PANE=$(kaku cli spawn --window-id 0 \
  --cwd /Users/bytedance/code/elephant.ai -- bash -l)
kaku cli set-tab-title --pane-id $TEST_PANE "TC-1 基础任务"
echo "TEST_PANE=$TEST_PANE"
```

### Step 2：注入对话

```bash
inject "帮我用 find 数一下 /Users/bytedance/code/elephant.ai 里有多少个 .go 文件，数完就停"
```

**内部触发链**：
```
LarkGateway.InjectMessageSync
  → AgentCoordinator → 识别为 runtime session 任务
  → alex runtime session start --member claude_code --parent-pane-id $TEST_PANE
  → ClaudeCodeAdapter.Start()
  → kaku cli split-pane → 新 pane (CC session pane)
  → unset CLAUDECODE && claude --dangerously-skip-permissions
  → inject goal → CC 执行 → notify_runtime.sh → POST /api/hooks/runtime
```

### Step 3：等待

```bash
sleep 30   # CC 执行约 15-30s
```

### Step 4a：验证 inject 响应

预期 `replies` 包含任务确认或结果：
```json
{
  "replies": [
    {"method": "AddReaction", "emoji": "OnIt"},
    {"method": "ReplyMessage", "content": "找到 32,315 个 .go 文件"}
  ]
}
```

### Step 4b：验证日志轨迹

```bash
grep "runtime_bus_event" ~/alex-service.log | tail -10
```

**预期日志序列**：
```
runtime_bus_event type=heartbeat  session_id=rs-xxxx at=HH:MM:SS
runtime_bus_event type=heartbeat  session_id=rs-xxxx at=HH:MM:SS  ← 每次 tool use
runtime_bus_event type=completed  session_id=rs-xxxx at=HH:MM:SS
```

**如果缺少 heartbeat**：检查 CC 是否配置了 `notify_runtime.sh` hook。
**如果缺少 completed**：检查 CC 是否配置了 Stop hook 或 pane 轮询是否检测到 bash 提示符。

### Step 4c：验证 Kaku GUI

```bash
# 找到 CC session pane（比 TEST_PANE 大 1 的 pane ID）
kaku cli list

# 读取 CC pane 最终输出
kaku cli get-text --pane-id <CC_PANE> | tail -15
```

**预期 Kaku 界面**：
```
L4L2Y39H4F:elephant.ai bytedance$ export RUNTIME_SESSION_ID='rs-xxxx' RUNTIME_HOOKS_URL='http://localhost:8080'
L4L2Y39H4F:elephant.ai bytedance$ unset CLAUDECODE && claude --dangerously-skip-permissions

 ▐▛███▜▌   Claude Code v2.1.71
 ...
❯ 帮我用 find 数一下...

⏺ Searched for 1 pattern
⏺ There are 32,315 .go files in the repository.

❯                     ← CC 等待下一条指令（任务完成）
```

### Step 5：清理

```bash
kaku cli kill-pane --pane-id $TEST_PANE
kaku cli kill-pane --pane-id <CC_PANE> 2>/dev/null || true
```

---

## TC-2：Stall 检测 + LeaderAgent 自动恢复

**场景**：CC 被注入一个"无限等待"任务，超过 stall 阈值后 LeaderAgent 自动恢复。

### Step 1：注入"会卡住"的任务

```bash
inject "帮我启动一个 runtime session，任务是：什么也不做，等我告诉你再说"
```

### Step 2：等待 Stall 触发（默认 60s）

```bash
sleep 75
```

### Step 3b：验证 stall 日志

```bash
grep "runtime_bus_event" ~/alex-service.log | grep "stalled" | tail -5
```

**预期**：
```
runtime_bus_event type=stalled session_id=rs-xxxx at=HH:MM:SS
```

**Leader 决策日志**（需要 LeaderAgent 已接入）：
```
runtime leader: session rs-xxxx decision=INJECT reason="session stalled for 60s"
```

### Step 3c：验证 Kaku GUI

```bash
kaku cli get-text --pane-id <CC_PANE> | tail -5
```

LeaderAgent 执行 INJECT 后，CC pane 应收到注入的提示文本（如"请继续执行任务，完成后停止"）。

### 手动备用（LeaderAgent 未接入时）

```bash
alex runtime session inject --id rs-xxxx --message "请继续执行任务，完成后停止。"
```

---

## TC-3：notify_runtime.sh 健壮性

**目标**：验证 hook 脚本在各种故障情况下不阻塞 CC（fire-and-forget）。

### 正常路径

```bash
RUNTIME_SESSION_ID="robust-test" \
RUNTIME_HOOKS_URL="http://localhost:8080" \
bash scripts/cc_hooks/notify_runtime.sh << 'EOF'
{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":"ok"}
EOF

# 验证：日志出现 heartbeat
grep "runtime_bus_event.*robust-test" ~/alex-service.log | tail -3
```

**预期**：`type=heartbeat session_id=robust-test`

### 服务不可用（端口不存在）

```bash
time RUNTIME_SESSION_ID="fail-test" \
RUNTIME_HOOKS_URL="http://localhost:9999" \
bash scripts/cc_hooks/notify_runtime.sh <<< '{}'
```

**预期**：`real` 时间 ≤ 5s（不阻塞 CC）

### 无 SESSION_ID（静默退出）

```bash
RUNTIME_SESSION_ID="" \
RUNTIME_HOOKS_URL="http://localhost:8080" \
bash scripts/cc_hooks/notify_runtime.sh <<< '{}'
echo "exit code: $?"
```

**预期**：`exit code: 0`，无任何 curl 调用

### 畸形 JSON payload

```bash
RUNTIME_SESSION_ID="json-test" \
RUNTIME_HOOKS_URL="http://localhost:8080" \
bash scripts/cc_hooks/notify_runtime.sh <<< 'not valid json at all'
grep "runtime_bus_event.*json-test" ~/alex-service.log | tail -3
```

**预期**：jq 失败 → fallback minimal payload → HTTP 200 → `type=heartbeat`

---

## 验收标准汇总

| # | 标准 | 验证层 | 命令 |
|---|---|---|---|
| 1 | inject 正常返回，有 reply | inject 响应 | `jq .replies` |
| 2 | CC pane 出现并可见执行过程 | Kaku GUI | `kaku cli get-text` |
| 3 | PostToolUse → heartbeat 事件到达 bus | 日志 | `grep runtime_bus_event` |
| 4 | Stop(end_turn) → completed 事件 | 日志 | `grep runtime_bus_event` |
| 5 | session 状态变为 `completed` | CLI | `alex runtime session status` |
| 6 | stall 超时 → EventStalled 出现 | 日志 | `grep stalled` |
| 7 | notify_runtime.sh 故障不阻塞 CC | 时间 | `time` ≤ 5s |
| 8 | SESSION_ID 为空时静默 exit 0 | 退出码 | `echo $?` |
| 9 | 测试完成后 pane 被清理 | Kaku 列表 | `kaku cli list` |

---

## 快速冒烟脚本

```bash
#!/bin/bash
# scripts/test/kaku-runtime-smoke.sh
# 用法：KAKU_PANE_ID=<id> bash scripts/test/kaku-runtime-smoke.sh

set -euo pipefail
HOOKS_URL="http://localhost:8080"
DEBUG_URL="http://localhost:9090"

echo "=== [1] Health Check ==="
curl -sf "$HOOKS_URL/health" | jq -r '"Server: \(.status)"'

echo "=== [2] /api/hooks/runtime endpoint ==="
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  "$HOOKS_URL/api/hooks/runtime?session_id=smoke-$$" \
  -H "Content-Type: application/json" \
  -d '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{},"tool_response":"ok"}')
echo "POST /api/hooks/runtime → HTTP $HTTP"
[[ "$HTTP" == "200" ]] || { echo "FAIL"; exit 1; }

echo "=== [3] Bus event logged ==="
sleep 1
EVENTS=$(grep "runtime_bus_event.*smoke-$$" ~/alex-service.log 2>/dev/null | wc -l)
echo "Events in log: $EVENTS"
[[ "$EVENTS" -ge 1 ]] || { echo "FAIL: no events"; exit 1; }

echo "=== [4] notify_runtime.sh robustness ==="
# 正常路径
RUNTIME_SESSION_ID="smoke-notify-$$" RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash scripts/cc_hooks/notify_runtime.sh <<< '{"hook_event_name":"Stop","stop_reason":"end_turn"}'
# 静默退出
RUNTIME_SESSION_ID="" RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash scripts/cc_hooks/notify_runtime.sh <<< '{}' && echo "Silent exit: OK"
# 超时不阻塞
ELAPSED=$(RUNTIME_SESSION_ID="x" RUNTIME_HOOKS_URL="http://localhost:9999" \
  bash -c 'time bash scripts/cc_hooks/notify_runtime.sh <<< "{}"' 2>&1 | grep real | awk '{print $2}')
echo "Timeout elapsed: $ELAPSED (should be ≤5s)"

echo ""
echo "✓ Smoke tests passed"
echo "→ 手动验证：inject 对话，观察 Kaku GUI + 日志"
```

---

## 当前未接入（待实现，不影响 TC-1~4）

| 功能 | 状态 | 实现路径 |
|---|---|---|
| LeaderAgent 服务器启动 | ⏳ | `bootstrap/server.go` 启动 `leader.Agent.Run(ctx)` |
| CC hooks 自动注册 | ⏳ | `ClaudeCodeAdapter.Start()` 写入临时 `settings.json` |
| Session 完成回飞书 | ⏳ | bus 订阅者调用 `LarkGateway.ReplyMessage` |
| `--depends-on` CLI flag | ⏳ | `alex runtime session start` 加 `--depends-on <id>` |
