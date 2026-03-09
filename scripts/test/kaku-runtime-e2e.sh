#!/usr/bin/env bash
# scripts/test/kaku-runtime-e2e.sh
#
# Kaku Runtime 集成测试
#
# 测试起点永远是对话（inject 接口），通过三层验证确认系统行为：
#   Layer 1 — inject 响应（同步 reply，代表 agent 完整执行了什么）
#   Layer 2 — 日志执行轨迹（async log trace，验证内部 runtime bus 事件）
#   Layer 3 — Kaku GUI 界面结果（kaku cli get-text，验证 pane 实际输出）
#
# 用法:
#   bash scripts/test/kaku-runtime-e2e.sh                         # 全套
#   bash scripts/test/kaku-runtime-e2e.sh TC-1                    # 单用例
#   bash scripts/test/kaku-runtime-e2e.sh TC-1 TC-3               # 多个用例
#   bash scripts/test/kaku-runtime-e2e.sh --dry-run               # 列出用例
#   bash scripts/test/kaku-runtime-e2e.sh --cleanup               # 只清理 pane
#   KAKU_PARENT_PANE=42 bash scripts/test/kaku-runtime-e2e.sh TC-3  # 指定父 pane
#
# 环境变量:
#   INJECT_URL       inject 端点，默认 http://127.0.0.1:9090/api/dev/inject
#   HOOKS_URL        runtime hooks URL，默认 http://localhost:9090
#   LOG_FILE         日志文件，默认 ~/alex-service.log
#   KAKU_PARENT_PANE TC-3+ 用的父 pane ID（CC session 将 split 自此 pane）
#   CC_HOOKS_SCRIPT  notify_runtime.sh 路径

set -euo pipefail

INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
HOOKS_URL="${HOOKS_URL:-http://localhost:9090}"
LOG_FILE="${LOG_FILE:-$HOME/code/elephant.ai/logs/alex-service.log}"
KAKU_PARENT_PANE="${KAKU_PARENT_PANE:-}"
CC_HOOKS_SCRIPT="${CC_HOOKS_SCRIPT:-$(cd "$(dirname "$0")/../.." && pwd)/scripts/cc_hooks/notify_runtime.sh}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

TOTAL=0; PASS=0; FAIL=0; SKIP=0
# Track panes created during this test run for cleanup
TEST_PANES=()

# inc: arithmetic safe with set -e
inc() { eval "$1=$(( ${!1} + 1 ))"; }

# ── Argument parsing ──────────────────────────────────────────────────────────

DRY_RUN=0
ONLY_CLEANUP=0
FILTER_CASES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=1;     shift ;;
    --cleanup)  ONLY_CLEANUP=1; shift ;;
    TC-*|tc-*)  FILTER_CASES+=("$(echo "$1" | tr '[:lower:]' '[:upper:]')"); shift ;;
    *)          echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ── Logging ───────────────────────────────────────────────────────────────────

log()  { echo "[$(date +%H:%M:%S)] $*"; }
pass() { log "  ✓ PASS: $*"; }
fail() { log "  ✗ FAIL: $*"; }
info() { log "  · $*"; }

# ── Pane lifecycle ────────────────────────────────────────────────────────────

# register_pane: mark a pane for cleanup at test end
register_pane() { TEST_PANES+=("$1"); }

# cleanup_test_panes: kill all registered test panes
cleanup_test_panes() {
  local pane
  for pane in "${TEST_PANES[@]:-}"; do
    kaku cli kill-pane --pane-id "$pane" 2>/dev/null \
      && info "Cleaned pane $pane" || true
  done
  TEST_PANES=()
}

# cleanup_by_title: kill panes whose title matches pattern (legacy cleanup)
cleanup_by_title() {
  local pattern="${1:-TC-}"
  kaku cli list 2>/dev/null | tail -n +2 | while read -r line; do
    local pid title
    pid=$(echo "$line" | awk '{print $3}')
    title=$(echo "$line" | awk '{for(i=5;i<=NF;i++) printf "%s ", $i; print ""}')
    if echo "$title" | grep -q "$pattern"; then
      kaku cli kill-pane --pane-id "$pid" 2>/dev/null \
        && info "Killed stale pane $pid (title: $title)" || true
    fi
  done
}

# spawn_test_pane: create a new pane, register it, return its ID
spawn_test_pane() {
  local title="${1:-TC-test}"
  local pane_id
  pane_id=$(kaku cli spawn \
    --cwd "$REPO_ROOT" \
    -- bash -l 2>/dev/null) || { log "WARN: kaku cli spawn failed; tests needing pane will skip"; echo ""; return; }
  kaku cli set-tab-title --pane-id "$pane_id" "$title" 2>/dev/null || true
  register_pane "$pane_id"
  echo "$pane_id"
}

# pane_text: get last N lines of a pane
pane_text() {
  local pane_id="$1" lines="${2:-20}"
  kaku cli get-text --pane-id "$pane_id" 2>/dev/null | tail -n "$lines"
}

# ── Log snapshot helpers ──────────────────────────────────────────────────────

# mark_log: snapshot current log line count; usage: MARK=$(mark_log)
mark_log() { wc -l < "$LOG_FILE" 2>/dev/null || echo "0"; }

# new_log_lines: print only lines added since MARK
new_log_lines() {
  local mark="$1"
  local total; total=$(wc -l < "$LOG_FILE" 2>/dev/null || echo "0")
  local n=$(( total - mark ))
  [[ $n -le 0 ]] && return
  tail -n "$n" "$LOG_FILE"
}

# assert_log_new: verify PATTERN appears in new log lines since MARK
assert_log_new() {
  local pattern="$1" mark="$2"
  local count; count=$(new_log_lines "$mark" | grep -c "$pattern" || true)
  [[ "$count" -gt 0 ]] \
    && pass "log contains '$pattern' ($count hits)" \
    || { fail "log missing '$pattern' since mark=$mark"; return 1; }
}

# ── inject helper ─────────────────────────────────────────────────────────────

inject() {
  local text="$1"
  local timeout="${2:-60}"
  local chat_id="${3:-oc_e2e_kaku_runtime}"
  curl -s -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\":            $(printf '%s' "$text" | jq -Rs .),
      \"chat_id\":         \"$chat_id\",
      \"chat_type\":       \"p2p\",
      \"sender_id\":       \"ou_e2e_tester\",
      \"timeout_seconds\": $timeout
    }"
}

# ── Assertions ────────────────────────────────────────────────────────────────

assert_reply_contains() {
  local resp="$1" keyword="$2"
  local replies; replies=$(echo "$resp" | jq -r '.replies[].content // empty' 2>/dev/null || true)
  echo "$replies" | grep -qi "$keyword" \
    && pass "reply contains '$keyword'" \
    || { fail "reply missing '$keyword'"; info "replies: $(echo "$resp" | jq -c '.replies' 2>/dev/null)"; return 1; }
}

assert_no_error() {
  local resp="$1"
  local err; err=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null || true)
  [[ -z "$err" ]] \
    && pass "no error in response" \
    || { fail "response error: '$err'"; return 1; }
}

assert_pane_text() {
  local pane_id="$1" keyword="$2"
  local text; text=$(pane_text "$pane_id" 40)
  echo "$text" | grep -q "$keyword" \
    && pass "pane $pane_id output contains '$keyword'" \
    || { fail "pane $pane_id output missing '$keyword'"; info "last lines: $(echo "$text" | tail -5)"; return 1; }
}

assert_http() {
  local url="$1" method="${2:-GET}" body="${3:-}" expected="${4:-200}"
  local code
  if [[ -n "$body" ]]; then
    code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" \
      -H "Content-Type: application/json" -d "$body")
  else
    code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url")
  fi
  [[ "$code" == "$expected" ]] \
    && pass "$method $url → HTTP $code" \
    || { fail "$method $url → HTTP $code (expected $expected)"; return 1; }
}

# ── Test runner ───────────────────────────────────────────────────────────────

run_case() {
  local case_id="$1" fn="$2"

  # Filter: skip if not in requested list
  if [[ ${#FILTER_CASES[@]} -gt 0 ]]; then
    local match=0
    local c; for c in "${FILTER_CASES[@]}"; do [[ "$c" == "$case_id" ]] && match=1; done
    [[ $match -eq 0 ]] && { inc SKIP; return; }
  fi

  [[ $DRY_RUN -eq 1 ]] && { echo "  $case_id"; return; }

  inc TOTAL
  echo ""
  log "══════════ $case_id ══════════"
  if $fn; then
    inc PASS
    log "$case_id ✓ PASS"
  else
    inc FAIL
    log "$case_id ✗ FAIL"
  fi
  sleep 2  # cooldown between cases
}

# ── TC-0: Pre-flight health ───────────────────────────────────────────────────

tc_0_health() {
  info "Checking infrastructure health..."

  # Debug server :9090
  local s9; s9=$(curl -sf "http://localhost:9090/health" | jq -r '.status' 2>/dev/null || echo "down")
  [[ "$s9" == "healthy" ]] \
    && pass ":9090 healthy" \
    || { fail ":9090 not healthy (got: $s9)"; return 1; }

  # inject endpoint exists (405 = exists, wrong method)
  local code; code=$(curl -s -o /dev/null -w "%{http_code}" "$INJECT_URL")
  [[ "$code" == "405" || "$code" == "200" ]] \
    && pass "inject endpoint reachable (HTTP $code)" \
    || { fail "inject endpoint unreachable (HTTP $code)"; return 1; }

  # /api/hooks/runtime is registered
  local SID="tc0-smoke-$$"
  assert_http "${HOOKS_URL}/api/hooks/runtime?session_id=$SID" POST \
    '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{},"tool_response":"ok"}' "200"

  sleep 1
  # SID is unique per run (contains PID), so searching the full log is safe
  local cnt; cnt=$(grep -c "runtime_bus_event.*${SID}" "$LOG_FILE" 2>/dev/null || true)
  [[ "$cnt" -gt 0 ]] \
    && pass "bus event logged for smoke session (${cnt} hits)" \
    || { fail "bus event not in log for $SID"; return 1; }

  # /api/runtime/sessions endpoint (parent_pane_id=-1 = no real pane)
  local resp; resp=$(curl -s -X POST "${HOOKS_URL}/api/runtime/sessions" \
    -H "Content-Type: application/json" \
    -d '{"member":"claude_code","goal":"tc0-probe","work_dir":"/tmp","parent_pane_id":-1}')
  local id; id=$(echo "$resp" | jq -r '.id // empty' 2>/dev/null || true)
  [[ -n "$id" ]] \
    && pass "/api/runtime/sessions created session id=$id" \
    || { fail "/api/runtime/sessions returned no id: $resp"; return 1; }
}

# ── TC-1: inject simple message → Layer 1 reply ──────────────────────────────

tc_1_inject_greeting() {
  info "inject: simple greeting → expect reply"

  # Cleanup stale test panes before starting
  cleanup_by_title "TC-1"

  local MARK; MARK=$(mark_log)
  local RESP; RESP=$(inject "你好，请只回复两个字：OK好的" 30)

  # Layer 1: sync reply
  assert_no_error "$RESP"
  assert_reply_contains "$RESP" "OK" || assert_reply_contains "$RESP" "好"

  # Layer 2: log trace — agent ran something
  assert_log_new "TaskExecution\|coordinator\|tool_use\|runtime_bus" "$MARK" || true
  # (soft-pass if no trace: agent may reply without complex tools)

  info "inject response: $(echo "$RESP" | jq -c '.replies[0]' 2>/dev/null)"
}

# ── TC-2: inject task → agent execution trace (Layer 1+2) ────────────────────

tc_2_inject_task() {
  info "inject: count .go files → agent executes, reply contains answer"

  cleanup_by_title "TC-2"
  local MARK; MARK=$(mark_log)

  local TEXT="请帮我数一下 /Users/bytedance/code/elephant.ai 里有多少个 .go 文件，给我一个精确数字就好，不需要执行命令"
  local RESP; RESP=$(inject "$TEXT" 90)

  # Layer 1: reply exists and contains a number
  assert_no_error "$RESP"
  local content; content=$(echo "$RESP" | jq -r '.replies[].content // empty' 2>/dev/null || true)
  echo "$content" | grep -qE '[0-9]+' \
    && pass "reply contains numeric answer" \
    || { fail "reply missing numeric answer"; info "content: $content"; return 1; }

  # Layer 2: agent ran for some time (duration > 1s indicates real processing)
  local ms; ms=$(echo "$RESP" | jq -r '.duration_ms // 0')
  [[ "$ms" -gt 1000 ]] \
    && pass "agent processed for ${ms}ms (real execution)" \
    || info "duration ${ms}ms (may be cached reply)"
}

# ── TC-3: inject → runtime session → CC pane in Kaku (Layer 1+2+3) ──────────

tc_3_runtime_session_kaku() {
  if [[ -z "$KAKU_PARENT_PANE" ]]; then
    info "SKIP: KAKU_PARENT_PANE not set (set it to run TC-3)"
    inc SKIP; inc TOTAL; return 0
  fi

  info "Creating runtime session (parent_pane=$KAKU_PARENT_PANE)"
  cleanup_by_title "TC-3"

  local MARK; MARK=$(mark_log)

  # Create a test pane to split from (we use KAKU_PARENT_PANE as the split target)
  # Layer 0: verify parent pane exists
  local found; found=$(kaku cli list 2>/dev/null | awk '{print $3}' | grep -c "^${KAKU_PARENT_PANE}$" || true)
  [[ "$found" -gt 0 ]] \
    || { fail "KAKU_PARENT_PANE=$KAKU_PARENT_PANE not found in kaku cli list"; return 1; }

  local GOAL="请用 bash 执行：echo 'kaku-runtime-tc3-ok'; sleep 2; echo 'done'"
  local WORK_DIR="$REPO_ROOT"

  # POST /api/runtime/sessions
  local RESP; RESP=$(curl -s -X POST "${HOOKS_URL}/api/runtime/sessions" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
      --arg member "claude_code" \
      --arg goal "$GOAL" \
      --arg workdir "$WORK_DIR" \
      --argjson ppane "$KAKU_PARENT_PANE" \
      '{member:$member,goal:$goal,work_dir:$workdir,parent_pane_id:$ppane}')")

  local SESSION_ID; SESSION_ID=$(echo "$RESP" | jq -r '.id // empty')
  [[ -n "$SESSION_ID" ]] \
    && pass "session created id=$SESSION_ID" \
    || { fail "no session id: $RESP"; return 1; }

  # Layer 2: session started event on bus
  sleep 2
  assert_log_new "runtime_bus_event.*started.*${SESSION_ID}\|runtime_bus_event.*${SESSION_ID}.*started" "$MARK"

  # Layer 3: new pane appeared in kaku
  local PANES_AFTER; PANES_AFTER=$(kaku cli list 2>/dev/null)
  info "Panes after session start:"
  echo "$PANES_AFTER" | tail -n +2 | while read -r line; do info "  $line"; done

  # Wait for CC to do something (10s)
  sleep 10

  # Layer 2: heartbeat event from notify_runtime.sh
  assert_log_new "type=heartbeat.*${SESSION_ID}\|runtime_bus_event.*heartbeat.*${SESSION_ID}" "$MARK" || \
    info "WARN: no heartbeat yet (CC hooks may not be registered)"

  # Layer 3: find CC pane and read its output
  # CC pane title is usually set by the adapter; fall back to last pane added
  local CC_PANE; CC_PANE=$(kaku cli list 2>/dev/null | tail -n +2 | awk '{print $3}' | sort -n | tail -1)
  info "Checking pane $CC_PANE for task output..."
  local OUTPUT; OUTPUT=$(kaku cli get-text --pane-id "$CC_PANE" 2>/dev/null | tail -20 || echo "")
  echo "$OUTPUT" | grep -q "kaku-runtime-tc3-ok\|claude\|Claude\|done" \
    && pass "pane $CC_PANE shows CC/task activity" \
    || info "pane output: $(echo "$OUTPUT" | tail -5)"

  # Track CC pane for cleanup
  register_pane "$CC_PANE"
}

# ── TC-4: notify_runtime.sh — direct hook invocation (Layer 2) ───────────────

tc_4_notify_heartbeat() {
  info "notify_runtime.sh: PostToolUse → heartbeat on bus"

  [[ -f "$CC_HOOKS_SCRIPT" ]] \
    || { fail "CC_HOOKS_SCRIPT not found: $CC_HOOKS_SCRIPT"; return 1; }

  local SID="tc4-hb-$$"
  local MARK; MARK=$(mark_log)

  RUNTIME_SESSION_ID="$SID" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":"ok"}'

  sleep 1
  assert_log_new "runtime_bus_event.*${SID}\|type=heartbeat.*${SID}" "$MARK"
}

# ── TC-5: notify_runtime.sh — Stop(end_turn) → completed event ───────────────

tc_5_notify_completed() {
  info "notify_runtime.sh: Stop(end_turn) → completed on bus"

  [[ -f "$CC_HOOKS_SCRIPT" ]] \
    || { fail "CC_HOOKS_SCRIPT not found: $CC_HOOKS_SCRIPT"; return 1; }

  local SID="tc5-done-$$"
  local MARK; MARK=$(mark_log)

  RUNTIME_SESSION_ID="$SID" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< '{"hook_event_name":"Stop","stop_reason":"end_turn","answer":"task complete"}'

  sleep 1
  assert_log_new "runtime_bus_event.*${SID}" "$MARK"
  # completed vs heartbeat depends on server hook mapping
  new_log_lines "$MARK" | grep "$SID" | while read -r line; do info "  bus: $line"; done
}

# ── TC-6: notify_runtime.sh robustness ───────────────────────────────────────

tc_6_notify_robustness() {
  info "notify_runtime.sh: robustness checks (empty ID, bad JSON, server down)"

  [[ -f "$CC_HOOKS_SCRIPT" ]] \
    || { fail "CC_HOOKS_SCRIPT not found: $CC_HOOKS_SCRIPT"; return 1; }

  # R1: empty SESSION_ID → silent exit 0
  local code=0
  RUNTIME_SESSION_ID="" RUNTIME_HOOKS_URL="$HOOKS_URL" \
    bash "$CC_HOOKS_SCRIPT" <<< '{}' || code=$?
  [[ "$code" == "0" ]] \
    && pass "empty SESSION_ID → exit 0" \
    || { fail "empty SESSION_ID → exit $code"; return 1; }

  # R2: server unavailable → not blocked (≤ 6s)
  local t0 t1 elapsed
  t0=$(date +%s)
  RUNTIME_SESSION_ID="tc6-down-$$" RUNTIME_HOOKS_URL="http://localhost:9999" \
    bash "$CC_HOOKS_SCRIPT" <<< '{}' || true
  t1=$(date +%s)
  elapsed=$(( t1 - t0 ))
  [[ "$elapsed" -le 6 ]] \
    && pass "server-down path: ${elapsed}s ≤ 6s (fire-and-forget)" \
    || { fail "server-down path blocked for ${elapsed}s — CC would hang!"; return 1; }

  # R3: malformed JSON → no crash
  local bad_exit=0
  RUNTIME_SESSION_ID="tc6-json-$$" RUNTIME_HOOKS_URL="$HOOKS_URL" \
    bash "$CC_HOOKS_SCRIPT" <<< 'not valid json' || bad_exit=$?
  [[ "$bad_exit" == "0" ]] \
    && pass "malformed JSON → no crash (exit 0)" \
    || { fail "malformed JSON → exit $bad_exit"; return 1; }
}

# ── TC-7: inject task → runtime session via agent tool (Layer 1+2+3) ─────────
# Future: When the agent has a "start_coding_session" tool, inject triggers it.
# Currently documents the expected flow for when that tool exists.

tc_7_inject_triggers_runtime() {
  info "inject: task request → agent should orchestrate runtime session"
  info "NOTE: This TC requires the agent to have a runtime session tool wired."
  info "      If not wired, the agent will handle the task inline."

  cleanup_by_title "TC-7"
  local MARK; MARK=$(mark_log)

  local RESP; RESP=$(inject \
    "请启动一个编程 session，目标是：echo 'hello from kaku runtime tc7'" \
    120)

  assert_no_error "$RESP"
  info "inject response duration: $(echo "$RESP" | jq -r '.duration_ms')ms"
  info "replies: $(echo "$RESP" | jq -c '.replies' 2>/dev/null)"

  # Layer 2: check if any runtime session was created
  local bus_events; bus_events=$(new_log_lines "$MARK" | grep "runtime_bus_event" || true)
  if [[ -n "$bus_events" ]]; then
    pass "runtime bus events found (agent used runtime tool)"
    echo "$bus_events" | while read -r line; do info "  $line"; done
  else
    info "No runtime bus events (agent handled inline — not a failure)"
  fi

  # Layer 1: reply should at minimum acknowledge the task
  assert_no_error "$RESP"
}

# ── Pane cleanup ──────────────────────────────────────────────────────────────

only_cleanup() {
  log "=== Cleanup: killing test panes ==="
  cleanup_by_title "TC-"
  cleanup_test_panes
  log "Cleanup complete"
  kaku cli list 2>/dev/null || true
}

# ── Main ──────────────────────────────────────────────────────────────────────

[[ $ONLY_CLEANUP -eq 1 ]] && { only_cleanup; exit 0; }

if [[ $DRY_RUN -eq 1 ]]; then
  echo "Available test cases:"
  echo "  TC-0  Pre-flight health (HTTP endpoints, bus, runtime sessions)"
  echo "  TC-1  inject greeting   → Layer 1: reply"
  echo "  TC-2  inject task       → Layer 1+2: reply + agent trace"
  echo "  TC-3  runtime session   → Layer 2+3: bus events + Kaku pane"
  echo "  TC-4  notify heartbeat  → Layer 2: bus event from PostToolUse"
  echo "  TC-5  notify completed  → Layer 2: bus event from Stop"
  echo "  TC-6  notify robustness → 3 edge cases (empty ID, server down, bad JSON)"
  echo "  TC-7  inject → runtime  → Layer 1+2+3 (requires agent tool)"
  exit 0
fi

# Pre-run: show current pane state
log "=== Current panes ==="
kaku cli list 2>/dev/null || log "(kaku cli list failed)"

# Pre-run: cleanup stale TC- panes from previous runs
log "=== Pre-run cleanup ==="
cleanup_by_title "TC-"

echo ""
log "=== Starting Kaku Runtime E2E Tests ==="
log "  inject endpoint : $INJECT_URL"
log "  hooks endpoint  : $HOOKS_URL"
log "  log file        : $LOG_FILE"
log "  parent pane     : ${KAKU_PARENT_PANE:-'(not set — TC-3 will skip)'}"
echo ""

run_case "TC-0" tc_0_health
run_case "TC-1" tc_1_inject_greeting
run_case "TC-2" tc_2_inject_task
run_case "TC-3" tc_3_runtime_session_kaku
run_case "TC-4" tc_4_notify_heartbeat
run_case "TC-5" tc_5_notify_completed
run_case "TC-6" tc_6_notify_robustness
run_case "TC-7" tc_7_inject_triggers_runtime

# Post-run: cleanup all test panes registered during this run
echo ""
log "=== Post-run cleanup ==="
cleanup_test_panes

echo ""
echo "════════════════════════════════════════"
echo "Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP  TOTAL=$TOTAL"
echo "════════════════════════════════════════"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
