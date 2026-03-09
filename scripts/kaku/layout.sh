#!/bin/bash
# layout.sh — Kaku 预置布局脚本
#
# 用法:
#   bash scripts/kaku/layout.sh 4grid [--cwd <dir>] [--pane-id <id>]
#   bash scripts/kaku/layout.sh 4grid --new-window [--cwd <dir>]    # 新窗口（不与用户当前窗口重叠）
#   bash scripts/kaku/layout.sh 4grid --new-tab    [--cwd <dir>]    # 当前窗口新 tab
#   bash scripts/kaku/layout.sh 2h    [--cwd <dir>] [--pane-id <id>]
#   bash scripts/kaku/layout.sh 2v    [--cwd <dir>] [--pane-id <id>]
#   bash scripts/kaku/layout.sh 3col  [--cwd <dir>] [--pane-id <id>]
#
# 输出: 每个 pane 的 ID（按 TOP_LEFT TOP_RIGHT BOT_LEFT BOT_RIGHT 顺序）
# 设置环境变量: KAKU_LAYOUT_PANES（空格分隔的 pane ID 列表）
#
# 依赖: kaku cli（$KAKU_BIN 或 /Applications/Kaku.app/Contents/MacOS/kaku）
set -euo pipefail

KAKU="${KAKU_BIN:-/Applications/Kaku.app/Contents/MacOS/kaku}"
LAYOUT="${1:-4grid}"
CWD="$(pwd)"
PARENT=""
NEW_WINDOW=false
NEW_TAB=false

# 解析参数
shift || true
while [[ $# -gt 0 ]]; do
  case "$1" in
    --cwd)         CWD="$2";    shift 2 ;;
    --pane-id)     PARENT="$2"; shift 2 ;;
    --new-window)  NEW_WINDOW=true; shift ;;
    --new-tab)     NEW_TAB=true;    shift ;;
    *)             shift ;;
  esac
done

# 确定起始 pane
if [[ "$NEW_WINDOW" == "true" ]]; then
  # 在新独立窗口创建第一个 pane（不影响用户当前窗口）
  PARENT=$("$KAKU" cli spawn --new-window --cwd "$CWD" -- zsh -l)
  # 激活新窗口使其可见
  "$KAKU" cli activate-pane --pane-id "$PARENT"
elif [[ "$NEW_TAB" == "true" ]]; then
  # 在当前窗口新 tab 创建第一个 pane
  PARENT=$("$KAKU" cli spawn --cwd "$CWD" -- zsh -l)
else
  # 沿用已有 pane（--pane-id 或 KAKU_PANE_ID）
  if [[ -z "$PARENT" ]]; then
    PARENT="${KAKU_PANE_ID:-}"
  fi
  if [[ -z "$PARENT" ]]; then
    echo "ERROR: parent pane ID required (--pane-id, KAKU_PANE_ID, --new-window, or --new-tab)" >&2
    exit 1
  fi
fi

kaku_split() {
  local parent="$1" dir="$2" pct="$3" cwd="$4"
  "$KAKU" cli split-pane \
    --pane-id "$parent" \
    "--${dir}" \
    --percent "$pct" \
    --cwd "$cwd" \
    -- zsh -l
}

case "$LAYOUT" in
  # ┌──────┬──────┐
  # │  TL  │  TR  │
  # ├──────┼──────┤
  # │  BL  │  BR  │
  # └──────┴──────┘
  4grid|quad)
    TL=$PARENT
    TR=$(kaku_split "$TL" right 50 "$CWD")
    BL=$(kaku_split "$TL" bottom 50 "$CWD")
    BR=$(kaku_split "$TR" bottom 50 "$CWD")

    echo "4grid layout:"
    printf "  TOP_LEFT=%-6s  TOP_RIGHT=%-6s\n" "$TL" "$TR"
    printf "  BOT_LEFT=%-6s  BOT_RIGHT=%-6s\n" "$BL" "$BR"
    export KAKU_LAYOUT_PANES="$TL $TR $BL $BR"
    ;;

  # ┌────────────┐
  # │    TOP     │
  # ├────────────┤
  # │   BOTTOM   │
  # └────────────┘
  2h|horizontal|top-bottom)
    TOP=$PARENT
    BOT=$(kaku_split "$TOP" bottom 50 "$CWD")

    echo "2h layout:"
    printf "  TOP=%-6s  BOTTOM=%-6s\n" "$TOP" "$BOT"
    export KAKU_LAYOUT_PANES="$TOP $BOT"
    ;;

  # ┌──────┬──────┐
  # │      │      │
  # │ LEFT │ RIGHT│
  # │      │      │
  # └──────┴──────┘
  2v|vertical|left-right)
    LEFT=$PARENT
    RIGHT=$(kaku_split "$LEFT" right 50 "$CWD")

    echo "2v layout:"
    printf "  LEFT=%-6s  RIGHT=%-6s\n" "$LEFT" "$RIGHT"
    export KAKU_LAYOUT_PANES="$LEFT $RIGHT"
    ;;

  # ┌──────┬──────┬──────┐
  # │      │      │      │
  # │  L   │  M   │  R   │
  # │      │      │      │
  # └──────┴──────┴──────┘
  3col|three)
    L=$PARENT
    M=$(kaku_split "$L" right 67 "$CWD")
    R=$(kaku_split "$M" right 50 "$CWD")

    echo "3col layout:"
    printf "  LEFT=%-6s  MID=%-6s  RIGHT=%-6s\n" "$L" "$M" "$R"
    export KAKU_LAYOUT_PANES="$L $M $R"
    ;;

  # ┌────────────┐
  # │    MAIN    │
  # ├──────┬─────┤
  # │  BL  │  BR │
  # └──────┴─────┘
  1+2|main-bottom)
    MAIN=$PARENT
    BL=$(kaku_split "$MAIN" bottom 35 "$CWD")
    BR=$(kaku_split "$BL" right 50 "$CWD")

    echo "1+2 layout:"
    printf "  MAIN=%-6s  BOT_LEFT=%-6s  BOT_RIGHT=%-6s\n" "$MAIN" "$BL" "$BR"
    export KAKU_LAYOUT_PANES="$MAIN $BL $BR"
    ;;

  help|--help|-h)
    cat <<'HELP'
Kaku Layout Presets:

  4grid   (quad)          2x2 四方格
  2h      (horizontal)    上下两行
  2v      (vertical)      左右两列
  3col    (three)         左中右三列
  1+2     (main-bottom)   上大下双（主+监控对）

Usage:
  KAKU_PANE_ID=<id> bash scripts/kaku/layout.sh <preset> [--cwd <dir>]
  bash scripts/kaku/layout.sh <preset> --pane-id <id> [--cwd <dir>]
HELP
    ;;

  *)
    echo "ERROR: unknown layout '${LAYOUT}'. Run with 'help' to see options." >&2
    exit 1
    ;;
esac
