# 集成测试通用方案

> 适用于所有经过对话管道（LarkGateway → Agent → 外部执行）的功能特性。

---

## 核心模型

**起点永远是对话注入**。每条测试用例模拟一条飞书消息进入系统，
通过三个正交的验证层确认系统行为：

```
┌─────────────────────────────────────────────────┐
│           POST /api/dev/inject (port 9090)       │
│              ← 所有测试的唯一入口                  │
└───────────────────────┬─────────────────────────┘
                        │
          ┌─────────────▼─────────────┐
          │     LarkGateway           │
          │     AgentCoordinator      │
          │     任意后端执行路径        │
          └──────┬───────────┬────────┘
                 │           │
    ┌────────────▼──┐   ┌────▼──────────────┐
    │ Layer 1        │   │ Layer 2           │
    │ inject 响应    │   │ 日志执行轨迹       │
    │（同步 reply）  │   │（async log trace） │
    └───────────────┘   └───────────────────┘
                 │
    ┌────────────▼──────────────────────┐
    │ Layer 3：外部系统状态             │
    │ Kaku GUI / DB / File / HTTP API   │
    └───────────────────────────────────┘
```

### 三层验证说明

| 层 | 验证内容 | 工具 | 典型检查 |
|---|---|---|---|
| **Layer 1** inject 响应 | Agent 在对话中的同步回复 | `jq .replies` | 有 `ReplyMessage`，内容含关键词 |
| **Layer 2** 日志轨迹 | 内部执行路径是否走对 | `grep` log | 关键日志行、事件顺序、时序 |
| **Layer 3** 外部状态 | 副作用是否发生 | `curl`/`kaku cli`/CLI | API 状态、文件、pane 内容 |

不同功能需要哪几层视情况而定。例如：
- 纯对话问答 → Layer 1 即可
- Agent Teams 任务 → Layer 1 + Layer 2
- Kaku Runtime session → Layer 1 + Layer 2 + Layer 3（GUI）

---

## 基础设施

### 端口

| 服务 | 地址 | 用途 |
|---|---|---|
| HTTP API | `http://localhost:8080` | 生产 API，含 webhooks |
| Debug 服务 | `http://localhost:9090` | `POST /api/dev/inject`，本地测试专用 |

### inject 函数

```bash
# 通用 inject 函数（复制到每个测试脚本）
INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
SENDER_ID="${SENDER_ID:-ou_e2e_test}"
TIMEOUT="${TIMEOUT:-120}"

inject() {
  local TEXT="$1"
  local CHAT_ID="${2:-oc_e2e_default}"
  local TIMEOUT_S="${3:-$TIMEOUT}"
  curl -s -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\":            $(printf '%s' "$TEXT" | jq -Rs .),
      \"chat_id\":         \"$CHAT_ID\",
      \"chat_type\":       \"p2p\",
      \"sender_id\":       \"$SENDER_ID\",
      \"timeout_seconds\": $TIMEOUT_S
    }"
}

# 检查 reply 包含关键词
assert_reply_contains() {
  local RESP="$1" KEYWORD="$2"
  echo "$RESP" | jq -r '.replies[].content' | grep -qi "$KEYWORD" \
    && echo "PASS: reply contains '$KEYWORD'" \
    || { echo "FAIL: reply missing '$KEYWORD'"; echo "$RESP" | jq '.replies'; return 1; }
}

# 检查无 error 字段
assert_no_error() {
  local RESP="$1"
  local ERR; ERR=$(echo "$RESP" | jq -r '.error // empty')
  [[ -z "$ERR" ]] \
    && echo "PASS: no error" \
    || { echo "FAIL: error='$ERR'"; return 1; }
}
```

### 日志监控

```bash
# 三条监控命令——开三个 pane 或用 tmux split
tail -f ~/alex-service.log                                    # 全量 INFO 日志
tail -f ~/alex-service.log | grep "runtime_bus_event"        # runtime 事件
tail -f ~/alex-service.log | grep -E "TaskExecution|session" # 任务生命周期
```

---

## Pane 生命周期管理

### 规则

1. **测试前**：列出当前 pane，kill 上次遗留的测试 pane
2. **测试中**：每个测试用一个独立 tab（`kaku cli spawn`）
3. **测试后**：用例完成后立即 kill 该 tab 的所有 pane

### 标准 Pane 操作

```bash
# 列出所有 pane（查找遗留）
kaku cli list

# 创建独立测试 tab（返回 pane ID）
TEST_PANE=$(kaku cli spawn --window-id 0 \
  --cwd /Users/bytedance/code/elephant.ai -- bash -l)
kaku cli set-tab-title --pane-id $TEST_PANE "TC-N 测试名"

# 读取 pane 输出
kaku cli get-text --pane-id $TEST_PANE | tail -20
kaku cli get-text --pane-id $TEST_PANE --start-line -100  # 滚动历史

# 注入文本（不提交）+ 提交
kaku cli send-text --pane-id $TEST_PANE "命令"
kaku cli send-text --no-paste --pane-id $TEST_PANE $'\r'

# 测试后清理
kaku cli kill-pane --pane-id $TEST_PANE 2>/dev/null || true
```

### 推荐布局（4grid）

```
┌──────────────────┬──────────────────┐
│   TL (当前 CC)   │   TR (测试 pane) │
├──────────────────┼──────────────────┤
│  BL (日志监控)   │  BR (inject 脚本) │
└──────────────────┴──────────────────┘
```

```bash
KAKU_PANE_ID=$KAKU_PANE_ID bash scripts/kaku/layout.sh 4grid \
  --cwd /Users/bytedance/code/elephant.ai
# → TOP_LEFT=X TOP_RIGHT=Y BOT_LEFT=Z BOT_RIGHT=W

kaku cli send-text --pane-id $BOT_LEFT \
  "tail -f ~/alex-service.log | grep -E 'runtime_bus|TaskExecution'"
kaku cli send-text --no-paste --pane-id $BOT_LEFT $'\r'
```

---

## 测试用例结构模板

```bash
tc_N_feature_name() {
  local CASE="TC-N"
  echo "=== $CASE: 功能描述 ==="

  # ── 前置条件 ──────────────────────────────
  # 建议：检查服务、pane 清理

  # ── Step 1: 注入对话消息 ──────────────────
  local RESP
  RESP=$(inject "你的指令文本" "chat_id" 120)

  # ── Step 2: Layer 1 — inject 响应 ─────────
  assert_no_error "$RESP"
  assert_reply_contains "$RESP" "关键词"

  # ── Step 3: Layer 2 — 日志轨迹 ────────────
  sleep 2  # 等异步事件写入日志
  grep "期望的日志关键词" ~/alex-service.log | tail -5

  # ── Step 4: Layer 3 — 外部状态 ────────────
  # curl / kaku cli get-text / 文件检查

  # ── 清理 ──────────────────────────────────
  kaku cli kill-pane --pane-id $TEST_PANE 2>/dev/null || true

  echo "$CASE: PASS"
}
```

---

## 测试脚本规范

所有集成测试脚本放在 `scripts/test/` 目录，遵循统一规范：

```bash
#!/usr/bin/env bash
# scripts/test/<feature>-e2e.sh
#
# <Feature> E2E 集成测试
#
# 用法：
#   ./scripts/test/<feature>-e2e.sh              # 全套
#   ./scripts/test/<feature>-e2e.sh --case TC-1  # 单个用例
#   ./scripts/test/<feature>-e2e.sh --dry-run    # 列出用例
#
# 依赖：server running on :8080/:9090, curl, jq

set -euo pipefail

INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
TIMEOUT="${TIMEOUT:-120}"
COOLDOWN="${COOLDOWN:-3}"
TMPDIR_RUN="/tmp/<feature>_e2e_$(date +%s)"
mkdir -p "$TMPDIR_RUN"

TOTAL=0; PASS=0; FAIL=0
FILTER_CASE=""
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --case)    FILTER_CASE="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --url)     INJECT_URL="$2"; shift 2 ;;
    *)         echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# ── 通用 helpers ────────────────────────────────────────────
log() { echo "[$(date +%H:%M:%S)] $*"; }
inject() { ... }            # 见上方
assert_reply_contains() { ... }
assert_no_error() { ... }

run_case() {
  local CASE_ID="$1" FN="$2"
  [[ -n "$FILTER_CASE" && "$FILTER_CASE" != "$CASE_ID" ]] && { ((SKIP++)); return; }
  [[ $DRY_RUN -eq 1 ]] && { echo "  $CASE_ID"; return; }
  ((TOTAL++))
  log "Running $CASE_ID..."
  if $FN; then ((PASS++)); else ((FAIL++)); fi
  sleep "$COOLDOWN"
}

# ── 测试用例 ───────────────────────────────────────────────
tc_1() { ... }
tc_2() { ... }

# ── 执行顺序 ───────────────────────────────────────────────
run_case "TC-1" tc_1
run_case "TC-2" tc_2

# ── 汇总 ───────────────────────────────────────────────────
echo ""
echo "Results: PASS=$PASS FAIL=$FAIL / TOTAL=$TOTAL"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
```

---

## 已有测试套件

| 套件 | 脚本 | 文档 | 覆盖功能 |
|---|---|---|---|
| Agent Teams | `scripts/test_agents_teams_e2e.sh` | `docs/guides/agents-teams-testing.md` | 对话驱动多 agent 协作 |
| Kaku Runtime | `scripts/test/kaku-runtime-e2e.sh`（待创建） | `docs/guides/kaku-runtime-integration-test.md` | CC/Codex session 生命周期 + hooks |

---

## 通用验收标准

| 维度 | 标准 |
|---|---|
| inject 成功 | HTTP 200，`error` 字段为空 |
| 有意义的回复 | `replies` 不为空，包含 `ReplyMessage` 或预期 emoji |
| 无回归 | 现有用例 PASS 率不下降 |
| 外部副作用 | Layer 3 验证通过（如适用） |
| 清理完整 | 测试后 `kaku cli list` 无遗留测试 pane |
| 耗时合理 | 单用例 < 5min（多 agent 任务 < 15min） |

---

## 故障排查

### inject 返回 500 / timeout

```bash
# 检查服务健康
curl -s http://localhost:8080/health | jq .
curl -s http://localhost:9090/healthz

# 重启 backend（非 CC 环境）
CLAUDECODE= alex dev restart backend
```

### Layer 2 日志缺失

```bash
# 确认日志路径
lsof -p $(lsof -i :8080 -Fp | head -1 | tr -d p) | grep "\.log"

# 低级别日志未开启
grep "log_level" configs/*.yaml
```

### Layer 3 Kaku pane 无内容

```bash
# 确认 Kaku 运行
kaku cli list

# 确认 pane ID 正确（从 list 输出中读取）
kaku cli get-text --pane-id $PANE --start-line -50
```

### Pane 遗留清理

```bash
# 批量 kill 多余的 pane（保留当前 pane）
KEEP=$KAKU_PANE_ID
kaku cli list | awk 'NR>1 {print $3}' | grep -v "^$KEEP$" | while read PANE; do
  kaku cli kill-pane --pane-id $PANE 2>/dev/null || true
done
```
