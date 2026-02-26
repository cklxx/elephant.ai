#!/usr/bin/env bash
#
# E2E inject test: verify LLM can operate all Lark document tools
# (docx, wiki, drive, bitable, sheets) through the channel tool.
#
# Usage:
#   bash scripts/e2e/lark_doc_tools_e2e.sh                          # all domains
#   bash scripts/e2e/lark_doc_tools_e2e.sh --domain docx            # single domain
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

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────

INJECT_URL="${INJECT_URL:-http://localhost:9090/api/dev/inject}"
CHAT_ID_PREFIX="${E2E_CHAT_ID_PREFIX:-e2e-doc}"
SENDER_ID="${E2E_SENDER_ID:-e2e-tester}"
TIMEOUT="${E2E_TIMEOUT:-180}"
BITABLE_APP_TOKEN="${E2E_BITABLE_APP_TOKEN:-}"
AUTO_REPLY="${E2E_AUTO_REPLY:-true}"
MAX_AUTO_REPLY="${E2E_MAX_AUTO_REPLY:-3}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"

# Parse --domain flag
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
    printf "${RED}[FAIL]${NC} %-35s — HTTP %s: %s ${YELLOW}(%ss)${NC}\n" "$label" "$http_code" "$err_msg" "$elapsed_s"
    RESULTS+=("FAIL|$label|HTTP $http_code: $err_msg|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi

  # Extract all reply content into a single string
  local all_content
  all_content=$(echo "$body" | jq -r '[.replies[]?.content // ""] | join(" ")' 2>/dev/null || echo "")

  if [[ -z "$all_content" ]]; then
    printf "${RED}[FAIL]${NC} %-35s — empty reply ${YELLOW}(%ss)${NC}\n" "$label" "$elapsed_s"
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
    printf "${GREEN}[PASS]${NC} %-35s — \"%s\" found in reply ${YELLOW}(%ss)${NC}\n" "$label" "$found" "$elapsed_s"
    RESULTS+=("PASS|$label|\"$found\" matched|${elapsed_s}s")
    PASSED=$((PASSED + 1))
    return 0
  else
    # Truncate reply for display
    local truncated
    truncated=$(echo "$all_content" | head -c 200)
    printf "${RED}[FAIL]${NC} %-35s — none of [%s] found ${YELLOW}(%ss)${NC}\n" "$label" "${keywords[*]}" "$elapsed_s"
    printf "       Reply: %.200s...\n" "$truncated"
    RESULTS+=("FAIL|$label|no keyword matched|${elapsed_s}s")
    FAILED=$((FAILED + 1))
    return 1
  fi
}

should_run() {
  local domain="$1"
  [[ -z "$DOMAIN_FILTER" || "$DOMAIN_FILTER" == "$domain" ]]
}

# ── Banner ────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}${CYAN}=== Lark Document Tools E2E Test ===${NC}"
echo -e "Inject URL:  ${INJECT_URL}"
echo -e "Timestamp:   ${TIMESTAMP}"
echo -e "Timeout:     ${TIMEOUT}s per request"
echo -e "Auto-reply:  ${AUTO_REPLY} (max ${MAX_AUTO_REPLY} rounds)"
[[ -n "$DOMAIN_FILTER" ]] && echo -e "Domain:      ${DOMAIN_FILTER}"
echo ""

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
    echo -e "${YELLOW}[SKIP] bitable — E2E_BITABLE_APP_TOKEN not set${NC}"
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

# ── 6. Cleanup ────────────────────────────────────────────────────

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

echo ""

# ── Summary ───────────────────────────────────────────────────────

echo -e "${BOLD}${CYAN}=== Summary ===${NC}"
echo ""

printf "%-8s %-35s %-35s %s\n" "Status" "Test" "Result" "Time"
printf "%-8s %-35s %-35s %s\n" "------" "----" "------" "----"
for r in "${RESULTS[@]+"${RESULTS[@]}"}"; do
  IFS='|' read -r status label detail elapsed <<< "$r"
  if [[ "$status" == "PASS" ]]; then
    printf "${GREEN}%-8s${NC} %-35s %-35s %s\n" "$status" "$label" "$detail" "$elapsed"
  else
    printf "${RED}%-8s${NC} %-35s %-35s %s\n" "$status" "$label" "$detail" "$elapsed"
  fi
done

echo ""
echo -e "${BOLD}Total: ${TOTAL}  |  ${GREEN}Passed: ${PASSED}${NC}  |  ${RED}Failed: ${FAILED}${NC}"
echo ""

if [[ "$FAILED" -gt 0 ]]; then
  exit 1
fi
