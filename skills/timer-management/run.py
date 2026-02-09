#!/usr/bin/env python3
"""timer-management skill — 定时提醒管理。

转发到 scripts/cli/timer/timer_cli.py。
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

_SCRIPTS = Path(__file__).resolve().parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

from cli.timer.timer_cli import cancel_timer, list_timers, set_timer


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}

    action = args.pop("action", "list")

    if action == "set":
        result = set_timer(args)
    elif action == "list":
        result = list_timers()
    elif action == "cancel":
        result = cancel_timer(args)
    else:
        result = {"success": False, "error": f"unknown action: {action}"}

    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
