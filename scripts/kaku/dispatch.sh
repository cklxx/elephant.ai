#!/bin/zsh
# dispatch.sh — 批量分发任务到多个 agent pane
#
# 用法:
#   bash scripts/kaku/dispatch.sh \
#     [--layout 4grid|2h|2v|3col|1+2] \
#     [--new-window] \
#     [--cwd /path/to/dir] \
#     [--config /path/to/tasks.yaml]
#
#   或通过环境变量指定任务：
#   DISPATCH_TASKS='
#   - agent: cc
#     goal: "任务1"
#   - agent: codex
#     goal: "任务2"
#   ' bash scripts/kaku/dispatch.sh
#
# tasks.yaml 格式:
#   tasks:
#     - agent: cc        # cc | codex
#       goal: "任务描述"
#     - agent: codex
#       goal: "任务描述"
#
# 输出: 每行一个 "pane_id agent_type" 映射
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"

LAYOUT="4grid"
NEW_WINDOW=true
CWD="$(pwd)"
CONFIG=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --layout)      LAYOUT="$2";     shift 2 ;;
    --new-window)  NEW_WINDOW=true; shift ;;
    --cwd)         CWD="$2";        shift 2 ;;
    --config)      CONFIG="$2";     shift 2 ;;
    *) echo "ERROR: unknown flag $1" >&2; exit 1 ;;
  esac
done

# --- 创建布局 ---
LAYOUT_FLAG=""
if [[ "$NEW_WINDOW" == "true" ]]; then
  LAYOUT_FLAG="--new-window"
fi

LAYOUT_OUT=$(bash "$SCRIPT_DIR/layout.sh" "$LAYOUT" $LAYOUT_FLAG --cwd "$CWD")
echo "Layout created:" >&2
echo "$LAYOUT_OUT" >&2

# 提取 pane ID 列表（排除 TOP_LEFT，那是发起者所在 pane）
PANE_IDS=()
while IFS='=' read -r key val; do
  key=$(echo "$key" | xargs)
  val=$(echo "$val" | xargs)
  case "$key" in
    TOP_RIGHT|BOT_LEFT|BOT_RIGHT|RIGHT|BOTTOM|CENTER|LEFT)
      PANE_IDS+=("$val")
      ;;
  esac
done <<< "$LAYOUT_OUT"

echo "Available panes: ${PANE_IDS[*]}" >&2

# --- 解析任务 ---
# 简单的行协议解析（避免依赖 yq）
# 从 config 文件或 DISPATCH_TASKS 环境变量读取
AGENTS=()
GOALS=()

if [[ -n "$CONFIG" && -f "$CONFIG" ]]; then
  TASK_INPUT=$(cat "$CONFIG")
elif [[ -n "${DISPATCH_TASKS:-}" ]]; then
  TASK_INPUT="$DISPATCH_TASKS"
else
  echo "ERROR: --config or DISPATCH_TASKS required" >&2
  exit 1
fi

# 简易 YAML 解析
current_agent=""
current_goal=""
while IFS= read -r line; do
  line_trimmed=$(echo "$line" | sed 's/^[[:space:]]*//')
  case "$line_trimmed" in
    "- agent:"*)
      if [[ -n "$current_agent" && -n "$current_goal" ]]; then
        AGENTS+=("$current_agent")
        GOALS+=("$current_goal")
      fi
      current_agent=$(echo "$line_trimmed" | sed 's/- agent:[[:space:]]*//' | tr -d '"' | tr -d "'")
      current_goal=""
      ;;
    "goal:"*)
      current_goal=$(echo "$line_trimmed" | sed 's/goal:[[:space:]]*//' | tr -d '"')
      ;;
  esac
done <<< "$TASK_INPUT"
# 追加最后一个
if [[ -n "$current_agent" && -n "$current_goal" ]]; then
  AGENTS+=("$current_agent")
  GOALS+=("$current_goal")
fi

NUM_TASKS=${#AGENTS[@]}
NUM_PANES=${#PANE_IDS[@]}

if [[ $NUM_TASKS -gt $NUM_PANES ]]; then
  echo "WARNING: $NUM_TASKS tasks but only $NUM_PANES panes, extra tasks will be skipped" >&2
fi

# --- 分发 ---
DISPATCHED=0
for i in $(seq 0 $((NUM_TASKS - 1))); do
  if [[ $i -ge $NUM_PANES ]]; then
    echo "SKIP: task $i (no pane available)" >&2
    continue
  fi

  PANE="${PANE_IDS[$i]}"
  AGENT="${AGENTS[$i]}"
  GOAL="${GOALS[$i]}"

  echo "Dispatching task $i → pane $PANE ($AGENT)" >&2

  case "$AGENT" in
    cc|claude_code|claude)
      bash "$SCRIPT_DIR/launch-cc.sh" \
        --pane-id "$PANE" \
        --goal "$GOAL" \
        --work-dir "$CWD" >/dev/null 2>&1 &
      ;;
    codex)
      bash "$SCRIPT_DIR/launch-codex.sh" \
        --pane-id "$PANE" \
        --goal "$GOAL" \
        --work-dir "$CWD" >/dev/null 2>&1 &
      ;;
    *)
      echo "ERROR: unknown agent type '$AGENT'" >&2
      continue
      ;;
  esac

  echo "$PANE $AGENT"
  DISPATCHED=$((DISPATCHED + 1))
done

wait
echo "Dispatched $DISPATCHED/$NUM_TASKS tasks" >&2
