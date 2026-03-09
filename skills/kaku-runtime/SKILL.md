---
name: kaku-runtime
description: >
  Kaku terminal pane 控制与 runtime session 管理技能。
  包含 kaku cli 正确用法（send-text/--no-paste/split-pane）、
  布局预置、CC/Codex 启动序列、以及 alex runtime session 生命周期命令。
triggers:
  intent_patterns:
    - "kaku|kaku cli|split.?pane|send.?text|no.?paste|runtime session|启动CC|启动claude|pane|面板布局"
  context_signals:
    keywords:
      - "kaku"
      - "split-pane"
      - "send-text"
      - "--no-paste"
      - "pane-id"
      - "KAKU_PANE_ID"
      - "runtime session"
      - "CLAUDECODE"
      - "layout.sh"
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 500
cooldown: 10
capabilities:
  - kaku_pane_control
  - kaku_layout_presets
  - runtime_session_lifecycle
  - cc_launch_sequence
governance_level: medium
activation_mode: auto
---

# kaku-runtime

Kaku 终端面板控制与多 session 运行时管理。

## ⚠️ 关键规则（违反会静默失败）

### 规则 1：提交必须用 `--no-paste $'\r'`，不能用 `\n`

```bash
# ✅ 正确 — 触发真实回车键
kaku cli send-text --pane-id $PANE "你的任务"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# ❌ 错误 — \n 在交互式 CLI（CC/Codex）中不会触发提交
kaku cli send-text --pane-id $PANE "你的任务\n"
kaku cli send-text --no-paste --pane-id $PANE $'\n'
```

### 规则 2：启动 CC 前必须 `unset CLAUDECODE`

```bash
# ✅ 正确
kaku cli send-text --pane-id $PANE "unset CLAUDECODE && claude --dangerously-skip-permissions"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# ❌ 错误 — 嵌套 session 会报错退出
kaku cli send-text --pane-id $PANE "claude --dangerously-skip-permissions"
```

### 规则 3：创建 pane 用 `split-pane`，不用 `spawn`

```bash
# ✅ 正确 — 挂在现有窗口，pane 不会消失
PANE=$(kaku cli split-pane --pane-id $PARENT --bottom --percent 65 --cwd $DIR -- zsh -l)

# ❌ 错误 — spawn 的 pane 在命令退出后消失
kaku cli spawn --cwd $DIR -- zsh -l
```

---

## 1. Pane 基础操作

```bash
# 查看所有 pane（输出列：WINID TABID PANEID WORKSPACE SIZE TITLE CWD）
kaku cli list

# 拆分 pane（返回新 pane ID）
# 方向标志：--bottom / --top / --left / --right（--horizontal 等同 --right）
PANE=$(kaku cli split-pane --pane-id $PARENT --bottom --percent 65 --cwd $DIR -- zsh -l)

# 注入文本（不提交，bracketed-paste 模式）
kaku cli send-text --pane-id $PANE "文本内容"

# 提交（⚠️ $'\r' 不是 $'\n'）
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# 注入并提交（shell 命令）
kaku cli send-text --pane-id $PANE "ls -la"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# 读取屏幕（当前可见区域）
kaku cli get-text --pane-id $PANE | tail -20

# 读取滚动历史（负数 = 向上滚动，例如 -100 表示 100 行前）
kaku cli get-text --pane-id $PANE --start-line -100

# 聚焦 / 关闭
kaku cli activate-pane --pane-id $PANE
kaku cli kill-pane --pane-id $PANE

# 设置 tab 标题（用 --pane-id 通过 pane 找到所在 tab）
kaku cli set-tab-title --pane-id $PANE "任务名称"
# 或直接指定 tab ID
kaku cli set-tab-title --tab-id $TAB "任务名称"
```

---

## 2. 布局预置（一键多 pane）

```bash
# 四方格 2×2 — 新独立窗口（推荐：不与用户当前窗口重叠）
eval $(bash scripts/kaku/layout.sh 4grid --new-window --cwd $DIR \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')

# 四方格 — 当前窗口新 tab
eval $(bash scripts/kaku/layout.sh 4grid --new-tab --cwd $DIR \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')

# 四方格 — 从指定 pane 分裂（用于已有 pane 的场景）
eval $(bash scripts/kaku/layout.sh 4grid --pane-id $P --cwd $DIR \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')

# 其他预置
bash scripts/kaku/layout.sh 2h   --new-window --cwd $DIR  # 上下两行，新窗口
bash scripts/kaku/layout.sh 2v   --pane-id $P --cwd $DIR  # 左右两列
bash scripts/kaku/layout.sh 3col --pane-id $P --cwd $DIR  # 左中右
bash scripts/kaku/layout.sh 1+2  --pane-id $P --cwd $DIR  # 主+底部双监控
```

---

## 3. 发送文本 + 自动回车（一行搞定）

```bash
# ✅ 推荐：用 send.sh wrapper，自动追加 Enter
bash scripts/kaku/send.sh --pane-id $PANE "你的命令"

# 等效的两步写法（老写法，不推荐，容易漏 $'\r'）
kaku cli send-text --pane-id $PANE "你的命令"
kaku cli send-text --no-paste --pane-id $PANE $'\r'

# 只注入不回车（粘贴到 CC 输入框，等待手动 submit）
bash scripts/kaku/send.sh --pane-id $PANE --no-submit "待补充的内容"
```

---

## 4. 启动 Claude Code（两种方式）

### 方式 A：通过 Runtime API（推荐，全自动）

LLM 只需一个 API 调用，所有步骤（split pane / unset CLAUDECODE / 等待 CC 启动 / 注入 goal / enter）全部自动完成：

> **⚠️ `parent_pane_id` 是必填字段**，省略会返回 400 Bad Request。
> 用 `-1` 表示「只创建 session 记录，不启动 CC pane」（调试/测试用）。
> 用实际 pane ID（如 `$KAKU_PANE_ID`）表示在该 pane 旁边 split 出一个 CC 子 pane 并启动 CC。

```bash
# 通过 HTTP API（:9090）— parent_pane_id 必传
curl -s -X POST http://localhost:9090/api/runtime/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "member": "claude_code",
    "goal": "你的任务描述",
    "work_dir": "/path/to/project",
    "parent_pane_id": 25
  }'
# 返回：{"id":"rs-xxx","state":"running",...}
# ❌ 错误：省略 parent_pane_id → 400 Bad Request

# 或通过 CLI（等价）
alex runtime session start \
  --member claude_code \
  --goal "你的任务描述" \
  --work-dir . \
  --parent-pane-id $KAKU_PANE_ID
```

### 方式 B：launch-cc.sh（一行搞定，手动场景推荐）

```bash
# 一行启动 CC：split pane + env + unset + claude + 注入 goal + Enter
PANE=$(bash scripts/kaku/launch-cc.sh \
  --parent-pane $KAKU_PARENT_PANE \
  --goal "你的任务描述" \
  --work-dir /path/to/project)
echo "CC started in pane $PANE"
```

### 方式 C：逐步手动 kaku CLI（仅调试用）

```bash
# Step 0: 确保 CC hooks 已注册（幂等，可重复执行）
# 这一步保证 notify_runtime.sh 在 ~/.claude/settings.json 中注册，
# 即使不走 runtime API，CC 的每次工具调用也能上报到 runtime bus。
bash scripts/cc_hooks/notify_runtime.sh --ensure-registered

# Step 1: 拆分 pane
PANE=$(kaku cli split-pane --pane-id $PARENT --bottom --percent 70 --cwd $WORKDIR -- zsh -l)

# Step 2: 设置 runtime 环境变量（如有已知 session ID 则填入）
bash scripts/kaku/send.sh --pane-id $PANE \
  "export RUNTIME_SESSION_ID=<id> RUNTIME_HOOKS_URL=http://localhost:9090"

# Step 3: 启动 CC（注意：unset 不加斜线，是 bash 内建命令）
# ✅ 正确：unset CLAUDECODE
# ❌ 错误：/unset CLAUDECODE（/unset 是路径，bash 找不到）
bash scripts/kaku/send.sh --pane-id $PANE \
  "unset CLAUDECODE && claude --dangerously-skip-permissions"

# Step 4: 等待 CC 欢迎界面（约 3s）
sleep 3

# Step 5: 注入目标并回车
bash scripts/kaku/send.sh --pane-id $PANE "你的任务描述"
```

**快速诊断**（检查 hooks 状态）：
```bash
bash scripts/cc_hooks/notify_runtime.sh --ensure-registered
cat ~/.claude/settings.json | jq '.hooks | keys'
```

---

## 4. 启动 Codex

```bash
PANE=$(kaku cli split-pane --pane-id $PARENT --bottom --percent 65 --cwd $WORKDIR -- zsh -l)
kaku cli send-text --pane-id $PANE "codex exec --full-auto -- '你的任务'"
kaku cli send-text --no-paste --pane-id $PANE $'\r'
```

---

## 5. Runtime Session 生命周期

```bash
# 启动 session（CC 在新 pane 中运行）— 两种等价方式
# CLI
alex runtime session start \
  --member claude_code \
  --goal "任务描述" \
  --work-dir . \
  --parent-pane-id $KAKU_PANE_ID

# HTTP API（:9090）
curl -s -X POST http://localhost:9090/api/runtime/sessions \
  -H "Content-Type: application/json" \
  -d '{"member":"claude_code","goal":"任务描述","work_dir":".","parent_pane_id":25}'

# 列出所有 session
alex runtime session list
curl -s http://localhost:9090/api/runtime/sessions | jq .

# 只看运行中
alex runtime session list --state running

# 查看 session 详情（JSON）
alex runtime session status <session-id>
curl -s http://localhost:9090/api/runtime/sessions/<id> | jq .

# 注入文本（解封 stalled session）
alex runtime session inject --id <id> --message "请继续"

# 停止 session
alex runtime session stop --id <id>
```

环境变量：
- `KAKU_PANE_ID` — 自动传入 `--parent-pane-id`
- `KAKU_STORE_DIR` — 持久化目录（默认 `~/.kaku/sessions`）

---

## 6. 检测 CC 完成状态

```bash
# 轮询直到 bash $ 提示符出现（CC 退出信号）
# macOS bash 提示符格式：hostname:dir user$  或  user@host dir %
while true; do
  LAST=$(kaku cli get-text --pane-id $PANE | tail -3)
  if echo "$LAST" | grep -qE '\$\s*$|%\s*$|bash-[0-9]'; then
    echo "CC session ended"
    break
  fi
  sleep 3
done
```

CC 屏幕状态特征：
- `❯ ` — 等待输入（needs_input）
- `⏺` 开头行滚动 — 正在执行（heartbeat）
- bash `$` 或 zsh `%` 提示符结尾 — CC 进程已退出（completed/failed）

> **注意**：macOS bash 提示符是 `hostname:dir user$` 格式，不是裸 `^$`。
> 用 `\$\s*$` 匹配行尾 `$`（bash）或 `%\s*$` 匹配 zsh 提示符。

---

## 7. 事件监控

```bash
# 实时监控所有 session 事件
tail -f ~/.kaku/sessions/*.events.jsonl | jq -r '"\(.type) \(.session_id)"'

# 查看单个 session 历史
cat ~/.kaku/sessions/<id>.events.jsonl | jq .

# 筛选 stall 事件
cat ~/.kaku/sessions/<id>.events.jsonl | jq 'select(.type == "stalled")'
```

---

## 推荐工作流（四方格 + 两个 session）

```bash
# 1. 四方格布局
KAKU_PANE_ID=$KAKU_PANE_ID bash scripts/kaku/layout.sh 4grid --cwd .
# TL=当前 TR=12 BL=13 BR=14

# 2. 右上角跑 CC session
alex runtime session start --member claude_code \
  --goal "分析代码结构" --work-dir . --parent-pane-id 12

# 3. 右下角跑第二个 session
alex runtime session start --member codex \
  --goal "实现分析结果" --work-dir . --parent-pane-id 14

# 4. 左下角监控事件
kaku cli send-text --pane-id 13 \
  "tail -f ~/.kaku/sessions/*.events.jsonl | jq -r '.type + \" \" + .session_id'"
kaku cli send-text --no-paste --pane-id 13 $'\r'
```
