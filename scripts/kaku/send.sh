#!/bin/bash
# send.sh — 向 Kaku pane 发送文本并自动提交（按下 Enter）
#
# 用法:
#   bash scripts/kaku/send.sh --pane-id <id> "命令文本"
#   bash scripts/kaku/send.sh --pane-id <id> --no-submit "只注入不回车"
#
# 说明:
#   kaku cli send-text 本身没有自动回车功能。
#   此脚本封装两步操作（paste + enter），让 LLM/脚本只需一行调用。
#   对交互式 CLI（Claude Code / Codex）—— 先 paste 文字，再发 \r 触发提交。
#
# 示例:
#   bash scripts/kaku/send.sh --pane-id 42 "ls -la"
#   bash scripts/kaku/send.sh --pane-id 42 --no-submit "partial text"
set -euo pipefail

KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"
PANE=""
SUBMIT=true
TEXT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pane-id)   PANE="$2";      shift 2 ;;
    --no-submit) SUBMIT=false;   shift ;;
    -*)          echo "ERROR: unknown flag $1" >&2; exit 1 ;;
    *)           TEXT="$1";      shift ;;
  esac
done

if [[ -z "$PANE" ]]; then
  echo "ERROR: --pane-id required" >&2
  exit 1
fi
if [[ -z "$TEXT" ]]; then
  echo "ERROR: text argument required" >&2
  exit 1
fi

# Step 1: paste text into the pane (bracketed paste mode friendly)
"$KAKU" cli send-text --pane-id "$PANE" "$TEXT"

# Step 2: send carriage return to submit (unless --no-submit)
if [[ "$SUBMIT" == "true" ]]; then
  "$KAKU" cli send-text --no-paste --pane-id "$PANE" $'\r'
fi
