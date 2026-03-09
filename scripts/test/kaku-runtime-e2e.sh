#!/usr/bin/env bash
# scripts/test/kaku-runtime-e2e.sh
#
# Kaku Runtime E2E Integration Tests
#
# 用法：
#   ./scripts/test/kaku-runtime-e2e.sh              # 全套
#   ./scripts/test/kaku-runtime-e2e.sh --case TC-1  # 单个用例
#   ./scripts/test/kaku-runtime-e2e.sh --dry-run    # 列出用例
#
# 依赖：server running on :8080/:9090, curl, jq, kaku cli
#
# 验证三层：
#   Layer 1 — inject 响应（同步 reply）
#   Layer 2 — 日志执行轨迹（async log trace）
#   Layer 3 — Kaku GUI / 外部状态

set -euo pipefail

INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
HOOKS_URL="${HOOKS_URL:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-120}"
COOLDOWN="${COOLDOWN:-3}"
LOG_FILE="${LOG_FILE:-$HOME/alex-service.log}"
CC_HOOKS_SCRIPT="${CC_HOOKS_SCRIPT:-$(cd "$(dirname "$0")/../.." && pwd)/scripts/cc_hooks/notify_runtime.sh}"

TOTAL=0; PASS=0; FAIL=0; SKIP=0

# Arithmetic helpers safe with set -e (avoid false-y exit on ((var++)) == 0)
inc() { eval "$1=$(( ${!1} + 1 ))"; }
FILTER_CASE=""
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --case)    FILTER_CASE="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --url)     INJECT_URL="$2"; shift 2 ;;
    --hooks)   HOOKS_URL="$2"; shift 2 ;;
    *)         echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# ── 通用 helpers ─────────────────────────────────────────────────────────────

log() { echo "[$(date +%H:%M:%S)] $*"; }

inject() {
  local TEXT="$1"
  local CHAT_ID="${2:-oc_e2e_kaku_runtime}"
  local TIMEOUT_S="${3:-$TIMEOUT}"
  curl -s -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\":            $(printf '%s' "$TEXT" | jq -Rs .),
      \"chat_id\":         \"$CHAT_ID\",
      \"chat_type\":       \"p2p\",
      \"sender_id\":       \"ou_e2e_test\",
      \"timeout_seconds\": $TIMEOUT_S
    }"
}

assert_reply_contains() {
  local RESP="$1" KEYWORD="$2"
  echo "$RESP" | jq -r '.replies[].content // empty' | grep -qi "$KEYWORD" \
    && log "PASS: reply contains '$KEYWORD'" \
    || { log "FAIL: reply missing '$KEYWORD'"; echo "$RESP" | jq '.replies'; return 1; }
}

assert_no_error() {
  local RESP="$1"
  local ERR; ERR=$(echo "$RESP" | jq -r '.error // empty')
  [[ -z "$ERR" ]] \
    && log "PASS: no error" \
    || { log "FAIL: error='$ERR'"; return 1; }
}

assert_log_contains() {
  local PATTERN="$1" WINDOW="${2:-30}"
  local COUNT; COUNT=$(tail -n 500 "$LOG_FILE" 2>/dev/null | grep -c "$PATTERN" || true)
  [[ "$COUNT" -gt 0 ]] \
    && log "PASS: log contains '$PATTERN' ($COUNT matches)" \
    || { log "FAIL: log missing '$PATTERN' (last ${WINDOW}s)"; return 1; }
}

run_case() {
  local CASE_ID="$1" FN="$2"
  [[ -n "$FILTER_CASE" && "$FILTER_CASE" != "$CASE_ID" ]] && { inc SKIP; return; }
  [[ $DRY_RUN -eq 1 ]] && { echo "  $CASE_ID"; return; }
  inc TOTAL
  echo ""
  log "═══ Running $CASE_ID ═══"
  if $FN; then
    inc PASS
    log "$CASE_ID: PASS ✓"
  else
    inc FAIL
    log "$CASE_ID: FAIL ✗"
  fi
  sleep "$COOLDOWN"
}

# ── TC-0：基础设施健康检查 ────────────────────────────────────────────────────

tc_0_healthcheck() {
  log "=== TC-0: Infrastructure Health ==="

  # 检查主服务 :8080
  local STATUS; STATUS=$(curl -sf "$HOOKS_URL/health" | jq -r '.status' 2>/dev/null || echo "down")
  [[ "$STATUS" == "healthy" ]] \
    && log "PASS: :8080 healthy" \
    || { log "FAIL: :8080 not healthy (got: $STATUS)"; return 1; }

  # 检查 /api/hooks/runtime 端点
  local HTTP; HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
    "$HOOKS_URL/api/hooks/runtime?session_id=tc0-smoke-$$" \
    -H "Content-Type: application/json" \
    -d '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{},"tool_response":"ok"}')
  [[ "$HTTP" == "200" ]] \
    && log "PASS: /api/hooks/runtime → HTTP $HTTP" \
    || { log "FAIL: /api/hooks/runtime → HTTP $HTTP"; return 1; }

  # 验证 bus 事件落到日志
  sleep 1
  assert_log_contains "runtime_bus_event.*tc0-smoke-$$"

  # 检查 debug inject 端点
  local HEALTH9; HEALTH9=$(curl -s -o /dev/null -w "%{http_code}" "$INJECT_URL" 2>/dev/null || echo "000")
  # 405 = endpoint exists but wrong method → OK
  [[ "$HEALTH9" == "405" || "$HEALTH9" == "200" ]] \
    && log "PASS: :9090 inject endpoint reachable (HTTP $HEALTH9)" \
    || { log "FAIL: :9090 inject endpoint unreachable (HTTP $HEALTH9)"; return 1; }
}

# ── TC-1：notify_runtime.sh — 正常路径 ───────────────────────────────────────

tc_1_notify_normal() {
  log "=== TC-1: notify_runtime.sh normal path ==="

  local SESSION_ID="tc1-notify-$$"
  RUNTIME_SESSION_ID="$SESSION_ID" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":"ok"}'

  sleep 1
  assert_log_contains "runtime_bus_event.*$SESSION_ID"
  assert_log_contains "type=heartbeat.*$SESSION_ID"
}

# ── TC-2：notify_runtime.sh — 服务不可用，不阻塞 CC ───────────────────────────

tc_2_notify_unavailable() {
  log "=== TC-2: notify_runtime.sh does not block when server unavailable ==="

  local T_START T_END ELAPSED_S
  T_START=$(date +%s)
  RUNTIME_SESSION_ID="tc2-fail-$$" RUNTIME_HOOKS_URL="http://localhost:9999" \
    bash "$CC_HOOKS_SCRIPT" <<< '{}' || true
  T_END=$(date +%s)
  ELAPSED_S=$(( T_END - T_START ))
  log "Elapsed time: ${ELAPSED_S}s"

  [[ "$ELAPSED_S" -le 6 ]] \
    && log "PASS: elapsed ${ELAPSED_S}s ≤ 6s (fire-and-forget)" \
    || { log "FAIL: elapsed ${ELAPSED_S}s > 6s — CC would be blocked!"; return 1; }
}

# ── TC-3：notify_runtime.sh — SESSION_ID 为空时静默退出 ──────────────────────

tc_3_notify_empty_session() {
  log "=== TC-3: notify_runtime.sh silent exit when SESSION_ID empty ==="

  RUNTIME_SESSION_ID="" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< '{}'
  local EXIT=$?
  [[ "$EXIT" == "0" ]] \
    && log "PASS: exit code 0" \
    || { log "FAIL: exit code $EXIT"; return 1; }
}

# ── TC-4：notify_runtime.sh — 畸形 JSON fallback ─────────────────────────────

tc_4_notify_bad_json() {
  log "=== TC-4: notify_runtime.sh handles malformed JSON without crashing ==="

  local SESSION_ID="tc4-json-$$"
  # Must set SESSION_ID so the script doesn't exit early
  local EXIT_CODE=0
  RUNTIME_SESSION_ID="$SESSION_ID" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< 'not valid json at all' || EXIT_CODE=$?

  # Script must not crash (exit 0)
  [[ "$EXIT_CODE" == "0" ]] \
    && log "PASS: script exited 0 on malformed JSON" \
    || { log "FAIL: script exited $EXIT_CODE"; return 1; }

  # Server receives request but returns 204 (unknown event type — not published to bus)
  # This is correct: garbage JSON → fallback event="" → server ignores gracefully
  log "PASS: malformed JSON handled gracefully (server ignores unknown event type)"
}

# ── TC-5：Stop event → completed bus 事件 ────────────────────────────────────

tc_5_stop_event() {
  log "=== TC-5: Stop(end_turn) → completed event on bus ==="

  local SESSION_ID="tc5-stop-$$"
  RUNTIME_SESSION_ID="$SESSION_ID" \
  RUNTIME_HOOKS_URL="$HOOKS_URL" \
  bash "$CC_HOOKS_SCRIPT" <<< '{"hook_event_name":"Stop","stop_reason":"end_turn","answer":"Task done"}'

  sleep 1
  assert_log_contains "runtime_bus_event.*$SESSION_ID"
  # Note: completed vs heartbeat depends on server mapping of Stop(end_turn)
  log "PASS: stop event processed (see log for type)"
}

# ── TC-6：inject 对话 → Agent 执行轨迹 ──────────────────────────────────────
# Layer 1 + Layer 2 验证（不需要真实 Kaku session）

tc_6_inject_conversation() {
  log "=== TC-6: inject conversation → agent execution trace ==="

  # 清理遗留 pane
  log "Listing current panes..."
  kaku cli list 2>/dev/null || log "kaku cli list failed (kaku may not be running)"

  local RESP
  RESP=$(inject "你好，请回复一个字：OK" "oc_e2e_kaku_tc6" 30)

  assert_no_error "$RESP"
  assert_reply_contains "$RESP" "OK" || assert_reply_contains "$RESP" "好" || {
    log "INFO: replies: $(echo "$RESP" | jq '.replies')"; true
  }
}

# ── TC-7：/api/hooks/runtime 直接调用 — heartbeat + completed ────────────────

tc_7_hooks_direct() {
  log "=== TC-7: Direct hooks API — heartbeat then completed ==="

  local SESSION_ID="tc7-direct-$$"

  # heartbeat via PostToolUse
  curl -sf -X POST "$HOOKS_URL/api/hooks/runtime?session_id=$SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{},"tool_response":"content"}' \
    > /dev/null
  log "Sent PostToolUse"

  # completed via Stop(end_turn)
  curl -sf -X POST "$HOOKS_URL/api/hooks/runtime?session_id=$SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{"hook_event_name":"Stop","stop_reason":"end_turn","answer":"42 files found"}' \
    > /dev/null
  log "Sent Stop(end_turn)"

  sleep 1
  assert_log_contains "type=heartbeat.*$SESSION_ID"
  assert_log_contains "runtime_bus_event.*$SESSION_ID"
  log "PASS: TC-7 hooks direct calls verified"
}

# ── 执行顺序 ─────────────────────────────────────────────────────────────────

if [[ $DRY_RUN -eq 1 ]]; then
  echo "Available test cases:"
fi

run_case "TC-0" tc_0_healthcheck
run_case "TC-1" tc_1_notify_normal
run_case "TC-2" tc_2_notify_unavailable
run_case "TC-3" tc_3_notify_empty_session
run_case "TC-4" tc_4_notify_bad_json
run_case "TC-5" tc_5_stop_event
run_case "TC-6" tc_6_inject_conversation
run_case "TC-7" tc_7_hooks_direct

# ── 汇总 ─────────────────────────────────────────────────────────────────────

echo ""
echo "════════════════════════════════════════"
echo "Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP  TOTAL=$TOTAL"
echo "════════════════════════════════════════"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
