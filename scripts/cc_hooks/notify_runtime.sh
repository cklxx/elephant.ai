#!/bin/bash
# notify_runtime.sh — Claude Code hook that forwards tool/stop events to the
# Kaku runtime event bus. Mirror of notify_lark.sh for the runtime subsystem.
#
# Configure in a CC session's settings.json (written by ClaudeCodeAdapter):
#
#   {
#     "hooks": {
#       "PostToolUse": [{ "hooks": [{ "type": "command",
#           "command": ".../notify_runtime.sh", "async": true }] }],
#       "Stop":        [{ "hooks": [{ "type": "command",
#           "command": ".../notify_runtime.sh", "async": true }] }]
#     }
#   }
#
# Required environment variables (set by ClaudeCodeAdapter before launching CC):
#   RUNTIME_SESSION_ID   — The Kaku runtime session ID.
#   RUNTIME_HOOKS_URL    — Base URL of the runtime hooks endpoint (e.g. http://localhost:8080).
#
set -euo pipefail

INPUT=$(cat)

SESSION_ID="${RUNTIME_SESSION_ID:-}"
HOOKS_URL="${RUNTIME_HOOKS_URL:-http://localhost:8080}"

if [ -z "$SESSION_ID" ]; then
  # Nothing to do without a session ID.
  exit 0
fi

# Normalise the CC hook payload into a flat JSON object accepted by
# RuntimeHooksHandler. We use the same field names as notify_lark.sh.
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
  # On jq failure forward a minimal payload.
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

URL="${HOOKS_URL}/api/hooks/runtime?session_id=${SESSION_ID}"

# Fire-and-forget: do not block Claude Code.
curl -s -o /dev/null -w '' \
  --connect-timeout 3 \
  --max-time 5 \
  -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" || true
