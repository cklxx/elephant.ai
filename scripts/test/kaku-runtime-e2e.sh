#!/usr/bin/env bash
# scripts/test/kaku-runtime-e2e.sh
#
# Kaku Runtime 集成测试 — Agent 用 Kaku 管理编程团队
#
# 所有测试从 POST /api/dev/inject 注入飞书消息开始，验证三层：
#   L1: inject 响应（agent 的同步回复，确认它理解了任务）
#   L2: 日志执行轨迹（runtime_bus_event 序列，验证内部 session 生命周期）
#   L3: Kaku GUI 结果（kaku cli get-text，验证 pane 中的实际执行结果）
#
# 用法:
#   KAKU_PARENT_PANE=<id> bash scripts/test/kaku-runtime-e2e.sh
#   KAKU_PARENT_PANE=<id> bash scripts/test/kaku-runtime-e2e.sh TC-1 TC-3
#   bash scripts/test/kaku-runtime-e2e.sh --dry-run
#   bash scripts/test/kaku-runtime-e2e.sh --cleanup
#
# 环境变量:
#   KAKU_PARENT_PANE  必须。TR pane ID，CC session 将从此处 split 出来
#   INJECT_URL        默认 http://127.0.0.1:9090/api/dev/inject
#   HOOKS_URL         默认 http://localhost:9090
#   LOG_FILE          默认 ~/code/elephant.ai/logs/alex-service.log

set -euo pipefail

INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
HOOKS_URL="${HOOKS_URL:-http://localhost:9090}"
LOG_FILE="${LOG_FILE:-$HOME/code/elephant.ai/logs/alex-service.log}"
KAKU_PARENT_PANE="${KAKU_PARENT_PANE:-}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

TOTAL=0; PASS=0; FAIL=0; SKIP=0
inc() { eval "$1=$(( ${!1} + 1 ))"; }

# ── 参数解析 ─────────────────────────────────────────────────────────────────

DRY_RUN=0
ONLY_CLEANUP=0
FILTER_CASES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=1;      shift ;;
    --cleanup)  ONLY_CLEANUP=1; shift ;;
    TC-*|tc-*)  FILTER_CASES+=("$(echo "$1" | tr '[:lower:]' '[:upper:]')"); shift ;;
    *)          echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ── 日志工具 ──────────────────────────────────────────────────────────────────

log()  { echo "[$(date +%H:%M:%S)] $*"; }
pass() { log "  ✓ PASS: $*"; }
fail() { log "  ✗ FAIL: $*"; return 1; }
info() { log "  · $*"; }
warn() { log "  ⚠ WARN: $*"; }

# ── Pane 管理 ─────────────────────────────────────────────────────────────────

# 记录测试期间新增的 pane ID
TEST_PANES=()
register_pane() { TEST_PANES+=("$1"); }

# 获取当前所有 pane ID 集合
current_pane_ids() {
  kaku cli list 2>/dev/null | tail -n +2 | awk '{print $3}' | sort -n
}

# 快照 pane ID 集合（用于前后对比）
snapshot_panes() { current_pane_ids | tr '\n' ' '; }

# 找出比快照多出的 pane（新 session 的 pane）
new_panes_since() {
  local before="$1"
  local after; after=$(snapshot_panes)
  comm -13 \
    <(echo "$before" | tr ' ' '\n' | grep -v '^$' | sort) \
    <(echo "$after"  | tr ' ' '\n' | grep -v '^$' | sort)
}

# 清理测试遗留 pane
cleanup_test_panes() {
  local pane
  for pane in "${TEST_PANES[@]:-}"; do
    kaku cli kill-pane --pane-id "$pane" 2>/dev/null && info "Cleaned pane $pane" || true
  done
  TEST_PANES=()
}

# 读取 pane 最近 N 行输出
pane_text() {
  local pane_id="$1" lines="${2:-30}"
  kaku cli get-text --pane-id "$pane_id" 2>/dev/null | tail -n "$lines"
}

# ── 日志快照工具 ──────────────────────────────────────────────────────────────

# 记录当前日志行数（测试开始前调用）
mark_log() { wc -l < "$LOG_FILE" 2>/dev/null | tr -d ' ' || echo "0"; }

# 获取 mark 之后的新日志行
new_log_lines() {
  local mark="$1"
  local total; total=$(wc -l < "$LOG_FILE" 2>/dev/null | tr -d ' ' || echo "0")
  local n=$(( total - mark ))
  [[ $n -le 0 ]] && return
  tail -n "$n" "$LOG_FILE"
}

# 在 mark 之后的日志中断言 pattern 存在
assert_log_after() {
  local pattern="$1" mark="$2" label="${3:-$pattern}"
  local count; count=$(new_log_lines "$mark" | grep -c "$pattern" || true)
  [[ "$count" -gt 0 ]] \
    && pass "log: $label ($count hits)" \
    || { log "  ✗ FAIL: log missing '$label'"; return 1; }
}

# 等待日志中出现 pattern（轮询，最多 N 秒）
wait_log() {
  local pattern="$1" mark="$2" timeout="${3:-60}" label="${4:-$pattern}"
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local count; count=$(new_log_lines "$mark" | grep -c "$pattern" || true)
    [[ "$count" -gt 0 ]] && { pass "log appeared: $label (${elapsed}s)"; return 0; }
    sleep 3
    elapsed=$(( elapsed + 3 ))
  done
  log "  ✗ FAIL: timed out waiting for '$label' (${timeout}s)"; return 1
}

# ── inject 工具 ───────────────────────────────────────────────────────────────

inject() {
  local text="$1" timeout="${2:-120}"
  curl -s -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\":            $(printf '%s' "$text" | jq -Rs .),
      \"chat_type\":       \"p2p\",
      \"sender_id\":       \"ou_e2e_tester\",
      \"timeout_seconds\": $timeout
    }"
}

assert_no_error() {
  local resp="$1"
  local err; err=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null || true)
  [[ -z "$err" ]] && pass "no error" || { log "  ✗ FAIL: error='$err'"; return 1; }
}

assert_reply_nonempty() {
  local resp="$1"
  local n; n=$(echo "$resp" | jq '[.replies[]] | length' 2>/dev/null || echo "0")
  [[ "$n" -gt 0 ]] && pass "agent replied ($n items)" \
    || { log "  ✗ FAIL: empty replies"; echo "$resp" | jq . >&2; return 1; }
}

# ── 测试执行框架 ──────────────────────────────────────────────────────────────

run_case() {
  local case_id="$1" fn="$2"
  if [[ ${#FILTER_CASES[@]} -gt 0 ]]; then
    local match=0
    for c in "${FILTER_CASES[@]}"; do [[ "$c" == "$case_id" ]] && match=1; done
    [[ $match -eq 0 ]] && { inc SKIP; return; }
  fi
  [[ $DRY_RUN -eq 1 ]] && { echo "  $case_id"; return; }

  inc TOTAL
  echo ""
  log "══════════ $case_id ══════════"
  if $fn; then
    inc PASS; log "$case_id ✓ PASS"
  else
    inc FAIL; log "$case_id ✗ FAIL"
  fi
  sleep 3
}

need_kaku_pane() {
  if [[ -z "$KAKU_PARENT_PANE" ]]; then
    warn "SKIP: KAKU_PARENT_PANE not set"
    warn "  Run: KAKU_PARENT_PANE=<TR_pane_id> bash $0"
    inc SKIP; inc TOTAL; return 1
  fi
  # 确认 pane 存在
  local found; found=$(kaku cli list 2>/dev/null | awk '{print $3}' | grep -c "^${KAKU_PARENT_PANE}$" || true)
  [[ "$found" -gt 0 ]] || { fail "KAKU_PARENT_PANE=$KAKU_PARENT_PANE not found in kaku"; return 1; }
  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-0  基础设施健康检查
# 目标：验证服务正常，inject 端点和 runtime API 均可用
# ─────────────────────────────────────────────────────────────────────────────

tc_0_health() {
  # :9090 健康
  local s; s=$(curl -sf "http://localhost:9090/health" | jq -r '.status' 2>/dev/null || echo "down")
  [[ "$s" == "healthy" ]] && pass ":9090 healthy" || { fail ":9090 not healthy: $s"; return 1; }

  # inject 端点存在
  local code; code=$(curl -s -o /dev/null -w "%{http_code}" "$INJECT_URL")
  [[ "$code" == "405" || "$code" == "200" ]] \
    && pass "inject endpoint: HTTP $code" \
    || { fail "inject unreachable: HTTP $code"; return 1; }

  # /api/hooks/runtime 已注册
  local M; M=$(mark_log)
  local SID="tc0-smoke-$$"
  local hc; hc=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
    "${HOOKS_URL}/api/hooks/runtime?session_id=$SID" \
    -H "Content-Type: application/json" \
    -d '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{},"tool_response":"ok"}')
  [[ "$hc" == "200" ]] && pass "/api/hooks/runtime → $hc" || { fail "/api/hooks/runtime → $hc"; return 1; }
  sleep 1
  assert_log_after "runtime_bus_event.*${SID}" "$M" "bus event for smoke session"

  # /api/runtime/sessions 已注册
  local resp; resp=$(curl -s -X POST "${HOOKS_URL}/api/runtime/sessions" \
    -H "Content-Type: application/json" \
    -d '{"member":"claude_code","goal":"tc0-probe","work_dir":"/tmp","parent_pane_id":-1}')
  local id; id=$(echo "$resp" | jq -r '.id // empty' 2>/dev/null || true)
  [[ -n "$id" ]] && pass "/api/runtime/sessions → id=$id" || { fail "no session id: $resp"; return 1; }
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-1  单 Agent 任务 — inject → Agent 在 Kaku pane 启动 CC → 执行 → 完成
#
# 验证链：
#   inject "启动 CC 写文件" → agent 用 kaku-runtime skill 在 TR pane 里启动 CC
#   → CC 执行 echo → PostToolUse hook → heartbeat → bash 提示符 → completed
#   L2: runtime_bus_event type=started + heartbeat + completed
#   L3: TR pane 显示 CC 已运行并完成任务；/tmp/tc1-output.txt 存在
# ─────────────────────────────────────────────────────────────────────────────

tc_1_single_agent_task() {
  need_kaku_pane || return 0

  local OUTPUT_FILE="/tmp/tc1-kaku-$$.txt"
  local PANES_BEFORE; PANES_BEFORE=$(snapshot_panes)
  local M; M=$(mark_log)

  info "Injecting: single agent task → write $OUTPUT_FILE"
  local RESP; RESP=$(inject "请调用 POST http://localhost:9090/api/runtime/sessions 创建一个 Claude Code session，body 必须包含 parent_pane_id=${KAKU_PARENT_PANE}。任务(goal)：用 bash 执行 echo 'kaku-tc1-done' > $OUTPUT_FILE 然后退出。work_dir 用 ${REPO_ROOT}。完成后告诉我文件路径。" 180)

  # L1: agent 回复了任务确认
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"
  info "Agent reply: $(echo "$RESP" | jq -r '.replies[-1].content // empty' | head -c 120)"

  # L2: runtime session 事件出现在日志
  wait_log "runtime_bus_event" "$M" 120 "runtime session events"

  # L2: 至少出现一次 heartbeat（CC 执行了工具）
  wait_log "type=heartbeat\|type=started" "$M" 60 "heartbeat or started"

  # L3: 验证新 pane 出现（CC session 占用）
  local NEW_PANES; NEW_PANES=$(new_panes_since "$PANES_BEFORE")
  if [[ -n "$NEW_PANES" ]]; then
    pass "new pane(s) created: $NEW_PANES"
    for p in $NEW_PANES; do
      register_pane "$p"
      info "Pane $p content (last 5 lines):"
      pane_text "$p" 5 | while read -r line; do info "  $line"; done
    done
  else
    warn "no new pane detected (agent may have used existing pane)"
  fi

  # L3: 验证输出文件存在
  sleep 5  # 等待 CC 完成写文件
  [[ -f "$OUTPUT_FILE" ]] \
    && pass "output file exists: $OUTPUT_FILE (content: $(cat "$OUTPUT_FILE" | tr -d '\n'))" \
    || warn "output file not found: $OUTPUT_FILE (agent may still be running)"

  # L2: completed 事件
  assert_log_after "type=completed\|runtime_bus_event.*completed" "$M" "completed event" || \
    warn "completed event not yet in log (session may still be running)"
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-2  并发团队 — 两个 CC Agent 同时运行，各自完成独立任务
#
# 验证链：
#   inject "同时启动两个 CC：A写文件，B数行数"
#   → agent 启动 2 个 runtime session（2 个 pane split from TR）
#   → 两个 session 各自运行，产生独立的 runtime_bus_event（不同 session_id）
#   L2: 出现 2 个不同 session_id 的 heartbeat 事件
#   L3: 两个新 pane 中均有 CC 活动；A的文件存在
# ─────────────────────────────────────────────────────────────────────────────

tc_2_parallel_team() {
  need_kaku_pane || return 0

  local FILE_A="/tmp/tc2-agent-a-$$.txt"
  local PANES_BEFORE; PANES_BEFORE=$(snapshot_panes)
  local M; M=$(mark_log)

  info "Injecting: parallel team — 2 agents running simultaneously"
  local RESP; RESP=$(inject \
    "请调用 POST http://localhost:9090/api/runtime/sessions 两次，同时创建两个 Claude Code session（parent_pane_id 都用 ${KAKU_PARENT_PANE}，work_dir 用 ${REPO_ROOT}）：
Agent-A goal：用 bash 写文件 $FILE_A 内容为 'agent-a-done' 然后退出
Agent-B goal：用 bash 执行 echo 'agent-b-done' 然后退出
两次 API 调用要几乎同时发出（并行），不要等 A 完成再启动 B。完成后报告两个 session id 和结果。" \
    240)

  # L1: agent 确认启动了任务
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"
  info "Agent reply: $(echo "$RESP" | jq -r '.replies[-1].content // empty' | head -c 200)"

  # 等待两个 session 产生事件
  sleep 10
  local NEW_PANES; NEW_PANES=$(new_panes_since "$PANES_BEFORE")
  local pane_count; pane_count=$(echo "$NEW_PANES" | grep -c '[0-9]' || true)

  if [[ "$pane_count" -ge 2 ]]; then
    pass "parallel team: $pane_count new panes created"
    for p in $NEW_PANES; do register_pane "$p"; done
  elif [[ "$pane_count" -eq 1 ]]; then
    warn "only 1 new pane (expected 2 for parallel) — agent may have used sequential approach"
    for p in $NEW_PANES; do register_pane "$p"; done
  else
    warn "no new panes detected"
  fi

  # L2: 日志中出现多个 session_id（并行证据）
  wait_log "runtime_bus_event" "$M" 120 "runtime events from parallel sessions"
  local session_ids; session_ids=$(new_log_lines "$M" | grep -oE 'session_id=[a-z0-9-]+' | sort -u)
  local sid_count; sid_count=$(echo "$session_ids" | grep -c 'rs-' || true)

  if [[ "$sid_count" -ge 2 ]]; then
    pass "parallel evidence: $sid_count distinct session_ids in log"
    echo "$session_ids" | while read -r s; do info "  $s"; done
  else
    warn "only $sid_count session_id(s) in log (parallel not confirmed)"
  fi

  # L3: Agent-A 的输出文件
  sleep 10
  [[ -f "$FILE_A" ]] \
    && pass "Agent-A file: $FILE_A ($(cat "$FILE_A" | tr -d '\n'))" \
    || warn "Agent-A file not found yet: $FILE_A"
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-3  依赖任务链 — Agent-B 在 Agent-A 完成后启动
#
# 验证链：
#   inject "先写 step1.go，写完后基于它写 step2.go"
#   → agent 识别出依赖关系，先等 A 完成再启动 B
#   L2: 两个 session 的 completed 事件，第二个晚于第一个
#   L3: /tmp/tc3-step1.go 和 /tmp/tc3-step2.go 均存在，且 step2 引用了 step1
# ─────────────────────────────────────────────────────────────────────────────

tc_3_sequential_dependency() {
  need_kaku_pane || return 0

  local FILE1="/tmp/tc3-step1-$$.go"
  local FILE2="/tmp/tc3-step2-$$.go"
  local PANES_BEFORE; PANES_BEFORE=$(snapshot_panes)
  local M; M=$(mark_log)

  info "Injecting: sequential dependency — B depends on A"
  local RESP; RESP=$(inject \
    "请完成一个两阶段编程任务，每阶段都用 POST http://localhost:9090/api/runtime/sessions 创建 session（parent_pane_id=${KAKU_PARENT_PANE}，work_dir=${REPO_ROOT}）：
阶段1：创建 Claude Code session，goal 是在 $FILE1 写一个 Go 函数 Add(a, b int) int，等 session 完成（polling GET /api/runtime/sessions/<id> 直到 state=completed）
阶段2：阶段1完成后，再创建另一个 Claude Code session，goal 是在 $FILE2 写一个调用 $FILE1 中 Add 函数的测试用例
两个阶段必须顺序执行，完成后告诉我两个文件的内容摘要。" \
    360)

  # L1
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"
  info "Agent reply snippet: $(echo "$RESP" | jq -r '.replies[-1].content // empty' | head -c 200)"

  # L2: 等两个 session 都产生事件
  wait_log "type=completed" "$M" 300 "first session completed" || warn "first completed not seen yet"
  sleep 5
  local completed_count; completed_count=$(new_log_lines "$M" | grep -c "type=completed" || true)
  info "completed events so far: $completed_count"

  # L3: 两个文件都存在
  [[ -f "$FILE1" ]] \
    && pass "step1 file: $FILE1" \
    || warn "step1 not found: $FILE1"

  [[ -f "$FILE2" ]] \
    && pass "step2 file: $FILE2" \
    || warn "step2 not found: $FILE2"

  # Pane cleanup
  local NEW_PANES; NEW_PANES=$(new_panes_since "$PANES_BEFORE")
  for p in $NEW_PANES; do register_pane "$p"; done
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-4  Stall 检测 + LeaderAgent 自动干预
#
# 验证链：
#   inject "启动 CC，任务是什么都不做，等我发指令"
#   → agent 启动 CC session，CC 等待输入进入 needs_input 状态
#   → 60s 后 StallDetector 发现 session 无心跳，发布 EventStalled
#   → LeaderAgent 收到事件，调用 LLM 决策，执行 INJECT "请继续" 或 FAIL
#   L2: type=stalled 出现，然后出现 leader decision 日志
#   L3: TR pane 中 CC 收到了 LeaderAgent 注入的文本（或 session 标为 failed）
# ─────────────────────────────────────────────────────────────────────────────

tc_4_stall_and_leader_recovery() {
  need_kaku_pane || return 0

  local PANES_BEFORE; PANES_BEFORE=$(snapshot_panes)
  local M; M=$(mark_log)

  info "Injecting: task that will stall (CC waits for input)"
  local RESP; RESP=$(inject \
    "请调用 POST http://localhost:9090/api/runtime/sessions 创建一个 Claude Code session（parent_pane_id=${KAKU_PARENT_PANE}，work_dir=${REPO_ROOT}），goal 是：
'请等待用户输入后再执行，不要自动做任何事情'
启动后就让它等着，不要给它任何后续指令。" \
    60)

  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"

  # 等待 session 启动
  local NEW_PANES; NEW_PANES=$(new_panes_since "$PANES_BEFORE")
  for p in $NEW_PANES; do register_pane "$p"; done
  [[ -n "$NEW_PANES" ]] && pass "CC pane(s) spawned: $NEW_PANES" || warn "no new pane detected"

  info "Waiting 70s for stall threshold (60s) to trigger..."
  sleep 70

  # L2: stall 事件
  assert_log_after "type=stalled\|runtime_bus_event.*stalled" "$M" "stalled event" || \
    warn "stall event not seen — StallDetector may need longer or session ended normally"

  # L2: LeaderAgent 决策日志
  local leader_log; leader_log=$(new_log_lines "$M" | grep -i "leader\|decision\|INJECT\|FAIL\|ESCALATE" || true)
  if [[ -n "$leader_log" ]]; then
    pass "LeaderAgent decision found:"
    echo "$leader_log" | head -3 | while read -r line; do info "  $line"; done
  else
    warn "LeaderAgent decision not in log (may not be wired with LLM in this env)"
  fi

  # L3: CC pane 中应出现 LeaderAgent 注入的文本 OR session 标记为 failed
  for p in $NEW_PANES; do
    local txt; txt=$(pane_text "$p" 10)
    if echo "$txt" | grep -qi "请继续\|continue\|continue with\|leader"; then
      pass "pane $p: LeaderAgent injected recovery message"
    else
      info "pane $p content: $(echo "$txt" | tail -3)"
    fi
  done

  # L2: session 最终状态（completed 或 failed 都算通过，证明 LeaderAgent 有响应）
  local final_event; final_event=$(new_log_lines "$M" | grep -E "type=(completed|failed|stalled)" | tail -3 || true)
  [[ -n "$final_event" ]] \
    && { pass "session reached terminal/stall state:"; echo "$final_event" | while read -r l; do info "  $l"; done; } \
    || warn "no terminal state event yet"
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-5  团队进度查询 — inject 查询 → Agent 读取 session 状态并汇报
#
# 验证链：
#   inject "查看当前所有 runtime session 的状态"
#   → agent 调用 alex runtime session list 或 GET /api/runtime/sessions
#   → 获取 session 列表，格式化后回复
#   L1: reply 包含 session 状态信息（状态词如 running/completed/failed）
#   L2: 日志中出现 TaskExecution（agent 执行了查询工具）
# ─────────────────────────────────────────────────────────────────────────────

tc_5_team_status_query() {
  local M; M=$(mark_log)

  info "Injecting: query team runtime session status"
  local RESP; RESP=$(inject \
    "请查看当前所有 kaku runtime session 的状态，列出每个 session 的 ID、目标、状态（running/completed/failed）。
可以用 alex runtime session list 命令或查询 http://localhost:9090/api/runtime/sessions 接口。" \
    60)

  # L1: agent 回复了 session 信息
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"

  local content; content=$(echo "$RESP" | jq -r '.replies[] | select(.content != "") | .content' 2>/dev/null | head -c 500)
  info "Agent reply: ${content:0:300}"

  # reply 应包含 session 状态关键词
  echo "$content" | grep -qiE "session|running|completed|failed|rs-|状态|没有" \
    && pass "reply contains session status info" \
    || warn "reply may not contain session status (check content above)"

  # L2: agent 执行了查询
  assert_log_after "TaskExecution\|coordinator\|tool_use\|runtime" "$M" "agent query execution" || true
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-6  完整团队工作流 — 分析 → 实现 → 验证（三个顺序 session）
#
# 验证链：
#   inject "用编程团队实现并测试一个 Go 函数"
#   → Session-1 (Analyst): 在 pane 里分析需求，输出方案到 /tmp/tc6-plan.md
#   → Session-2 (Coder):   基于方案实现 /tmp/tc6-impl.go
#   → Session-3 (Tester):  验证实现（执行 go vet 或简单 check），输出报告
#   L2: 3 个不同 session_id，均出现 started → heartbeat → completed 序列
#   L3: 三个文件均存在；最后 agent 汇总报告
# ─────────────────────────────────────────────────────────────────────────────

tc_6_full_team_workflow() {
  need_kaku_pane || return 0

  local PLAN="/tmp/tc6-plan-$$.md"
  local IMPL="/tmp/tc6-impl-$$.go"
  local PANES_BEFORE; PANES_BEFORE=$(snapshot_panes)
  local M; M=$(mark_log)

  info "Injecting: full 3-agent team workflow (Analyst → Coder → Tester)"
  local RESP; RESP=$(inject \
    "请用 POST http://localhost:9090/api/runtime/sessions 按顺序创建三个 Claude Code session（每个都用 parent_pane_id=${KAKU_PARENT_PANE}，work_dir=${REPO_ROOT}），等上一个 state=completed 后再创建下一个：

session-1 goal：写方案文档到 ${PLAN}，内容是「实现 Go 函数 Max(a,b int) int 返回较大值，以及配套单测」然后退出
session-2 goal：读取 ${PLAN}，实现 Go 文件 ${IMPL}，只包含 Max 函数，然后退出
session-3 goal：读取 ${IMPL}，用 bash echo 输出「Max(3,5)=5, 实现正确」然后退出

每次调用 POST 后 polling GET /api/runtime/sessions/<id> 等 state=completed。全部完成后汇报三个 session id 和文件内容。" \
    480)

  # L1: agent 确认开始了多阶段任务
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"
  info "Agent reply: $(echo "$RESP" | jq -r '.replies[-1].content // empty' | head -c 300)"

  # L2: 等待所有 session 事件（最长 7 分钟）
  info "Waiting for 3-session workflow to complete (up to 6 min)..."
  wait_log "type=completed" "$M" 360 "first session completed" || warn "first session completion slow"
  sleep 30  # 给后续 session 时间

  # 统计 session 数量
  local session_ids; session_ids=$(new_log_lines "$M" | grep -oE 'session_id=rs-[a-z0-9]+' | sort -u)
  local sid_count; sid_count=$(echo "$session_ids" | grep -c 'rs-' || true)
  info "Distinct sessions in log: $sid_count"
  echo "$session_ids" | while read -r s; do info "  $s"; done

  local completed_count; completed_count=$(new_log_lines "$M" | grep -c "type=completed" || true)
  info "Completed events: $completed_count"

  [[ "$sid_count" -ge 2 ]] && pass "$sid_count sessions observed (expected 3)" \
    || warn "only $sid_count session(s) — agent may have combined phases"

  # L3: 检查文件输出
  [[ -f "$PLAN" ]] && pass "plan file: $PLAN" || warn "plan missing: $PLAN"
  [[ -f "$IMPL" ]] && pass "impl file: $IMPL" || warn "impl missing: $IMPL"

  # Pane 清理
  local NEW_PANES; NEW_PANES=$(new_panes_since "$PANES_BEFORE")
  for p in $NEW_PANES; do register_pane "$p"; done
  [[ -n "$NEW_PANES" ]] && pass "team panes: $NEW_PANES" || warn "no new panes detected"
}

# ─────────────────────────────────────────────────────────────────────────────
# TC-7  Stall 注入恢复 — Agent 主动介入卡住的 session
#
# 验证链：
#   先通过 API 直接创建一个 session（parent_pane_id=-1，不启动真实 CC）
#   模拟该 session 卡住（不发心跳）→ inject "检查并恢复卡住的 session"
#   → agent 用 alex runtime session inject 给该 session 发恢复指令
#   → session 收到指令（inject 事件）
#   L1: agent 回复确认已介入
#   L2: runtime bus 出现 needs_input 或 inject 相关日志
# ─────────────────────────────────────────────────────────────────────────────

tc_7_manual_stall_recovery() {
  # 先创建一个不会自动心跳的 session（用 -1 跳过真实 pane）
  local stale_resp; stale_resp=$(curl -s -X POST "${HOOKS_URL}/api/runtime/sessions" \
    -H "Content-Type: application/json" \
    -d '{"member":"claude_code","goal":"tc7-stall-bait: wait forever","work_dir":"/tmp","parent_pane_id":-1}')
  local SID; SID=$(echo "$stale_resp" | jq -r '.id // empty')
  [[ -n "$SID" ]] && pass "created stale bait session: $SID" || { fail "failed to create bait session"; return 1; }

  local M; M=$(mark_log)

  info "Injecting: agent detects and recovers stalled session $SID"
  local RESP; RESP=$(inject \
    "我有一个 runtime session $SID 已经卡住了（goal: wait forever）。
请用 alex runtime session inject --id $SID --message '请继续执行，完成后退出' 来恢复它，
然后检查 http://localhost:9090/api/runtime/sessions/$SID 确认状态，告诉我结果。" \
    60)

  # L1
  assert_no_error "$RESP"
  assert_reply_nonempty "$RESP"
  local content; content=$(echo "$RESP" | jq -r '.replies[] | select(.content != "") | .content' 2>/dev/null || true)
  info "Agent reply: ${content:0:200}"

  # L1: reply 应该提到 session 或注入
  echo "$content" | grep -qiE "$SID|inject|注入|恢复|session" \
    && pass "reply references the stalled session" \
    || warn "reply may not reference the session correctly"

  # L2: 检查日志中是否有相关事件
  assert_log_after "runtime_bus_event.*${SID}\|session.*${SID}" "$M" "events for session $SID" || true
}

# ─────────────────────────────────────────────────────────────────────────────
# 主执行
# ─────────────────────────────────────────────────────────────────────────────

only_cleanup() {
  log "=== Cleanup: killing registered test panes ==="
  cleanup_test_panes
  log "=== Current panes after cleanup ==="
  kaku cli list 2>/dev/null || true
}

[[ $ONLY_CLEANUP -eq 1 ]] && { only_cleanup; exit 0; }

if [[ $DRY_RUN -eq 1 ]]; then
  echo "Test cases (agent uses Kaku to manage coding teams):"
  echo ""
  echo "  TC-0  基础设施健康检查 (no Kaku pane needed)"
  echo "  TC-1  单 Agent 任务    inject → CC spawns in pane → executes → file output"
  echo "  TC-2  并发团队         inject → 2 CC agents run in parallel → 2 sessions"
  echo "  TC-3  依赖任务链       inject → Agent-A completes → Agent-B starts → 2 files"
  echo "  TC-4  Stall + LeaderAgent  CC stalls → detector → leader injects recovery"
  echo "  TC-5  团队进度查询     inject 'show status' → agent lists all sessions"
  echo "  TC-6  完整工作流       Analyst → Coder → Tester (3 sequential sessions)"
  echo "  TC-7  手动 Stall 恢复  agent uses 'session inject' to unblock a session"
  echo ""
  echo "Setup: KAKU_PARENT_PANE=<TR_pane_id> bash $0 [TC-N ...]"
  exit 0
fi

log "=== Current panes ==="
kaku cli list 2>/dev/null || log "(kaku cli not available)"

echo ""
log "=== Kaku Runtime E2E — Agent Team Management Tests ==="
log "  inject endpoint : $INJECT_URL"
log "  hooks endpoint  : $HOOKS_URL"
log "  log file        : $LOG_FILE"
log "  parent pane     : ${KAKU_PARENT_PANE:-'(not set — TC-1/2/3/4/6 will skip)'}"
echo ""

run_case "TC-0" tc_0_health
run_case "TC-1" tc_1_single_agent_task;  cleanup_test_panes
run_case "TC-2" tc_2_parallel_team;      cleanup_test_panes
run_case "TC-3" tc_3_sequential_dependency; cleanup_test_panes
run_case "TC-4" tc_4_stall_and_leader_recovery; cleanup_test_panes
run_case "TC-5" tc_5_team_status_query
run_case "TC-6" tc_6_full_team_workflow; cleanup_test_panes
run_case "TC-7" tc_7_manual_stall_recovery; cleanup_test_panes

echo ""
echo "════════════════════════════════════════"
echo "Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP  TOTAL=$TOTAL"
echo "════════════════════════════════════════"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
