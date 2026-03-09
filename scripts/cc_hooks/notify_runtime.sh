#!/bin/bash
# notify_runtime.sh — Claude Code hook that forwards tool/stop events to the
# Kaku runtime event bus. Mirror of notify_lark.sh for the runtime subsystem.
#
# Normal mode (called by CC as a hook):
#   CC invokes this on every PostToolUse / Stop event via settings.json.
#
# Self-registration mode (call before launching CC):
#   bash scripts/cc_hooks/notify_runtime.sh --ensure-registered
#   Checks ~/.claude/settings.json and adds itself if absent. Safe to call
#   repeatedly (idempotent). Use this in kaku-runtime skill before any
#   manual `kaku cli` launch to guarantee hooks are wired even when the
#   runtime API (ClaudeCodeAdapter) is not used.
#
# Required env vars (set by ClaudeCodeAdapter or manually before CC launch):
#   RUNTIME_SESSION_ID   — The Kaku runtime session ID.
#   RUNTIME_HOOKS_URL    — Base URL of the runtime hooks endpoint.
#
set -euo pipefail

SCRIPT_PATH="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
SETTINGS_FILE="${HOME}/.claude/settings.json"

# ── --ensure-registered mode ──────────────────────────────────────────────────
if [[ "${1:-}" == "--ensure-registered" ]]; then
  # Read existing settings (create empty object if absent).
  if [[ -f "$SETTINGS_FILE" ]]; then
    CONTENT=$(cat "$SETTINGS_FILE")
  else
    CONTENT="{}"
  fi

  # Check if this script is already registered under PostToolUse.
  if echo "$CONTENT" | jq -e --arg p "$SCRIPT_PATH" '
      .hooks.PostToolUse[]?.hooks[]?.command == $p' >/dev/null 2>&1; then
    echo "notify_runtime.sh: already registered in $SETTINGS_FILE"
    exit 0
  fi

  # Build the hook entry and merge it in.
  HOOK_ENTRY=$(jq -cn --arg cmd "$SCRIPT_PATH" '{
    hooks: [{ type: "command", command: $cmd, async: true }]
  }')

  UPDATED=$(echo "$CONTENT" | jq --argjson entry "$HOOK_ENTRY" '
    .hooks.PostToolUse += [$entry] |
    .hooks.Stop        += [$entry]
  ')

  # Atomic write.
  TMP=$(mktemp "${SETTINGS_FILE}.XXXXXX")
  echo "$UPDATED" > "$TMP"
  mv "$TMP" "$SETTINGS_FILE"
  echo "notify_runtime.sh: registered in $SETTINGS_FILE (PostToolUse + Stop)"
  exit 0
fi

# ── Normal hook mode ───────────────────────────────────────────────────────────
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
