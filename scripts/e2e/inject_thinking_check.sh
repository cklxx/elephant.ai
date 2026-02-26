#!/usr/bin/env bash
set -euo pipefail

INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
CHAT_ID="${CHAT_ID:-oc_e2e_thinking_check}"
SENDER_ID="${SENDER_ID:-ou_e2e_thinking_check}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-180}"
TOOL_MESSAGE_ROUNDS="${TOOL_MESSAGE_ROUNDS:-5}"
TEXT="${*:-请先思考再回答，最终只输出 OK。}"

MARKER="E2E_THINKING_CHECK_$(date +%s)"
REQ_FILE="/tmp/inject_thinking_req_${MARKER}.json"
RESP_FILE="/tmp/inject_thinking_resp_${MARKER}.json"
HDR_FILE="/tmp/inject_thinking_hdr_${MARKER}.txt"

jq -n \
  --arg text "${MARKER} ${TEXT}" \
  --arg chat_id "${CHAT_ID}" \
  --arg sender_id "${SENDER_ID}" \
  --argjson timeout_seconds "${TIMEOUT_SECONDS}" \
  --argjson tool_message_rounds "${TOOL_MESSAGE_ROUNDS}" \
  '{
    text: $text,
    chat_id: $chat_id,
    chat_type: "p2p",
    sender_id: $sender_id,
    tool_message_rounds: $tool_message_rounds,
    timeout_seconds: $timeout_seconds,
    auto_reply: false,
    max_auto_reply_rounds: 3
  }' > "${REQ_FILE}"

curl -sS -D "${HDR_FILE}" -o "${RESP_FILE}" \
  -X POST "${INJECT_URL}" \
  -H "Content-Type: application/json" \
  --data @"${REQ_FILE}"

HTTP_STATUS="$(head -n 1 "${HDR_FILE}" | tr -d '\r')"
HTTP_CODE="$(echo "${HTTP_STATUS}" | awk '{print $2}')"

if [[ "${HTTP_CODE}" != "200" ]]; then
  echo "MARKER=${MARKER}"
  echo "HTTP_STATUS=${HTTP_STATUS}"
  echo "REQUEST=${REQ_FILE}"
  echo "RESPONSE=${RESP_FILE}"
  jq -c '.' "${RESP_FILE}" || cat "${RESP_FILE}"
  exit 1
fi

echo "MARKER=${MARKER}"
echo "HTTP_STATUS=${HTTP_STATUS}"
echo "REQUEST_KEYS=$(jq -r 'keys | join(",")' "${REQ_FILE}")"
echo "RESPONSE_KEYS=$(jq -r 'keys | join(",")' "${RESP_FILE}")"
echo "RESPONSE_SUMMARY=$(jq -c '{duration_ms,replies_count:(.replies|length),first_reply_method:(.replies[0].method // ""),first_reply_msg_type:(.replies[0].msg_type // ""),first_reply_content:(.replies[0].content // "")}' "${RESP_FILE}")"

MATCH_LINE=""
for _ in $(seq 1 20); do
  MATCH_LINE="$(rg -n --fixed-strings "${MARKER}" logs/requests/llm.jsonl | tail -n 1 || true)"
  if [[ -n "${MATCH_LINE}" ]]; then
    break
  fi
  sleep 0.5
done
if [[ -z "${MATCH_LINE}" ]]; then
  echo "LOG_ID=<not-found>"
  echo "No request log line matched marker. Check logs/requests/llm.jsonl availability."
  exit 2
fi

LOG_JSON="${MATCH_LINE#*:}"
LOG_ID="$(echo "${LOG_JSON}" | jq -r '.log_id')"
echo "LOG_ID=${LOG_ID}"

echo "LLM_ENTRIES:"
rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | \
  jq -c '{timestamp,entry_type,request_id,reasoning:(.payload.reasoning // null),has_thinking:((.payload.thinking // null)!=null),stop_reason:(.payload.stop_reason // null),content_preview:((.payload.content // "")[:120])}'

REQ_WITH_REASONING="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="request" and .payload.reasoning!=null) | .request_id' | wc -l | tr -d ' ')"
RESP_WITH_THINKING="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="response" and .payload.thinking!=null) | .request_id' | wc -l | tr -d ' ')"
MAIN_REQUEST_ID="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="request" and .payload.reasoning!=null) | .request_id' | head -n 1)"
THINKING_TEXT="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="response" and .payload.thinking!=null) | [.payload.thinking.parts[]?.text] | join("")' | head -n 1)"
SEND_MESSAGE_TOOL_CALLS="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="response" and .payload.tool_calls!=null) | (.payload.tool_calls[]?.name // empty) | select(.=="channel" or .=="lark_send_message")' | wc -l | tr -d ' ')"
UPDATE_CONFIG_TOOL_CALLS="$(rg --fixed-strings "${LOG_ID}" logs/requests/llm.jsonl | jq -r 'select(.entry_type=="response" and .payload.tool_calls!=null) | (.payload.tool_calls[]?.name // empty) | select(.=="update_config")' | wc -l | tr -d ' ')"

API_RESPONSE_FIELDS_OK=0
if [[ -n "${MAIN_REQUEST_ID}" ]]; then
  if rg --fixed-strings "${MAIN_REQUEST_ID}" logs/requests/llm.jsonl \
    | jq -e 'select(.entry_type=="response") | (.payload|has("content")) and (.payload|has("usage")) and (.payload|has("stop_reason"))' >/dev/null; then
    API_RESPONSE_FIELDS_OK=1
  fi
fi

LARK_HAS_THINKING=0
if [[ -n "${THINKING_TEXT}" ]]; then
  if jq -e --arg thinking "${THINKING_TEXT}" \
    '(.replies // [])
      | map(((.content | fromjson? | .text) // .content // ""))
      | any(contains($thinking))' "${RESP_FILE}" >/dev/null; then
    LARK_HAS_THINKING=1
  fi
fi

TEXT_REPLY_COUNT="$(jq -r '[.replies[]? | select(.msg_type=="text")] | length' "${RESP_FILE}")"
LARK_MULTI_MESSAGE=0
if [[ "${TEXT_REPLY_COUNT}" -gt 1 ]]; then
  LARK_MULTI_MESSAGE=1
fi
CHECK_TOOL_SEND_MESSAGE_CALLED=0
if [[ "${SEND_MESSAGE_TOOL_CALLS}" -gt 0 ]]; then
  CHECK_TOOL_SEND_MESSAGE_CALLED=1
fi
CHECK_UNEXPECTED_UPDATE_CONFIG=0
if [[ "${UPDATE_CONFIG_TOOL_CALLS}" -gt 0 ]]; then
  CHECK_UNEXPECTED_UPDATE_CONFIG=1
fi

echo "CHECK_REQUEST_HAS_REASONING=${REQ_WITH_REASONING}"
echo "CHECK_RESPONSE_HAS_THINKING=${RESP_WITH_THINKING}"
echo "CHECK_LARK_HAS_THINKING=${LARK_HAS_THINKING}"
echo "CHECK_LARK_MULTI_MESSAGE=${LARK_MULTI_MESSAGE}"
echo "CHECK_TOOL_SEND_MESSAGE_CALLED=${CHECK_TOOL_SEND_MESSAGE_CALLED}"
echo "CHECK_UNEXPECTED_UPDATE_CONFIG=${CHECK_UNEXPECTED_UPDATE_CONFIG}"
echo "CHECK_API_RESPONSE_FIELDS=${API_RESPONSE_FIELDS_OK}"
echo "MAIN_REQUEST_ID=${MAIN_REQUEST_ID:-<none>}"
echo "THINKING_TEXT_PREVIEW=$(printf '%s' "${THINKING_TEXT}" | cut -c1-120)"
echo "SEND_MESSAGE_TOOL_CALLS=${SEND_MESSAGE_TOOL_CALLS}"
echo "UPDATE_CONFIG_TOOL_CALLS=${UPDATE_CONFIG_TOOL_CALLS}"
echo "TEXT_REPLY_COUNT=${TEXT_REPLY_COUNT}"
echo "REQUEST_FILE=${REQ_FILE}"
echo "RESPONSE_FILE=${RESP_FILE}"
