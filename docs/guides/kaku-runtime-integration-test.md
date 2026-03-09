# Kaku Runtime 集成测试通用方案

## 核心原则

**起点永远是对话**。所有测试从 `POST /api/dev/inject` 注入一条飞书消息开始——
这是用户真实使用路径的起点，不直接调用内部 API 或 CLI。

通过三层验证确认系统行为：

```
[1] inject → 模拟用户发飞书消息
       ↓
[2] 观察 Agent 执行轨迹（日志）
       ↓
[3] 核验 Kaku GUI 界面结果（kaku cli get-text）
```

> **禁止**：在 E2E 测试步骤中直接调用 `alex runtime session start`、
> `POST /api/runtime/sessions` 等内部接口——那是 CLI/API 功能本身的单元测试，
> 由 CLI 开发者通过 Go test 和单元测试保障，不是 E2E 测试的范畴。

---

## 基础设施

### 端口

| 服务 | 地址 | 用途 |
|---|---|---|
| Debug 服务 | `http://localhost:9090` | inject 端点、runtime hooks、runtime session API |
| 主服务 | `http://localhost:8080` | 对外 API（可选） |

### inject 接口

```bash
# 通用 inject 函数
inject() {
  local TEXT="$1" TIMEOUT="${2:-60}"
  curl -s -X POST "http://localhost:9090/api/dev/inject" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\": $(printf '%s' "$TEXT" | jq -Rs .),
      \"chat_type\": \"p2p\",
      \"timeout_seconds\": $TIMEOUT
    }" | jq .
}

inject "帮我数一下项目里有多少个 .go 文件"
```

响应格式：

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

---

## 四宫格测试布局（标准基础设施）

所有集成测试使用统一的四宫格布局。布局由 `scripts/kaku/layout.sh` 创建，
必须在测试前通过 `/kaku-runtime-test` skill 建立。

```
┌──────────────────────┬──────────────────────┐
│   TL: 当前 Claude    │   TR: CC session     │
│  (不改动)            │  runtime 在此 split  │
├──────────────────────┼──────────────────────┤
│   BL: 日志监控        │   BR: inject 命令    │
│  tail -f + grep      │  curl / 测试脚本     │
└──────────────────────┴──────────────────────┘
```

### 布局创建

```bash
# 从当前 Claude pane 分裂三个辅助 pane
eval $(bash scripts/kaku/layout.sh 4grid \
  --pane-id "$KAKU_PANE_ID" \
  --cwd /Users/bytedance/code/elephant.ai \
  | grep -E "TOP_LEFT|TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | sed 's/  /\n/g' | awk '{print "export "$0}')

# BL：启动日志监控（测试期间保持运行）
kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/code/elephant.ai/logs/alex-service.log | grep -E 'runtime_bus_event|TaskExecution'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'

# 告知 runtime：CC session 应 split 自 TR pane
export KAKU_PARENT_PANE=$TOP_RIGHT
```

### 布局清理（测试完成后）

```bash
kaku cli kill-pane --pane-id $TOP_RIGHT 2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_LEFT  2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_RIGHT 2>/dev/null || true
kaku cli list   # 确认恢复
```

### Pane 复用原则

- **同一功能的 pane 复用**：每次测试前检查 pane 数量，不要无限 spawn
- **标题标记**：测试 pane 用 `kaku cli set-tab-title` 标记，便于识别和清理
- **TC 执行顺序复用 TR pane**：多个 TC 复用同一 TR 作为 CC session 父 pane，测试间不重建

---

## 测试用例结构

每个测试用例遵循固定模板：

```
TC-N <名称>
├── [前提] 检查服务健康
├── Step 1: inject 消息（起点）
├── Step 2: 等待异步执行
├── Step 3a: Layer 1 — inject 响应验证（同步 reply）
├── Step 3b: Layer 2 — 日志验证（BL pane，async）
├── Step 3c: Layer 3 — Kaku GUI 验证（TR pane get-text）
└── Step 4: [仅出错时] 清理本 TC 创建的 pane
```

---

## TC-1 基础对话

**目标**：验证 inject → agent 执行 → 回复全链路可用。

```bash
# Step 1: inject
RESP=$(inject "你好，请只回复两个字：OK好的" 30)

# Step 3a: Layer 1
echo "$RESP" | jq '.replies[].content'
# 期望: "OK好的" 或含"OK"/"好"

# Step 3b: Layer 2（BL pane 已在监控）
tail -30 ~/code/elephant.ai/logs/alex-service.log | grep "TaskExecution"
# 期望: agent 有处理记录
```

---

## TC-2 任务执行

**目标**：验证 agent 理解任务意图、执行工具、返回结果。

```bash
# Step 1: inject
RESP=$(inject "请告诉我项目里大约有多少个 .go 文件（估算即可）" 60)

# Step 3a: Layer 1 — 回复含数字
echo "$RESP" | jq '.replies[].content'

# Step 3b: Layer 2 — agent 使用了工具
tail -50 ~/code/elephant.ai/logs/alex-service.log | grep -E "tool_use|Bash|Glob"
```

---

## TC-3 Runtime Session（CC 在 Kaku 出现）

**目标**：inject 一个编程任务 → agent 触发 runtime → CC 在 TR pane 运行 → hooks 回调 → 完成通知。

```bash
# Step 1: inject
RESP=$(inject \
  "请启动一个编程 session，任务是：echo kaku-runtime-tc3-ok，完成后告诉我结果" \
  120)

# Step 2: 等待
sleep 20

# Step 3a: Layer 1 — agent 回复确认
echo "$RESP" | jq '.replies[].content'

# Step 3b: Layer 2 — runtime bus 事件链
tail -100 ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event"
# 期望序列：
#   runtime_bus_event type=started  session_id=rs-xxxx
#   runtime_bus_event type=heartbeat session_id=rs-xxxx  ← CC 每次 tool use
#   runtime_bus_event type=completed session_id=rs-xxxx

# Step 3c: Layer 3 — TR pane 中的 CC 输出
kaku cli get-text --pane-id $TOP_RIGHT | tail -20
# 期望：
#   kaku-runtime-tc3-ok
#   ❯                  ← CC 等待下一条指令
```

**内部触发链（informational）**：

```
inject
  → LarkGateway.InjectMessageSync
  → AgentCoordinator（LLM 决策）
  → [如果 agent 有 runtime_session 工具] → POST /api/runtime/sessions
  → Runtime.CreateSession + StartSession(parentPaneID=$TOP_RIGHT)
  → ClaudeCodeAdapter: split TR → 新 CC pane
  → CC 启动，携带 RUNTIME_SESSION_ID + RUNTIME_HOOKS_URL
  → CC 执行 echo → PostToolUse hook → notify_runtime.sh
  → POST /api/hooks/runtime → heartbeat event → bus → log
  → CC 结束 → Stop(end_turn) → completed event → 飞书通知
```

---

## TC-4 Stall 检测（观察 LeaderAgent）

**目标**：注入"会卡住"的请求，60s 后 StallDetector 触发，LeaderAgent 自动决策。

```bash
# Step 1: inject 一个不明确/会卡的任务
inject "帮我启动一个编程 session，先什么都不做，等我说再动" 30

# Step 2: 等待 stall 阈值（60s）
sleep 75

# Step 3b: Layer 2
tail -100 ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event" | grep -E "stalled|stall"
# 期望：type=stalled session_id=rs-xxxx

tail -100 ~/code/elephant.ai/logs/alex-service.log | grep "leader"
# 期望：leader: session rs-xxxx decision=INJECT ...
```

---

## 执行方式

### 方式 A：通过 skill（推荐）

```
/kaku-runtime-test
/kaku-runtime-test TC-3
```

### 方式 B：直接运行脚本

```bash
# 全套
KAKU_PARENT_PANE=$TOP_RIGHT bash scripts/test/kaku-runtime-e2e.sh

# 单用例
KAKU_PARENT_PANE=$TOP_RIGHT bash scripts/test/kaku-runtime-e2e.sh TC-3
```

### 方式 C：手动逐步（调试用）

按各 TC 的 bash 命令逐步执行，每步观察 BL 日志和 TR pane 输出。

---

## 验收标准汇总

| # | 层 | 验证点 | 通过条件 |
|---|---|---|---|
| 1 | L1 | inject 返回 | `.error` 空，`.replies` 非空 |
| 2 | L1 | TC-1 reply 内容 | 含 "OK" 或 "好" |
| 3 | L1 | TC-2 reply 内容 | 含数字 |
| 4 | L2 | agent 日志轨迹 | log 有 TaskExecution/tool_use |
| 5 | L2 | heartbeat 事件 | `type=heartbeat` 每次 tool use |
| 6 | L2 | completed 事件 | `type=completed` 在 CC 结束时 |
| 7 | L2 | stall 事件（TC-4） | `type=stalled` 60s 后出现 |
| 8 | L3 | TR pane 有 CC | `kaku cli get-text $TOP_RIGHT` 含 CC 输出 |
| 9 | 清理 | pane 恢复 | 测试后 `kaku cli list` 恢复原数量 |
