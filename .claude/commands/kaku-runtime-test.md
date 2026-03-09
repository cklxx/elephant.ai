# /kaku-runtime-test — Kaku Runtime 集成测试（从对话注入起点）

端到端测试 Kaku Runtime。**起点永远是 inject 接口**（模拟用户发飞书消息），
通过三层验证观察系统行为，不直接调用内部 CLI。

**参数**: `$ARGUMENTS`（可选，如 `TC-1`、`TC-2 TC-3`；不写则全跑）

---

## 测试原则

```
[1] inject → POST /api/dev/inject（模拟飞书对话）
       ↓
[2] 观察 Agent 执行轨迹（BL pane 日志）
       ↓
[3] 观察 Kaku GUI 结果（TR pane 的 CC session 输出）
```

**不允许**在测试步骤中直接调用 `alex runtime session start` 或 `POST /api/runtime/sessions`——那是 CLI 开发者的测试，不是 E2E。

---

## Step 0 — 建立四宫格测试布局

用 `scripts/kaku/layout.sh` 从当前 pane（`$KAKU_PANE_ID`）分裂出三个辅助 pane：

```bash
eval $(bash scripts/kaku/layout.sh 4grid \
  --pane-id "$KAKU_PANE_ID" \
  --cwd /Users/bytedance/code/elephant.ai \
  | grep -E "TOP_LEFT|TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | sed 's/  /\n/g' | awk '{print "export "$0}')

echo "TL=$TOP_LEFT TR=$TOP_RIGHT BL=$BOT_LEFT BR=$BOT_RIGHT"
```

```
┌──────────────────────┬──────────────────────┐
│   TL: 当前 Claude    │   TR: CC session 在此出现  │
│  (KAKU_PANE_ID)      │  (runtime 会 split 此 pane) │
├──────────────────────┼──────────────────────┤
│   BL: 日志监控        │   BR: inject 命令    │
│  tail + grep runtime │  curl 注入，看响应    │
└──────────────────────┴──────────────────────┘
```

在 BL pane 启动日志监控：

```bash
kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/code/elephant.ai/logs/alex-service.log | grep -E 'runtime_bus_event|TaskExecution'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'
```

告知 runtime：CC session 应 split 自 TR pane：

```bash
export KAKU_PARENT_PANE=$TOP_RIGHT
```

---

## Step 1 — 运行测试

在 BR pane 执行（或在当前 Claude pane 直接 Bash 调用）：

```bash
KAKU_PARENT_PANE=$TOP_RIGHT \
bash scripts/test/kaku-runtime-e2e.sh $ARGUMENTS
```

---

## 测试用例（每个从 inject 开始）

### TC-1 基础对话

```bash
# BR pane 执行
curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{"text":"你好，请只回复两个字：OK好的","chat_type":"p2p","timeout_seconds":30}' | jq .
```

- **Layer 1**（inject 响应）：`.replies[].content` 含"OK"或"好"
- **Layer 2**（BL 日志）：出现 `TaskExecution` 或 agent 处理记录

---

### TC-2 任务执行

```bash
curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{"text":"请告诉我项目里大约有多少个 .go 文件（估算即可）","chat_type":"p2p","timeout_seconds":60}' | jq .
```

- **Layer 1**：reply 含数字或估算答案
- **Layer 2**：BL 日志出现 agent tool 调用轨迹

---

### TC-3 Runtime Session（CC 在 Kaku 中运行）

```bash
curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d "{
    \"text\": \"请启动一个编程 session，任务是：echo kaku-runtime-tc3-ok，完成后告诉我结果\",
    \"chat_type\": \"p2p\",
    \"timeout_seconds\": 120
  }" | jq .
```

等待约 20s，然后三层验证：

```bash
# Layer 2：BL 日志应出现 runtime_bus_event
tail -50 ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event"
# 期望：type=heartbeat, type=completed

# Layer 3：TR pane (TOP_RIGHT) 应出现 CC session
kaku cli get-text --pane-id $TOP_RIGHT | tail -20
# 期望：CC 运行并输出 "kaku-runtime-tc3-ok"
```

---

### TC-4 Stall 检测（等 60s 以上）

注入一个"会卡住"的请求，不注入任何 goal 给 CC，等待 stall 超时：

```bash
# Layer 2：60s 后应出现
tail -100 ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event" | grep "stalled"
# 期望：type=stalled session_id=rs-xxxx
```

---

## Step 2 — 完成后清理

```bash
# Kill 三个辅助 pane（TL 即当前 Claude pane 保留）
kaku cli kill-pane --pane-id $TOP_RIGHT 2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_LEFT  2>/dev/null || true
kaku cli kill-pane --pane-id $BOT_RIGHT 2>/dev/null || true

# 确认 pane 数量恢复
kaku cli list
```

---

## 验收标准

| 层 | 检查点 | 期望 |
|---|---|---|
| L1 | inject 响应 | `.error` 为空，`.replies` 非空 |
| L1 | TC-1 reply | 含"OK"/"好" |
| L2 | runtime bus 事件 | `grep runtime_bus_event` 有输出 |
| L2 | heartbeat | `type=heartbeat` 出现（CC 每次 tool use）|
| L2 | completed | `type=completed` 出现（CC 结束） |
| L3 | TR pane 内容 | `kaku cli get-text $TOP_RIGHT` 含任务结果 |
| 清理 | pane 数量 | 测试结束后恢复到测试前数量 |
