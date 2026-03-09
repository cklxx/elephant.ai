# Kaku Runtime 集成测试通用方案

## 核心原则

**起点永远是对话**。所有测试从 `POST /api/dev/inject` 注入飞书消息开始。
测试验证的是"Agent 用 Kaku 管理编程团队"这条完整路径——不是 CLI 单元测试。

```
飞书消息（inject）
  ↓
Agent（LLM + kaku-runtime skill）
  ├─ kaku cli split-pane  → 新 pane
  ├─ CC/Codex 启动        → 在 pane 里执行任务
  └─ alex runtime session → 管理 session 生命周期
       ↓  PostToolUse hook → notify_runtime.sh
  runtime bus（heartbeat / stalled / completed）
       ↓
  StallDetector → LeaderAgent → INJECT/FAIL 决策
       ↓
  飞书通知（completed/failed）
```

**禁止**在测试步骤中直接调用 `POST /api/runtime/sessions` 或 `alex runtime session start`
来替代 agent 的行为——那些是 CLI 开发者的单元测试，不是 E2E。

---

## 四宫格布局（标准基础设施）

```
┌──────────────────────┬──────────────────────┐
│   TL: Claude（当前） │   TR: CC team panes  │
│  测试汇报、控制       │  agent 在此 split CC │
├──────────────────────┼──────────────────────┤
│   BL: 日志监控        │   BR: （空闲备用）   │
│  runtime_bus_event   │                      │
└──────────────────────┴──────────────────────┘
```

```bash
# 从当前 KAKU_PANE_ID 建立四宫格
eval $(bash scripts/kaku/layout.sh 4grid \
  --pane-id "$KAKU_PANE_ID" \
  --cwd /Users/bytedance/code/elephant.ai \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')

# BL：日志监控（保持运行）
kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/code/elephant.ai/logs/alex-service.log | grep -E 'runtime_bus_event|leader|TaskExecution'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'

# 告知测试脚本：CC session 从 TR pane split
export KAKU_PARENT_PANE=$TOP_RIGHT
```

**Pane 管理规则**：
- 测试前：不额外创建 pane，只建四宫格（4 个总）
- 测试中：agent 在 TR 区域 split 出 CC pane，测试脚本记录并在结束后清理
- 测试后：kill TR/BL/BR，恢复到测试前数量

---

## 三层验证体系

每个测试用例均按三层验证：

| 层 | 来源 | 命令 | 确认什么 |
|---|---|---|---|
| L1 | inject 同步响应 | `jq .replies` | Agent 理解了任务意图 |
| L2 | 日志异步事件 | `grep runtime_bus_event` | 内部 session 生命周期正确 |
| L3 | Kaku pane 输出 | `kaku cli get-text <CC_PANE>` | 实际执行结果符合预期 |

---

## 测试用例

### TC-1 单 Agent 任务

**场景**：inject → Agent 用 kaku-runtime skill 启动一个 CC session → CC 执行写文件 → 完成

```bash
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "请用 kaku runtime 在 pane '$TOP_RIGHT' 里启动 Claude Code，任务是：echo kaku-tc1-done > /tmp/tc1.txt 然后退出",
    "chat_type": "p2p",
    "timeout_seconds": 180
  }')

# L1: reply 确认启动
echo "$RESP" | jq '.replies[-1].content'

# L2: 等待 runtime 事件
tail -100 ~/code/elephant.ai/logs/alex-service.log | grep "runtime_bus_event"
# 期望：type=started → type=heartbeat → type=completed

# L3: 新 pane 出现，文件存在
kaku cli list
cat /tmp/tc1.txt   # → kaku-tc1-done
```

---

### TC-2 并发团队（两个 Agent 同时运行）

**场景**：inject → Agent 同时启动两个 CC session，各自完成独立任务

```bash
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "请同时启动两个 Claude Code session 并行工作：Agent-A 写 /tmp/tc2-a.txt 内容 agent-a-done，Agent-B 写 /tmp/tc2-b.txt 内容 agent-b-done，两个同时启动不要等待",
    "chat_type": "p2p",
    "timeout_seconds": 240
  }')

# L2: 两个不同 session_id 出现在日志
grep "runtime_bus_event" ~/code/elephant.ai/logs/alex-service.log | tail -20

# L3: 两个新 pane + 两个文件
kaku cli list
ls -la /tmp/tc2-*.txt
```

**判定**：日志中出现 2 个不同 `session_id=rs-xxx`，两个文件均存在。

---

### TC-3 依赖任务链（A 完成后 B 才启动）

**场景**：Agent-A 输出方案 → Agent-B 基于方案实现 → 严格顺序

```bash
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "分两个阶段（顺序执行，不能并行）：阶段1启动CC写 /tmp/tc3-plan.md 内容为「实现 Max(a,b int) int」；阶段1完成后再启动阶段2的CC读 /tmp/tc3-plan.md 并写 /tmp/tc3-impl.go 实现该函数",
    "chat_type": "p2p",
    "timeout_seconds": 360
  }')

# L2: 两个 completed 事件，第二个晚于第一个
grep "type=completed" ~/code/elephant.ai/logs/alex-service.log | tail -5

# L3: 两文件存在，step2 引用了 step1
cat /tmp/tc3-impl.go
```

---

### TC-4 Stall 检测 + LeaderAgent 自动干预

**场景**：CC session 无心跳 60s → StallDetector 发布 EventStalled → LeaderAgent 决策 INJECT/FAIL

```bash
# inject 一个会卡的任务
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "请在 pane '$TOP_RIGHT' 附近启动 Claude Code，任务是「等待用户输入，什么都不做」，启动后不要给它任何指令",
    "chat_type": "p2p",
    "timeout_seconds": 30
  }')

# 等待 70s 让 stall 触发
sleep 70

# L2: stall 事件
grep "runtime_bus_event" ~/code/elephant.ai/logs/alex-service.log | grep "stalled" | tail -3
# 期望：type=stalled session_id=rs-xxxx

# L2: LeaderAgent 决策
grep -i "leader\|decision\|INJECT\|FAIL" ~/code/elephant.ai/logs/alex-service.log | tail -5

# L3: CC pane 中收到了恢复文本（如果 LeaderAgent 选 INJECT）
kaku cli get-text --pane-id <CC_PANE> | tail -5
```

---

### TC-5 团队进度查询

**场景**：inject "查看所有 session 状态" → Agent 调用 CLI/API 汇报

```bash
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "请查看当前所有 kaku runtime session 的状态，用表格列出 ID、目标、状态",
    "chat_type": "p2p",
    "timeout_seconds": 60
  }')

echo "$RESP" | jq -r '.replies[-1].content'
# 期望：包含 session ID 列表和各自状态
```

---

### TC-6 完整团队工作流（分析→实现→验证）

**场景**：三个顺序 CC session 完成完整软件交付流程

```bash
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d '{
    "text": "请用三个顺序 Claude Code session 完成完整流程：(1)分析师写方案 /tmp/tc6-plan.md (2)开发者读方案实现 /tmp/tc6-impl.go (3)验证者运行验证输出结论。严格顺序，完成后汇报",
    "chat_type": "p2p",
    "timeout_seconds": 480
  }')

# L2: 三个 session_id，三个 completed
grep "runtime_bus_event" ~/code/elephant.ai/logs/alex-service.log | grep "rs-" | tail -20

# L3: 三个输出文件
ls -la /tmp/tc6-*.md /tmp/tc6-*.go
```

---

### TC-7 手动 Stall 恢复（Agent 主动注入卡住的 session）

**场景**：告诉 Agent 某个 session 卡住了，Agent 用 `session inject` 命令恢复它

```bash
# 先通过 API 创建一个不会自动运行的 bait session
BAIT=$(curl -s -X POST http://localhost:9090/api/runtime/sessions \
  -H "Content-Type: application/json" \
  -d '{"member":"claude_code","goal":"wait forever","work_dir":"/tmp","parent_pane_id":-1}' | jq -r .id)

# inject 让 Agent 去恢复它
RESP=$(curl -s -X POST http://localhost:9090/api/dev/inject \
  -H "Content-Type: application/json" \
  -d "{
    \"text\": \"session $BAIT 卡住了，请用 alex runtime session inject --id $BAIT --message '请继续并退出' 来恢复它，然后告诉我结果\",
    \"chat_type\": \"p2p\",
    \"timeout_seconds\": 60
  }")

echo "$RESP" | jq '.replies[-1].content'
# 期望：agent 确认已注入恢复指令
```

---

## 执行方式

### 方式 A：skill（推荐）

```
/kaku-runtime-test
/kaku-runtime-test TC-1
/kaku-runtime-test TC-4 TC-6
```

### 方式 B：脚本

```bash
# 全套
KAKU_PARENT_PANE=$TOP_RIGHT bash scripts/test/kaku-runtime-e2e.sh

# 单个用例
KAKU_PARENT_PANE=$TOP_RIGHT bash scripts/test/kaku-runtime-e2e.sh TC-4

# dry-run 查看用例列表
bash scripts/test/kaku-runtime-e2e.sh --dry-run
```

### 方式 C：逐步手动

按上述各 TC 的 curl 命令手动执行，在 BL pane 实时观察日志。

---

## 验收标准汇总

| TC | 核心验证 | 最低通过条件 |
|---|---|---|
| TC-0 | 健康 + API 可用 | HTTP 200 + bus 事件落日志 |
| TC-1 | 单 agent 执行 | 新 pane + heartbeat + 文件存在 |
| TC-2 | 并发 2 agents | 2 个不同 session_id + 2 文件 |
| TC-3 | 顺序依赖 | 2 个 completed 事件（有序）+ 2 文件 |
| TC-4 | Stall + LeaderAgent | `type=stalled` + leader 决策日志 |
| TC-5 | 状态查询 | reply 含 session 状态信息 |
| TC-6 | 3-agent 流水线 | ≥2 个 session_id + plan/impl 文件 |
| TC-7 | 手动注入恢复 | reply 确认 agent 执行了注入命令 |
