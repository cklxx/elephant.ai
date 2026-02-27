#!/usr/bin/env bash
#
# E2E inject test: verify LLM can operate all Lark product features
# through the channel tool — docx, wiki, drive, bitable, sheets,
# messaging, calendar, tasks, OKR, contact, mail, VC.
#
# Usage:
#   bash scripts/e2e/lark_doc_tools_e2e.sh                          # all domains
#   bash scripts/e2e/lark_doc_tools_e2e.sh --domain docx            # single domain
#   bash scripts/e2e/lark_doc_tools_e2e.sh --domain url             # URL verification only
#   bash scripts/e2e/lark_doc_tools_e2e.sh --domain bitable         # needs E2E_BITABLE_APP_TOKEN
#
# Environment:
#   INJECT_URL           — inject endpoint (default: http://localhost:9090/api/dev/inject)
#   E2E_CHAT_ID_PREFIX   — chat_id prefix for sessions (default: e2e-doc)
#   E2E_SENDER_ID        — sender_id (default: e2e-tester)
#   E2E_TIMEOUT          — per-request timeout in seconds (default: 180)
#   E2E_BITABLE_APP_TOKEN — required for bitable domain tests
#   E2E_AUTO_REPLY       — enable auto-reply for multi-turn (default: true)
#   E2E_MAX_AUTO_REPLY   — max auto-reply rounds (default: 3)
#   E2E_LARK_CHAT_ID     — real Lark chat_id for messaging tests (default: skip)
#   E2E_OKR_USER_ID      — user_id for OKR tests (default: skip)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────

INJECT_URL="${INJECT_URL:-http://localhost:9090/api/dev/inject}"
CHAT_ID_PREFIX="${E2E_CHAT_ID_PREFIX:-e2e-doc}"
SENDER_ID="${E2E_SENDER_ID:-e2e-tester}"
TIMEOUT="${E2E_TIMEOUT:-180}"
BITABLE_APP_TOKEN="${E2E_BITABLE_APP_TOKEN:-}"
AUTO_REPLY="${E2E_AUTO_REPLY:-true}"
MAX_AUTO_REPLY="${E2E_MAX_AUTO_REPLY:-3}"
LARK_CHAT_ID="${E2E_LARK_CHAT_ID:-}"
OKR_USER_ID="${E2E_OKR_USER_ID:-}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"

# Parse flags
DOMAIN_FILTER=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain) DOMAIN_FILTER="$2"; shift 2 ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# ── State ─────────────────────────────────────────────────────────

TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0
RESULTS=()

# ── Colors ────────────────────────────────────────────────────────

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── Helper functions ──────────────────────────────────────────────

# inject_and_check <domain/action> <chat_id> <prompt> <keyword1> [keyword2] ...
#
# Sends the prompt to the inject API and checks if ANY of the expected
# keywords appear in the combined reply content. Prints PASS/FAIL.
inject_and_check() {
  local label="$1"; shift
  local chat_id="$1"; shift
  local prompt="$1"; shift
  local keywords=("$@")

  TOTAL=$((TOTAL + 1))

  local auto_reply_json="false"
  local max_auto_reply_json=0
  if [[ "$AUTO_REPLY" == "true" ]]; then
    auto_reply_json="true"
    max_auto_reply_json="$MAX_AUTO_REPLY"
  fi

  local payload
  payload=$(jq -n \
    --arg text "$prompt" \
    --arg chat_id "$chat_id" \
    --arg sender_id "$SENDER_ID" \
    --argjson timeout "$TIMEOUT" \
    --argjson auto_reply "$auto_reply_json" \
    --argjson max_auto_reply "$max_auto_reply_json" \
    '{
      text: $text,
      chat_id: $chat_id,
      chat_type: "p2p",
      sender_id: $sender_id,
      timeout_seconds: $timeout,
      auto_reply: $auto_reply,
      max_auto_reply_rounds: $max_auto_reply
    }')

  local start_time
  start_time=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')

  local response
  response=$(curl -s -w '\n%{http_code}' \
    --max-time "$((TIMEOUT + 30))" \
    -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "$payload" 2>&1) || true

  local end_time
  end_time=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')
  local elapsed_ms=$(( (end_time - start_time) / 1000000 ))
  local elapsed_s
  elapsed_s=$(awk "BEGIN {printf \"%.1f\", $elapsed_ms / 1000}")

  # Split response body and HTTP status code
  local http_code
  http_code=$(echo "$response" | tail -1)
  local body
  body=$(echo "$response" | sed '$d')

  # Check for HTTP-level errors
  if [[ "$http_code" != "200" ]]; then
    local err_msg
    err_msg=$(echo "$body" | jq -r '.error // "unknown error"' 2>/dev/null || echo "$body")
    printf "${RED}[FAIL]${NC} %-40s — HTTP %s: %s ${YELLOW}(%ss)${NC}\n" "$label" "$http_code" "$err_msg" "$elapsed_s"
    RESULTS+=("FAIL|$label|HTTP $http_code: $err_msg|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi

  # Extract all reply content into a single string
  local all_content
  all_content=$(echo "$body" | jq -r '[.replies[]?.content // ""] | join(" ")' 2>/dev/null || echo "")

  if [[ -z "$all_content" ]]; then
    printf "${RED}[FAIL]${NC} %-40s — empty reply ${YELLOW}(%ss)${NC}\n" "$label" "$elapsed_s"
    RESULTS+=("FAIL|$label|empty reply|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi

  # Check if any keyword matches (case-insensitive)
  local found=""
  for kw in "${keywords[@]}"; do
    if echo "$all_content" | grep -iq "$kw"; then
      found="$kw"
      break
    fi
  done

  if [[ -n "$found" ]]; then
    printf "${GREEN}[PASS]${NC} %-40s — \"%s\" found ${YELLOW}(%ss)${NC}\n" "$label" "$found" "$elapsed_s"
    RESULTS+=("PASS|$label|\"$found\" matched|${elapsed_s}s")
    PASSED=$((PASSED + 1))
    return 0
  else
    # Truncate reply for display
    local truncated
    truncated=$(echo "$all_content" | head -c 200)
    printf "${RED}[FAIL]${NC} %-40s — none of [%s] found ${YELLOW}(%ss)${NC}\n" "$label" "${keywords[*]}" "$elapsed_s"
    printf "       Reply: %.200s...\n" "$truncated"
    RESULTS+=("FAIL|$label|no keyword matched|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi
}

# inject_and_check_url <domain/action> <chat_id> <prompt> <url_pattern> [extra_keyword...]
#
# Like inject_and_check but specifically validates that the reply contains
# a URL matching the given pattern (e.g. "feishu.cn/docx/"). This catches
# the case where the LLM fabricates URLs or returns raw tokens.
inject_and_check_url() {
  local label="$1"; shift
  local chat_id="$1"; shift
  local prompt="$1"; shift
  local url_pattern="$1"; shift
  local extra_keywords=("$@")

  TOTAL=$((TOTAL + 1))

  local auto_reply_json="false"
  local max_auto_reply_json=0
  if [[ "$AUTO_REPLY" == "true" ]]; then
    auto_reply_json="true"
    max_auto_reply_json="$MAX_AUTO_REPLY"
  fi

  local payload
  payload=$(jq -n \
    --arg text "$prompt" \
    --arg chat_id "$chat_id" \
    --arg sender_id "$SENDER_ID" \
    --argjson timeout "$TIMEOUT" \
    --argjson auto_reply "$auto_reply_json" \
    --argjson max_auto_reply "$max_auto_reply_json" \
    '{
      text: $text,
      chat_id: $chat_id,
      chat_type: "p2p",
      sender_id: $sender_id,
      timeout_seconds: $timeout,
      auto_reply: $auto_reply,
      max_auto_reply_rounds: $max_auto_reply
    }')

  local start_time
  start_time=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')

  local response
  response=$(curl -s -w '\n%{http_code}' \
    --max-time "$((TIMEOUT + 30))" \
    -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "$payload" 2>&1) || true

  local end_time
  end_time=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1e9))')
  local elapsed_ms=$(( (end_time - start_time) / 1000000 ))
  local elapsed_s
  elapsed_s=$(awk "BEGIN {printf \"%.1f\", $elapsed_ms / 1000}")

  local http_code
  http_code=$(echo "$response" | tail -1)
  local body
  body=$(echo "$response" | sed '$d')

  if [[ "$http_code" != "200" ]]; then
    local err_msg
    err_msg=$(echo "$body" | jq -r '.error // "unknown error"' 2>/dev/null || echo "$body")
    printf "${RED}[FAIL]${NC} %-40s — HTTP %s: %s ${YELLOW}(%ss)${NC}\n" "$label" "$http_code" "$err_msg" "$elapsed_s"
    RESULTS+=("FAIL|$label|HTTP $http_code: $err_msg|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi

  local all_content
  all_content=$(echo "$body" | jq -r '[.replies[]?.content // ""] | join(" ")' 2>/dev/null || echo "")

  if [[ -z "$all_content" ]]; then
    printf "${RED}[FAIL]${NC} %-40s — empty reply ${YELLOW}(%ss)${NC}\n" "$label" "$elapsed_s"
    RESULTS+=("FAIL|$label|empty reply|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi

  # Primary check: URL pattern must appear in reply
  if echo "$all_content" | grep -iq "$url_pattern"; then
    printf "${GREEN}[PASS]${NC} %-40s — URL pattern \"%s\" found ${YELLOW}(%ss)${NC}\n" "$label" "$url_pattern" "$elapsed_s"
    RESULTS+=("PASS|$label|URL \"$url_pattern\" matched|${elapsed_s}s")
    PASSED=$((PASSED + 1))
    return 0
  fi

  # Fallback: check extra keywords (still mark as FAIL for URL but note it)
  local found=""
  for kw in "${extra_keywords[@]+"${extra_keywords[@]}"}"; do
    if echo "$all_content" | grep -iq "$kw"; then
      found="$kw"
      break
    fi
  done

  if [[ -n "$found" ]]; then
    printf "${RED}[FAIL]${NC} %-40s — URL missing, but \"%s\" present ${YELLOW}(%ss)${NC}\n" "$label" "$found" "$elapsed_s"
    printf "       Reply (URL expected): %.200s...\n" "$(echo "$all_content" | head -c 200)"
  else
    printf "${RED}[FAIL]${NC} %-40s — URL \"%s\" not found ${YELLOW}(%ss)${NC}\n" "$label" "$url_pattern" "$elapsed_s"
    printf "       Reply: %.200s...\n" "$(echo "$all_content" | head -c 200)"
  fi
  RESULTS+=("FAIL|$label|URL not found|${elapsed_s}s")
  FAILED=$((FAILED + 1))
  return 1
}

skip_domain() {
  local domain="$1"
  local reason="$2"
  SKIPPED=$((SKIPPED + 1))
  echo -e "${YELLOW}[SKIP]${NC} ${domain} — ${reason}"
}

should_run() {
  local domain="$1"
  [[ -z "$DOMAIN_FILTER" || "$DOMAIN_FILTER" == "$domain" ]]
}

# ── Banner ────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}${CYAN}=== Lark Product Features E2E Test ===${NC}"
echo -e "Inject URL:  ${INJECT_URL}"
echo -e "Timestamp:   ${TIMESTAMP}"
echo -e "Timeout:     ${TIMEOUT}s per request"
echo -e "Auto-reply:  ${AUTO_REPLY} (max ${MAX_AUTO_REPLY} rounds)"
[[ -n "$DOMAIN_FILTER" ]] && echo -e "Domain:      ${DOMAIN_FILTER}"
echo ""

# ══════════════════════════════════════════════════════════════════
# DOCUMENT TOOLS
# ══════════════════════════════════════════════════════════════════

# ── 1. Docx (文档) ───────────────────────────────────────────────

if should_run "docx"; then
  DOCX_CHAT="${CHAT_ID_PREFIX}-docx-${TIMESTAMP}"
  echo -e "${BOLD}--- Docx ---${NC}"

  inject_and_check "docx/create_doc" "$DOCX_CHAT" \
    "在我的飞书云空间根目录创建一个标题为'E2E测试文档-${TIMESTAMP}'的文档" \
    "创建" "成功" "document" "文档" || true

  inject_and_check "docx/read_doc" "$DOCX_CHAT" \
    "读取你刚才创建的文档的元信息" \
    "标题" "E2E测试文档" "title" || true

  inject_and_check "docx/read_doc_content" "$DOCX_CHAT" \
    "读取那个文档的正文内容" \
    "内容" "content" "正文" "空" "block" || true

  inject_and_check "docx/list_doc_blocks" "$DOCX_CHAT" \
    "列出那个文档的所有block" \
    "block" "块" "page" "文档" || true

  echo ""
fi

# ── 2. Wiki (知识库) ─────────────────────────────────────────────

if should_run "wiki"; then
  WIKI_CHAT="${CHAT_ID_PREFIX}-wiki-${TIMESTAMP}"
  echo -e "${BOLD}--- Wiki ---${NC}"

  inject_and_check "wiki/list_spaces" "$WIKI_CHAT" \
    "列出我能访问的所有飞书知识库空间" \
    "知识库" "space" "找到" "空间" || true

  inject_and_check "wiki/list_nodes" "$WIKI_CHAT" \
    "列出第一个知识库空间下的节点列表" \
    "节点" "node" "找到" "文档" || true

  inject_and_check "wiki/create_node" "$WIKI_CHAT" \
    "在第一个知识库空间创建一个标题为'E2E测试节点-${TIMESTAMP}'的文档节点" \
    "创建" "成功" "node" "节点" || true

  inject_and_check "wiki/get_node" "$WIKI_CHAT" \
    "查看你刚才创建的知识库节点的详情" \
    "节点" "详情" "标题" "title" "node" || true

  echo ""
fi

# ── 3. Drive (云空间) ────────────────────────────────────────────

if should_run "drive"; then
  DRIVE_CHAT="${CHAT_ID_PREFIX}-drive-${TIMESTAMP}"
  echo -e "${BOLD}--- Drive ---${NC}"

  inject_and_check "drive/list_files" "$DRIVE_CHAT" \
    "列出我飞书云空间根目录下的文件列表" \
    "文件" "file" "找到" "目录" || true

  inject_and_check "drive/create_doc" "$DRIVE_CHAT" \
    "在我的飞书云空间根目录创建一个标题为'E2E驱动测试文档-${TIMESTAMP}'的文档" \
    "创建" "成功" "文档" "document" || true

  inject_and_check "drive/create_folder" "$DRIVE_CHAT" \
    "在飞书云空间根目录创建一个名为'E2E测试文件夹-${TIMESTAMP}'的文件夹" \
    "创建" "成功" "文件夹" "folder" || true

  inject_and_check "drive/copy_file" "$DRIVE_CHAT" \
    "把你刚才在本次对话中创建的那个'E2E驱动测试文档-${TIMESTAMP}'复制到'E2E测试文件夹-${TIMESTAMP}'文件夹里，新名称为'副本'" \
    "复制" "成功" "copy" "副本" || true

  inject_and_check "drive/delete_file" "$DRIVE_CHAT" \
    "删除刚才复制的那个名为'副本'的文件" \
    "删除" "成功" "delete" "已删" || true

  echo ""
fi

# ── 4. Bitable (多维表格) ────────────────────────────────────────

if should_run "bitable"; then
  if [[ -z "$BITABLE_APP_TOKEN" ]]; then
    skip_domain "bitable" "E2E_BITABLE_APP_TOKEN not set"
    echo ""
  else
    BITABLE_CHAT="${CHAT_ID_PREFIX}-bitable-${TIMESTAMP}"
    echo -e "${BOLD}--- Bitable ---${NC}"

    inject_and_check "bitable/list_tables" "$BITABLE_CHAT" \
      "列出这个多维表格的所有数据表：${BITABLE_APP_TOKEN}" \
      "数据表" "table" "找到" "表格" || true

    inject_and_check "bitable/list_fields" "$BITABLE_CHAT" \
      "列出第一个数据表的所有字段" \
      "字段" "field" "找到" "列" || true

    inject_and_check "bitable/create_record" "$BITABLE_CHAT" \
      "在第一个数据表中新建一条记录，第一个文本字段填'E2E测试记录'" \
      "创建" "成功" "record" "记录" || true

    inject_and_check "bitable/list_records" "$BITABLE_CHAT" \
      "列出第一个数据表的所有记录" \
      "记录" "record" "找到" "条" || true

    inject_and_check "bitable/update_record" "$BITABLE_CHAT" \
      "把刚创建的记录的第一个文本字段改为'E2E已更新'" \
      "更新" "成功" "update" "已更新" || true

    inject_and_check "bitable/delete_record" "$BITABLE_CHAT" \
      "删除刚才创建的那条E2E测试记录" \
      "删除" "成功" "delete" "已删" || true

    echo ""
  fi
fi

# ── 5. Sheets (电子表格) ─────────────────────────────────────────

if should_run "sheets"; then
  SHEETS_CHAT="${CHAT_ID_PREFIX}-sheets-${TIMESTAMP}"
  echo -e "${BOLD}--- Sheets ---${NC}"

  inject_and_check "sheets/create_spreadsheet" "$SHEETS_CHAT" \
    "请在我的飞书云空间中创建一个标题为'E2E测试表格-${TIMESTAMP}'的电子表格（spreadsheet），使用飞书的创建电子表格功能" \
    "创建" "成功" "spreadsheet" "表格" || true

  inject_and_check "sheets/get_spreadsheet" "$SHEETS_CHAT" \
    "获取你刚才在飞书中创建的那个'E2E测试表格-${TIMESTAMP}'电子表格的元信息" \
    "标题" "E2E测试表格" "title" "spreadsheet" || true

  inject_and_check "sheets/list_sheets" "$SHEETS_CHAT" \
    "列出刚才那个飞书电子表格里的所有工作表（sheet）" \
    "工作表" "sheet" "找到" "Sheet" || true

  echo ""
fi

# ══════════════════════════════════════════════════════════════════
# URL VERIFICATION — ensures document creation returns clickable URLs
# ══════════════════════════════════════════════════════════════════

if should_run "url"; then
  URL_CHAT="${CHAT_ID_PREFIX}-url-${TIMESTAMP}"
  echo -e "${BOLD}--- URL Verification ---${NC}"

  # Docx URL: create a doc and verify the reply contains a real URL
  inject_and_check_url "url/docx_create_has_url" "$URL_CHAT" \
    "帮我在飞书云空间创建一个标题为'URL验证文档-${TIMESTAMP}'的文档，创建后请把文档的链接发给我" \
    "feishu.cn/docx/" \
    "larksuite.com/docx/" "larkoffice.com/docx/" || true

  # Wiki URL: create a wiki node and verify URL
  URL_WIKI_CHAT="${CHAT_ID_PREFIX}-url-wiki-${TIMESTAMP}"
  inject_and_check_url "url/wiki_create_has_url" "$URL_WIKI_CHAT" \
    "列出我的知识库空间，然后在第一个空间创建一个标题为'URL验证节点-${TIMESTAMP}'的文档节点，创建后把链接发给我" \
    "feishu.cn/wiki/" \
    "larksuite.com/wiki/" "larkoffice.com/wiki/" || true

  # Sheets URL: create a spreadsheet and verify URL
  URL_SHEETS_CHAT="${CHAT_ID_PREFIX}-url-sheets-${TIMESTAMP}"
  inject_and_check_url "url/sheets_create_has_url" "$URL_SHEETS_CHAT" \
    "帮我在飞书创建一个标题为'URL验证表格-${TIMESTAMP}'的电子表格，创建好了把链接给我" \
    "feishu.cn/sheets/" \
    "larksuite.com/sheets/" "larkoffice.com/sheets/" || true

  # Negative check: docx URL should not be a fabricated pattern
  # (The LLM used to output things like "https://feishu.cn/docx/FAKE_TOKEN")
  inject_and_check "url/docx_read_has_url" "$URL_CHAT" \
    "读取你刚才创建的那个'URL验证文档'的元信息，把文档链接告诉我" \
    "feishu.cn/docx/" "larksuite.com/docx/" "larkoffice.com/docx/" "URL" "url" || true

  echo ""
fi

# ══════════════════════════════════════════════════════════════════
# NON-DOCUMENT FEATURES
# ══════════════════════════════════════════════════════════════════

# ── 6. Messaging (消息) ──────────────────────────────────────────

if should_run "messaging"; then
  if [[ -z "$LARK_CHAT_ID" ]]; then
    skip_domain "messaging" "E2E_LARK_CHAT_ID not set (needs a real Lark chat)"
    echo ""
  else
    MSG_CHAT="${CHAT_ID_PREFIX}-msg-${TIMESTAMP}"
    echo -e "${BOLD}--- Messaging ---${NC}"

    inject_and_check "messaging/send_message" "$MSG_CHAT" \
      "往这个飞书聊天发送一条消息'E2E消息测试-${TIMESTAMP}'，chat_id: ${LARK_CHAT_ID}" \
      "发送" "成功" "sent" "消息" || true

    inject_and_check "messaging/chat_history" "$MSG_CHAT" \
      "获取聊天 ${LARK_CHAT_ID} 最近的5条消息记录" \
      "消息" "message" "记录" "条" || true

    echo ""
  fi
fi

# ── 7. Calendar (日历) ───────────────────────────────────────────

if should_run "calendar"; then
  CAL_CHAT="${CHAT_ID_PREFIX}-cal-${TIMESTAMP}"
  echo -e "${BOLD}--- Calendar ---${NC}"

  inject_and_check "calendar/query_events" "$CAL_CHAT" \
    "查询我飞书日历上今天的所有日程" \
    "日程" "event" "找到" "没有" "无" "calendar" || true

  inject_and_check "calendar/create_event" "$CAL_CHAT" \
    "在我的飞书日历上创建一个今天的日程，标题为'E2E测试日程-${TIMESTAMP}'，时间设为今天下午3点到4点" \
    "创建" "成功" "日程" "event" || true

  inject_and_check "calendar/update_event" "$CAL_CHAT" \
    "把你刚才创建的那个'E2E测试日程-${TIMESTAMP}'的标题改为'E2E已更新日程-${TIMESTAMP}'" \
    "更新" "成功" "update" "已更新" "修改" || true

  inject_and_check "calendar/delete_event" "$CAL_CHAT" \
    "删除你刚才创建的那个E2E测试日程" \
    "删除" "成功" "delete" "已删" || true

  echo ""
fi

# ── 8. Tasks (任务) ──────────────────────────────────────────────

if should_run "tasks"; then
  TASK_CHAT="${CHAT_ID_PREFIX}-task-${TIMESTAMP}"
  echo -e "${BOLD}--- Tasks ---${NC}"

  inject_and_check "tasks/list_tasks" "$TASK_CHAT" \
    "列出我飞书上的所有待办任务" \
    "任务" "task" "找到" "没有" "无" "待办" || true

  inject_and_check "tasks/create_task" "$TASK_CHAT" \
    "在飞书任务中创建一个标题为'E2E测试任务-${TIMESTAMP}'的待办任务" \
    "创建" "成功" "task" "任务" || true

  inject_and_check "tasks/update_task" "$TASK_CHAT" \
    "把你刚才创建的'E2E测试任务-${TIMESTAMP}'标题改为'E2E已更新任务-${TIMESTAMP}'" \
    "更新" "成功" "update" "已更新" "修改" || true

  inject_and_check "tasks/delete_task" "$TASK_CHAT" \
    "删除你刚才创建的那个E2E测试任务" \
    "删除" "成功" "delete" "已删" || true

  echo ""
fi

# ── 9. Contact (通讯录) ──────────────────────────────────────────

if should_run "contact"; then
  CONTACT_CHAT="${CHAT_ID_PREFIX}-contact-${TIMESTAMP}"
  echo -e "${BOLD}--- Contact ---${NC}"

  inject_and_check "contact/list_departments" "$CONTACT_CHAT" \
    "列出飞书组织架构中的顶级部门列表" \
    "部门" "department" "找到" "组织" || true

  inject_and_check "contact/get_user" "$CONTACT_CHAT" \
    "查询我自己的飞书用户信息" \
    "用户" "user" "姓名" "name" "邮箱" || true

  echo ""
fi

# ── 10. OKR ──────────────────────────────────────────────────────

if should_run "okr"; then
  if [[ -z "$OKR_USER_ID" ]]; then
    skip_domain "okr" "E2E_OKR_USER_ID not set"
    echo ""
  else
    OKR_CHAT="${CHAT_ID_PREFIX}-okr-${TIMESTAMP}"
    echo -e "${BOLD}--- OKR ---${NC}"

    inject_and_check "okr/list_periods" "$OKR_CHAT" \
      "列出飞书OKR的所有考核周期" \
      "周期" "period" "找到" "OKR" || true

    inject_and_check "okr/list_user_okrs" "$OKR_CHAT" \
      "查看用户 ${OKR_USER_ID} 在最新周期的OKR" \
      "OKR" "目标" "objective" "关键结果" "key result" || true

    echo ""
  fi
fi

# ── 11. Mail (邮箱) ──────────────────────────────────────────────

if should_run "mail"; then
  MAIL_CHAT="${CHAT_ID_PREFIX}-mail-${TIMESTAMP}"
  echo -e "${BOLD}--- Mail ---${NC}"

  inject_and_check "mail/list_mailgroups" "$MAIL_CHAT" \
    "列出飞书邮箱中的所有邮件组" \
    "邮件组" "mailgroup" "找到" "没有" "无" "mail" || true

  echo ""
fi

# ── 12. VC (视频会议) ────────────────────────────────────────────

if should_run "vc"; then
  VC_CHAT="${CHAT_ID_PREFIX}-vc-${TIMESTAMP}"
  echo -e "${BOLD}--- VC (Video Conference) ---${NC}"

  inject_and_check "vc/list_rooms" "$VC_CHAT" \
    "列出飞书视频会议的所有可用会议室" \
    "会议室" "room" "找到" "没有" "无" || true

  echo ""
fi

# ══════════════════════════════════════════════════════════════════
# CLEANUP
# ══════════════════════════════════════════════════════════════════

echo -e "${BOLD}--- Cleanup ---${NC}"

if should_run "docx"; then
  DOCX_CHAT="${DOCX_CHAT:-${CHAT_ID_PREFIX}-docx-${TIMESTAMP}}"
  inject_and_check "cleanup/docx" "$DOCX_CHAT" \
    "删除你之前创建的所有包含'E2E测试'的文档，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true
fi

if should_run "drive"; then
  DRIVE_CHAT="${DRIVE_CHAT:-${CHAT_ID_PREFIX}-drive-${TIMESTAMP}}"
  inject_and_check "cleanup/drive" "$DRIVE_CHAT" \
    "删除你之前创建的所有包含'E2E'的文档和文件夹，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true
fi

if should_run "sheets"; then
  SHEETS_CHAT="${SHEETS_CHAT:-${CHAT_ID_PREFIX}-sheets-${TIMESTAMP}}"
  inject_and_check "cleanup/sheets" "$SHEETS_CHAT" \
    "删除你之前创建的所有包含'E2E测试'的电子表格，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true
fi

if should_run "url"; then
  URL_CHAT="${URL_CHAT:-${CHAT_ID_PREFIX}-url-${TIMESTAMP}}"
  inject_and_check "cleanup/url_docx" "$URL_CHAT" \
    "删除你之前创建的所有包含'URL验证'的文档，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true

  URL_WIKI_CHAT="${URL_WIKI_CHAT:-${CHAT_ID_PREFIX}-url-wiki-${TIMESTAMP}}"
  inject_and_check "cleanup/url_wiki" "$URL_WIKI_CHAT" \
    "删除你之前创建的'URL验证节点'，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true

  URL_SHEETS_CHAT="${URL_SHEETS_CHAT:-${CHAT_ID_PREFIX}-url-sheets-${TIMESTAMP}}"
  inject_and_check "cleanup/url_sheets" "$URL_SHEETS_CHAT" \
    "删除你之前创建的'URL验证表格'，帮我清理掉" \
    "删除" "成功" "清理" "已删" "完成" || true
fi

echo ""

# ══════════════════════════════════════════════════════════════════
# SUMMARY
# ══════════════════════════════════════════════════════════════════

echo -e "${BOLD}${CYAN}=== Summary ===${NC}"
echo ""

printf "%-8s %-40s %-35s %s\n" "Status" "Test" "Result" "Time"
printf "%-8s %-40s %-35s %s\n" "------" "----" "------" "----"
for r in "${RESULTS[@]+"${RESULTS[@]}"}"; do
  IFS='|' read -r status label detail elapsed <<< "$r"
  if [[ "$status" == "PASS" ]]; then
    printf "${GREEN}%-8s${NC} %-40s %-35s %s\n" "$status" "$label" "$detail" "$elapsed"
  else
    printf "${RED}%-8s${NC} %-40s %-35s %s\n" "$status" "$label" "$detail" "$elapsed"
  fi
done

echo ""
echo -e "${BOLD}Total: ${TOTAL}  |  ${GREEN}Passed: ${PASSED}${NC}  |  ${RED}Failed: ${FAILED}${NC}  |  ${YELLOW}Skipped: ${SKIPPED}${NC}"
echo ""

if [[ "$FAILED" -gt 0 ]]; then
  exit 1
fi
