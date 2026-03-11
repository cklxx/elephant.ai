#!/bin/zsh
# launch-cc.sh — 一步启动 Claude Code 到 Kaku pane
#
# 用法:
#   # 模式 A: split 新 pane
#   bash scripts/kaku/launch-cc.sh \
#     --parent-pane <id> \
#     --goal "你的任务" \
#     [--work-dir /path/to/dir]
#
#   # 模式 B: 复用已有 pane（不 split，适配 layout.sh 预创建的 pane）
#   bash scripts/kaku/launch-cc.sh \
#     --pane-id <id> \
#     --goal "你的任务"
#
# 输出: pane ID
set -euo pipefail

KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

PARENT_PANE=""
PANE_ID=""
GOAL=""
WORK_DIR="$(pwd)"
SESSION_ID=""
HOOKS_URL="${RUNTIME_HOOKS_URL:-http://localhost:9090}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --parent-pane)  PARENT_PANE="$2"; shift 2 ;;
    --pane-id)      PANE_ID="$2";     shift 2 ;;
    --goal)         GOAL="$2";        shift 2 ;;
    --work-dir)     WORK_DIR="$2";    shift 2 ;;
    --session-id)   SESSION_ID="$2";  shift 2 ;;
    --hooks-url)    HOOKS_URL="$2";   shift 2 ;;
    *) echo "ERROR: unknown flag $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$PARENT_PANE" && -z "$PANE_ID" ]]; then
  echo "ERROR: --parent-pane or --pane-id required" >&2; exit 1
fi
if [[ -z "$GOAL" ]]; then
  echo "ERROR: --goal required" >&2; exit 1
fi

# Step 1: 确保 notify_runtime.sh 已注册
bash "$SCRIPT_DIR/../cc_hooks/notify_runtime.sh" --ensure-registered >&2

# Step 2: 获取目标 pane（split 新的或复用已有的）
if [[ -n "$PANE_ID" ]]; then
  PANE="$PANE_ID"
else
  PANE=$("$KAKU" cli split-pane \
    --pane-id "$PARENT_PANE" \
    --bottom \
    --percent 65 \
    --cwd "$WORK_DIR" \
    -- zsh -l)
fi

# Step 3: 导出环境变量（给 notify_runtime.sh 用）
ENV_LINE="export RUNTIME_HOOKS_URL='${HOOKS_URL}'"
if [[ -n "$SESSION_ID" ]]; then
  ENV_LINE="${ENV_LINE} RUNTIME_SESSION_ID='${SESSION_ID}'"
fi
"$KAKU" cli send-text --pane-id "$PANE" "$ENV_LINE"
"$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'
sleep 0.5

# Step 4: 启动 CC（unset CLAUDECODE 防止嵌套 session 报错）
"$KAKU" cli send-text --pane-id "$PANE" "unset CLAUDECODE && claude --dangerously-skip-permissions"
"$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'

# Step 5: 等待 CC 欢迎界面（❯ 提示符）
sleep 3

# Step 6: 注入 goal + Enter
"$KAKU" cli send-text --pane-id "$PANE" "$GOAL"
sleep 0.3
"$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'

# 输出 pane ID 供调用方记录
echo "$PANE"
