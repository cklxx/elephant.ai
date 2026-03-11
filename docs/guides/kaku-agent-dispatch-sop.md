# Kaku CLI Agent Dispatch SOP

Updated: 2026-03-11

Leader agent（CC 主会话）通过 kaku CLI 指挥其他 agent（CC / Codex / Kimi）在独立 pane 中并行执行任务。

---

## 1. 一键分发（推荐）

### 1.1 使用 launch 脚本

```bash
# 启动 CC 并注入任务（全自动：split → unset CLAUDECODE → 启动 → 等待 → 注入 goal）
PANE=$(bash scripts/kaku/launch-cc.sh \
  --parent-pane $KAKU_PANE_ID \
  --goal "你的任务描述" \
  --work-dir /path/to/project)

# 启动 Codex 并注入任务（全自动：split → 启动 → 跳过更新 → 等待 → 注入 goal）
PANE=$(bash scripts/kaku/launch-codex.sh \
  --parent-pane $KAKU_PANE_ID \
  --goal "你的任务描述" \
  --work-dir /path/to/project)
```

### 1.2 批量分发（dispatch.sh）

```bash
# 通过 YAML 配置文件批量分发
cat > /tmp/tasks.yaml <<'EOF'
- agent: cc
  goal: 在 worktree 中执行 blocker-gitsignal 计划
- agent: codex
  goal: 为 runtime 添加集成测试
- agent: cc
  goal: rebase observability 分支并合并
- agent: codex
  goal: 审计 Go 源码注释
EOF

bash scripts/kaku/dispatch.sh \
  --layout 4grid \
  --new-window \
  --cwd $(pwd) \
  --config /tmp/tasks.yaml
```

### 1.3 一键监控（monitor.sh）

```bash
# 单次检查所有 pane
bash scripts/kaku/monitor.sh --panes "7 8 9 10"

# 持续监控（每 30s 刷新）
bash scripts/kaku/monitor.sh --panes "7 8 9 10" --watch 30

# 自动发现所有 pane
bash scripts/kaku/monitor.sh
```

---

## 2. 手动分发（逐步操作）

### 2.1 创建布局

```bash
eval $(bash scripts/kaku/layout.sh 4grid --new-window --cwd $(pwd) \
  | grep -E "TOP_RIGHT|BOT_LEFT|BOT_RIGHT" \
  | awk '{print "export "$0}')
```

布局选项：`4grid`（2x2）、`2h`（上下）、`2v`（左右）、`3col`（三列）、`1+2`（主+双监控）。

### 2.2 启动 Claude Code

```bash
# 方式 A: launch-cc.sh（推荐，一步到位）
PANE=$(bash scripts/kaku/launch-cc.sh \
  --parent-pane $TOP_RIGHT --goal "任务" --work-dir .)

# 方式 B: 手动分步
bash scripts/kaku/send.sh --pane-id $PANE \
  "unset CLAUDECODE && claude --dangerously-skip-permissions"
sleep 5
bash scripts/kaku/send.sh --pane-id $PANE "任务描述"
```

### 2.3 启动 Codex

```bash
# 方式 A: launch-codex.sh（推荐，自动处理更新提示）
PANE=$(bash scripts/kaku/launch-codex.sh \
  --parent-pane $BOT_LEFT --goal "任务" --work-dir .)

# 方式 B: 手动分步
bash scripts/kaku/send.sh --pane-id $PANE \
  "codex --dangerously-bypass-approvals-and-sandbox"
sleep 5  # 等待就绪 + 跳过更新提示
bash scripts/kaku/send.sh --pane-id $PANE "任务描述"
```

> **不要用** `codex exec --full-auto`：沙箱限制导致无法操作 `.git`。

### 2.4 Kimi（仅调研）

```bash
alex team run --template kimi_research \
  --goal "调研主题。仅分析，不修改代码。"
alex team status --all
```

---

## 3. 监控与状态判断

### 3.1 手动监控

```bash
for pane in 7 8 9 10; do
  echo "=== Pane $pane ==="
  kaku cli get-text --pane-id $pane | tail -8
  echo
done
```

### 3.2 完成信号

| Agent | 完成信号 | 空闲信号 |
|-------|----------|----------|
| CC | `✻ Cooked for` | 空 `❯` 提示符 |
| Codex | `tokens used` + bash `$` | `left ·` 提示行 |
| Kimi | `alex team status` → `role_completed` | — |

### 3.3 CC 状态详细

| 屏幕特征 | 状态 |
|----------|------|
| `❯ ` 空行 | 等待输入（可注入新任务） |
| `⏺` / `●` / `Working` | 正在执行 |
| `✻ Cooked for` | 本轮完成 |
| bash `$` / zsh `%` | CC 已退出 |

---

## 4. 干预与收尾

```bash
# 注入额外指令
bash scripts/kaku/send.sh --pane-id $PANE "补充指令"

# 中断 CC
kaku cli send-text --no-paste --pane-id $PANE $'\x1b'

# 终止 pane
kaku cli kill-pane --pane-id $PANE

# 读取滚动历史
kaku cli get-text --pane-id $PANE --start-line -100

# 批量清理
for pane in 7 8 9 10; do kaku cli kill-pane --pane-id $pane 2>/dev/null; done
```

---

## 5. 可用脚本清单

| 脚本 | 用途 |
|------|------|
| `scripts/kaku/layout.sh` | 创建 pane 布局 |
| `scripts/kaku/send.sh` | 向 pane 注入文本 + 自动回车 |
| `scripts/kaku/launch-cc.sh` | 一步启动 CC 并注入任务 |
| `scripts/kaku/launch-codex.sh` | 一步启动 Codex 并注入任务 |
| `scripts/kaku/dispatch.sh` | 批量分发多任务到多 pane |
| `scripts/kaku/monitor.sh` | 批量监控 pane 状态 |

---

## 6. 注意事项

1. **CC 必须 `unset CLAUDECODE`**：否则嵌套检测拒绝启动（launch-cc.sh 已内置）
2. **Codex 用交互式**：`codex --dangerously-bypass-approvals-and-sandbox`（launch-codex.sh 已内置）
3. **Kimi 仅做调研**：智能有限，不分配代码修改任务
4. **Codex 更新提示**：启动时可能弹出更新提示需按 Enter（launch-codex.sh 自动处理）
5. **CC 启动延迟**：launch-cc.sh 自动等待 3-5s，手动操作也要等
6. **推荐 2CC + 2Codex**：CC 适合复杂/需要判断的任务，Codex 适合明确的编码任务
7. **Worktree 优先**：代码修改任务应在 goal 中要求使用 worktree
8. **关闭无用窗口**：任务完成后及时 `kill-pane` 清理
