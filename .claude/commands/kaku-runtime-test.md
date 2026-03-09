# /kaku-runtime-test — Kaku Runtime 集成测试

测试 Agent 用 Kaku 管理编程团队的完整端到端流程。
**起点永远是 inject**（模拟飞书消息），不直接调用内部 CLI。

**参数**: `$ARGUMENTS`（可选，如 `TC-1`、`TC-2 TC-4`；不填则全跑）

---

## 测试场景概览

Agent 收到飞书消息后，通过 kaku-runtime skill 在 Kaku pane 中启动 CC/Codex 编程团队，
监控其执行，在卡住时介入，最终汇报结果。测试验证这整条链路。

```
用户飞书消息（inject）
  ↓
Agent（LLM + kaku-runtime skill）
  ↓  kaku cli split-pane → CC/Codex 启动
Kaku pane（TR 区域）
  ↓  CC 执行工具 → PostToolUse hook → notify_runtime.sh
runtime bus（:9090/api/hooks/runtime）
  ↓  heartbeat / stalled / completed 事件
StallDetector（60s 无心跳）→ LeaderAgent（决策：INJECT/FAIL）
  ↓
飞书通知（completed/failed）
```

---

## Step 0 — 建立四宫格布局

```bash
# 从当前 Claude pane 分裂三个辅助 pane
eval $(bash scripts/kaku/layout.sh 4grid \
  --pane-id "$KAKU_PANE_ID" \
  --cwd /Users/bytedance/code/elephant.ai \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')

# BL：实时日志（runtime bus 事件 + 工具调用）
kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/code/elephant.ai/logs/alex-service.log | grep -E 'runtime_bus_event|TaskExecution|leader'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'
```

```
┌──────────────────────┬──────────────────────┐
│   TL: Claude (当前)  │   TR: CC team panes  │
│  测试结果在此汇报     │  agent splits here   │
├──────────────────────┼──────────────────────┤
│   BL: 日志监控        │   BR: （空闲）       │
│  runtime_bus_event   │                      │
└──────────────────────┴──────────────────────┘
```

---

## Step 1 — 运行测试

```bash
# 告知测试脚本：CC session 应 split 自 TR pane
KAKU_PARENT_PANE=$TOP_RIGHT \
bash scripts/test/kaku-runtime-e2e.sh $ARGUMENTS 2>&1 | tee /tmp/kaku-e2e-$(date +%H%M).log
```

---

## 测试用例

| TC | 场景 | 关键验证 |
|---|---|---|
| TC-0 | 基础设施健康检查 | HTTP 200 + bus 事件 + session API |
| TC-1 | 单 Agent 写文件 | 新 pane 出现 + heartbeat + 文件存在 |
| TC-2 | 两个 Agent 并行 | 2 个不同 session_id + 2 个新 pane |
| TC-3 | A→B 依赖任务链 | 两文件存在，B 晚于 A 的 completed |
| TC-4 | Stall + LeaderAgent | `type=stalled` + LeaderAgent 决策日志 |
| TC-5 | 查询团队状态 | reply 含 session 状态信息 |
| TC-6 | 分析→实现→验证 | 3 个 session_id + 3 个输出文件 |
| TC-7 | 手动注入恢复卡住的 session | agent 使用 `session inject` 命令 |

---

## Step 2 — 观察验证（三层）

**Layer 1（BL pane 实时可见）**:

```bash
# runtime bus 事件序列
tail -f ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event"
# 期望：started → heartbeat × N → completed
```

**Layer 2（TR pane 中 CC 实际执行）**:

```bash
# 找到新增的 CC pane ID
kaku cli list

# 读取 CC pane 输出
kaku cli get-text --pane-id <CC_PANE_ID> | tail -20
# 期望：⏺ 工具调用行 + 最终输出 + bash 提示符（CC 结束标志）
```

**Layer 3（文件/状态验证）**:

```bash
# TC-1 文件
ls -la /tmp/tc1-kaku-*.txt

# TC-6 三阶段文件
ls -la /tmp/tc6-*.md /tmp/tc6-*.go

# 所有 session 当前状态
curl -s http://localhost:9090/api/runtime/sessions | jq '.[].state'
```

---

## Step 3 — 清理

```bash
# Kill 三个辅助 pane（TR/BL/BR）
kaku cli kill-pane --pane-id $TOP_RIGHT 2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_LEFT  2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_RIGHT 2>/dev/null || true

# 清理临时文件
rm -f /tmp/tc{1,2,3,6}-*.txt /tmp/tc{3,6}-*.go /tmp/tc6-*.md

kaku cli list
```

---

## 验收标准

| 层 | 验证点 | 期望 |
|---|---|---|
| L1 | inject 响应 | `.error` 空，`.replies` 非空，内容提及任务 |
| L2 | runtime bus 序列 | started → heartbeat(s) → completed |
| L2 | 并行证据（TC-2） | 2 个不同 session_id |
| L2 | stall 检测（TC-4） | `type=stalled` 在 60s 后出现 |
| L2 | LeaderAgent 决策（TC-4） | log 中有 `leader decision=INJECT/FAIL` |
| L3 | 新 pane 出现 | `kaku cli list` 多出对应 CC pane |
| L3 | CC pane 有执行轨迹 | `get-text` 含 `⏺` 工具调用记录 |
| L3 | 任务输出文件存在 | `/tmp/tc*.go` `/tmp/tc*.txt` |
| 清理 | pane 数量恢复 | 测试后 `kaku cli list` 恢复原数量 |
