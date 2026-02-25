#!/bin/bash
# notify_lark.sh — Claude Code hook script that forwards tool/stop events
# to elephant.ai's hooks bridge, which relays them to Lark as friendly
# Chinese status messages.
#
# Usage: Configure as a Claude Code hook in .claude/settings.json:
#
#   {
#     "hooks": {
#       "PostToolUse": [{
#         "hooks": [{ "type": "command", "command": "\"$CLAUDE_PROJECT_DIR\"/scripts/cc_hooks/notify_lark.sh", "async": true }]
#       }],
#       "Stop": [{
#         "hooks": [{ "type": "command", "command": "\"$CLAUDE_PROJECT_DIR\"/scripts/cc_hooks/notify_lark.sh", "async": true }]
#       }]
#     }
#   }
#
# Environment variables:
#   ELEPHANT_HOOKS_URL    Base URL of elephant.ai server (default: http://localhost:8080)
#   ELEPHANT_HOOKS_TOKEN  Bearer token for authentication (optional)
#   ELEPHANT_HOOKS_CHAT   Override target Lark chat ID (optional)
#
set -euo pipefail

# Read the hook event JSON from stdin.
INPUT=$(cat)

HOOKS_URL="${ELEPHANT_HOOKS_URL:-http://localhost:8080}"
HOOKS_TOKEN="${ELEPHANT_HOOKS_TOKEN:-}"

# Build a normalized payload for hooks bridge (always valid JSON object).
if ! PAYLOAD=$(echo "$INPUT" | jq -c '
  def text_or_empty:
    if . == null then ""
    elif type == "string" then .
    else (try tostring catch "")
    end;
  {
    event: ((.hook_event_name // .event // .event_name // "") | text_or_empty),
    session_id: ((.session_id // .session // .sessionId // "") | text_or_empty),
    tool_name: ((.tool_name // .tool // .name // "") | text_or_empty),
    tool_input: (.tool_input // .tool_args // .input // .arguments // {}),
    output: ((.tool_response // .output // .result // "") | text_or_empty),
    error: ((.error // .err // "") | text_or_empty),
    stop_reason: ((.stop_reason // .reason // .stop // "") | text_or_empty),
    answer: ((.answer // .final_answer // .finalAnswer // .output // "") | text_or_empty)
  }' 2>/dev/null); then
  # As a last resort, forward a minimal payload instead of posting malformed JSON.
  PAYLOAD=$(jq -cn --arg raw "$INPUT" '{
    event: "",
    session_id: "",
    tool_name: "",
    tool_input: {},
    output: $raw,
    error: "",
    stop_reason: "",
    answer: ""
  }')
fi

# Build URL with optional chat_id override.
URL="${HOOKS_URL}/api/hooks/claude-code"
if [ -n "${ELEPHANT_HOOKS_CHAT:-}" ]; then
  URL="${URL}?chat_id=${ELEPHANT_HOOKS_CHAT}"
fi

# Build auth header.
AUTH_HEADER=""
if [ -n "$HOOKS_TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${HOOKS_TOKEN}"
fi

# POST to the hooks bridge (fire-and-forget, don't block Claude).
if [ -n "$AUTH_HEADER" ]; then
  curl -s -o /dev/null -w '' \
    --connect-timeout 3 \
    --max-time 5 \
    -X POST "$URL" \
    -H "Content-Type: application/json" \
    -H "$AUTH_HEADER" \
    -d "$PAYLOAD" || true
else
  curl -s -o /dev/null -w '' \
    --connect-timeout 3 \
    --max-time 5 \
    -X POST "$URL" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" || true
fi
