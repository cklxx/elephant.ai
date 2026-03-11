#!/bin/zsh
# monitor.sh — 批量监控多个 agent pane 状态
#
# 用法:
#   bash scripts/kaku/monitor.sh [--panes "7 8 9 10"] [--lines 8] [--watch 30]
#
# 参数:
#   --panes   空格分隔的 pane ID 列表（默认：自动发现非 leader pane）
#   --lines   每个 pane 显示的行数（默认 8）
#   --watch   持续监控间隔秒数（默认 0 = 单次）
set -euo pipefail

KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"
PANES=""
LINES_N=8
WATCH=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --panes)  PANES="$2";    shift 2 ;;
    --lines)  LINES_N="$2";  shift 2 ;;
    --watch)  WATCH="$2";    shift 2 ;;
    *) echo "ERROR: unknown flag $1" >&2; exit 1 ;;
  esac
done

# 如果未指定 pane，自动获取所有
if [[ -z "$PANES" ]]; then
  PANES=$("$KAKU" cli list 2>/dev/null | awk 'NR>1 {print $3}' | tr '\n' ' ')
fi

detect_status() {
  local text="$1"
  if echo "$text" | grep -qE '✻ Cooked for|✓ Done'; then
    echo "✅ DONE"
  elif echo "$text" | grep -qE 'tokens used'; then
    echo "✅ DONE (codex)"
  elif echo "$text" | grep -qE '⏺|●|Working|Pondering|Thundering'; then
    echo "🔄 WORKING"
  elif echo "$text" | grep -qE '❯\s*$'; then
    echo "⏸️  IDLE"
  elif echo "$text" | grep -qE '\$\s*$|%\s*$|bash-[0-9]'; then
    echo "💤 SHELL"
  else
    echo "❓ UNKNOWN"
  fi
}

run_check() {
  echo "=== $(date '+%H:%M:%S') ==="
  for pane in $PANES; do
    TEXT=$("$KAKU" cli get-text --pane-id "$pane" 2>/dev/null || echo "(unreachable)")
    STATUS=$(detect_status "$TEXT")
    TAIL=$(echo "$TEXT" | tail -"$LINES_N")
    echo ""
    echo "--- Pane $pane [$STATUS] ---"
    echo "$TAIL"
  done
  echo ""
}

if [[ $WATCH -gt 0 ]]; then
  while true; do
    clear
    run_check
    sleep "$WATCH"
  done
else
  run_check
fi
