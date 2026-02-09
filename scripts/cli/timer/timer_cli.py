#!/usr/bin/env python3
"""Thin CLI wrapper for elephant.ai timer management.

This CLI communicates with the running elephant.ai backend via a Unix socket
or HTTP endpoint to manage timers. For standalone testing it provides a
file-based fallback.

Usage:
    timer_cli.py set   '{"delay":"30m", "task":"remind me to drink water"}'
    timer_cli.py list
    timer_cli.py cancel '{"id":"timer-123"}'
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from pathlib import Path

_TIMER_FILE = Path(os.environ.get("ELEPHANT_TIMER_FILE", "/tmp/elephant_timers.json"))


def _load_timers() -> list[dict]:
    if _TIMER_FILE.exists():
        return json.loads(_TIMER_FILE.read_text())
    return []


def _save_timers(timers: list[dict]) -> None:
    _TIMER_FILE.write_text(json.dumps(timers, ensure_ascii=False, indent=2))


def _parse_delay(delay_str: str) -> int:
    """Parse delay string like '30m', '2h', '90s' to seconds."""
    s = delay_str.strip().lower()
    if s.endswith("h"):
        return int(s[:-1]) * 3600
    if s.endswith("m"):
        return int(s[:-1]) * 60
    if s.endswith("s"):
        return int(s[:-1])
    return int(s)


def set_timer(args: dict) -> dict:
    delay = args.get("delay", "")
    task = args.get("task", "")
    if not delay or not task:
        return {"success": False, "error": "delay and task are required"}

    seconds = _parse_delay(delay)
    fire_at = time.time() + seconds
    timer_id = f"timer-{int(time.time() * 1000) % 100000}"

    timers = _load_timers()
    entry = {
        "id": timer_id,
        "task": task,
        "delay": delay,
        "fire_at": fire_at,
        "fire_at_human": time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(fire_at)),
        "created": time.time(),
        "status": "active",
    }
    timers.append(entry)
    _save_timers(timers)

    return {"success": True, "timer": entry}


def list_timers() -> dict:
    timers = _load_timers()
    now = time.time()
    for t in timers:
        if t["status"] == "active" and t["fire_at"] < now:
            t["status"] = "fired"
    _save_timers(timers)
    return {"success": True, "timers": timers, "count": len(timers)}


def cancel_timer(args: dict) -> dict:
    timer_id = args.get("id", "")
    if not timer_id:
        return {"success": False, "error": "id is required"}

    timers = _load_timers()
    found = False
    for t in timers:
        if t["id"] == timer_id:
            t["status"] = "cancelled"
            found = True
            break

    if not found:
        return {"success": False, "error": f"timer {timer_id} not found"}

    _save_timers(timers)
    return {"success": True, "message": f"timer {timer_id} cancelled"}


def main() -> None:
    parser = argparse.ArgumentParser(description="Timer management CLI")
    parser.add_argument("action", choices=["set", "list", "cancel"])
    parser.add_argument("args", nargs="?", default="{}")
    parsed = parser.parse_args()

    args = json.loads(parsed.args) if parsed.args != "{}" else {}

    if parsed.action == "set":
        result = set_timer(args)
    elif parsed.action == "list":
        result = list_timers()
    elif parsed.action == "cancel":
        result = cancel_timer(args)
    else:
        result = {"success": False, "error": f"unknown action: {parsed.action}"}

    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
