#!/usr/bin/env python3
"""lark-conversation-governor skill — Lark 对话治理。"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import time
from typing import Any


_DEFAULT_STOP_SIGNALS = ["停止提醒", "暂停主动", "stop reminders", "disable proactive"]


def _in_quiet_hours(now_hour: int, quiet_hours: list[int]) -> bool:
    if len(quiet_hours) != 2:
        return False
    start, end = quiet_hours
    if start == end:
        return False
    if start < end:
        return start <= now_hour < end
    return now_hour >= start or now_hour < end


def evaluate(args: dict[str, Any]) -> dict[str, Any]:
    text = str(args.get("text", "")).strip()
    proactive = bool(args.get("proactive", True))
    proactive_level = str(args.get("proactive_level", "medium")).strip().lower() or "medium"
    quiet_hours = args.get("quiet_hours", [22, 8])
    if not isinstance(quiet_hours, list):
        quiet_hours = [22, 8]
    stop_signals = args.get("stop_signals", _DEFAULT_STOP_SIGNALS)
    if not isinstance(stop_signals, list):
        stop_signals = _DEFAULT_STOP_SIGNALS

    lowered = text.lower()
    for signal in stop_signals:
        sig = str(signal).strip().lower()
        if sig and sig in lowered:
            return {
                "success": True,
                "decision": "disable_proactive",
                "should_send": False,
                "reason": f"stop signal detected: {signal}",
            }

    now_hour = int(args.get("now_hour", time.localtime().tm_hour))
    if proactive and _in_quiet_hours(now_hour, quiet_hours):
        return {
            "success": True,
            "decision": "defer",
            "should_send": False,
            "reason": "in quiet hours",
        }

    cadence = {"low": "daily", "medium": "half-day", "high": "3x-day"}.get(proactive_level, "half-day")
    return {
        "success": True,
        "decision": "send",
        "should_send": True,
        "cadence": cadence,
        "reason": "policy allows send",
    }


def compose(args: dict[str, Any]) -> dict[str, Any]:
    objective = str(args.get("objective", "")).strip()
    status = str(args.get("status", "")).strip()
    next_step = str(args.get("next_step", "")).strip()
    if not objective:
        return {"success": False, "error": "objective is required"}

    lines = [f"目标：{objective}"]
    if status:
        lines.append(f"当前进度：{status}")
    if next_step:
        lines.append(f"下一步：{next_step}")
    lines.append("如需暂停主动提醒，请直接回复“停止提醒”。")
    return {"success": True, "message": "\n".join(lines)}


def run(args: dict[str, Any]) -> dict[str, Any]:
    action = args.pop("action", "evaluate")
    handlers = {"evaluate": evaluate, "compose": compose}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
