#!/bin/zsh
# launch-codex.sh — 一步启动 Codex 到 Kaku pane
#
# 用法:
#   # 模式 A: split 新 pane
#   bash scripts/kaku/launch-codex.sh \
#     --parent-pane <id> \
#     --goal "你的任务" \
#     [--work-dir /path/to/dir]
#
#   # 模式 B: 复用已有 pane（不 split，适配 layout.sh 预创建的 pane）
#   bash scripts/kaku/launch-codex.sh \
#     --pane-id <id> \
#     --goal "你的任务"
#
# 输出: pane ID
set -euo pipefail

KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"

PARENT_PANE=""
PANE_ID=""
GOAL=""
WORK_DIR="$(pwd)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --parent-pane)  PARENT_PANE="$2"; shift 2 ;;
    --pane-id)      PANE_ID="$2";     shift 2 ;;
    --goal)         GOAL="$2";        shift 2 ;;
    --work-dir)     WORK_DIR="$2";    shift 2 ;;
    *) echo "ERROR: unknown flag $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$PARENT_PANE" && -z "$PANE_ID" ]]; then
  echo "ERROR: --parent-pane or --pane-id required" >&2; exit 1
fi
if [[ -z "$GOAL" ]]; then
  echo "ERROR: --goal required" >&2; exit 1
fi

# Step 1: 获取目标 pane（split 新的或复用已有的）
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

# Step 2: 启动 Codex（交互式，无沙箱）
"$KAKU" cli send-text --pane-id "$PANE" "codex --dangerously-bypass-approvals-and-sandbox"
"$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'

# Step 3: 等待启动 + 跳过更新提示
# Codex 启动时可能弹出更新提示，需要按 Enter 跳过
sleep 3

# 检测是否有更新提示（"Press enter to continue"）
SCREEN=$("$KAKU" cli get-text --pane-id "$PANE" 2>/dev/null || true)
if echo "$SCREEN" | grep -q "Press enter to continue"; then
  "$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'
  sleep 2
  # 可能有第二个确认提示
  SCREEN=$("$KAKU" cli get-text --pane-id "$PANE" 2>/dev/null || true)
  if echo "$SCREEN" | grep -q "Press enter to continue"; then
    "$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'
    sleep 2
  fi
fi

# Step 4: 等待 Codex 就绪（出现 "left ·" 提示符）
MAX_WAIT=30
WAITED=0
while [[ $WAITED -lt $MAX_WAIT ]]; do
  SCREEN=$("$KAKU" cli get-text --pane-id "$PANE" 2>/dev/null || true)
  if echo "$SCREEN" | grep -qE "left · |❯|left$"; then
    break
  fi
  sleep 2
  WAITED=$((WAITED + 2))
done

if [[ $WAITED -ge $MAX_WAIT ]]; then
  echo "WARNING: Codex may not be ready after ${MAX_WAIT}s" >&2
fi

# Step 5: 注入 goal + Enter
"$KAKU" cli send-text --pane-id "$PANE" "$GOAL"
sleep 0.3
"$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'

echo "$PANE"
