#!/usr/bin/env python3
"""reminder-scheduler skill — 单次提醒与周期计划统一调度。"""

from __future__ import annotations

import datetime as dt
import json
import os
from pathlib import Path
import sys
from typing import Any
import uuid

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

from cli.timer.timer_cli import cancel_timer, list_timers, set_timer

_PLAN_STORE_PATH = Path(
    os.environ.get(
        "ALEX_REMINDER_SCHEDULER_STORE",
        os.path.expanduser("~/.alex/reminder-scheduler/plans.json"),
    )
)


def _ensure_plan_store() -> None:
    _PLAN_STORE_PATH.parent.mkdir(parents=True, exist_ok=True)
    if not _PLAN_STORE_PATH.exists():
        _PLAN_STORE_PATH.write_text("[]\n", encoding="utf-8")


def _load_plans() -> list[dict[str, Any]]:
    _ensure_plan_store()
    return json.loads(_PLAN_STORE_PATH.read_text(encoding="utf-8"))


def _save_plans(plans: list[dict[str, Any]]) -> None:
    _ensure_plan_store()
    _PLAN_STORE_PATH.write_text(
        json.dumps(plans, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def _now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()


def _parse_iso(value: str) -> dt.datetime | None:
    cleaned = value.strip()
    if not cleaned:
        return None
    normalized = cleaned.replace("Z", "+00:00")
    try:
        parsed = dt.datetime.fromisoformat(normalized)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=dt.timezone.utc)
    return parsed


def set_once(args: dict[str, Any]) -> dict[str, Any]:
    return set_timer(args)


def list_once(_: dict[str, Any]) -> dict[str, Any]:
    return list_timers()


def cancel_once(args: dict[str, Any]) -> dict[str, Any]:
    return cancel_timer(args)


def upsert_plan(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    schedule = str(args.get("schedule", "")).strip()
    task = str(args.get("task", "")).strip()
    if not name or not schedule or not task:
        return {"success": False, "error": "name, schedule, task are required"}

    plans = _load_plans()
    now = _now_iso()

    for plan in plans:
        if plan.get("name") != name:
            continue
        plan["schedule"] = schedule
        plan["task"] = task
        plan["channel"] = str(args.get("channel", plan.get("channel", "lark"))).strip() or "lark"
        plan["enabled"] = bool(args.get("enabled", plan.get("enabled", True)))
        plan["metadata"] = args.get("metadata", plan.get("metadata", {}))
        incoming_next_run = str(args.get("next_run_at", "")).strip()
        if incoming_next_run:
            plan["next_run_at"] = incoming_next_run
        plan["updated_at"] = now
        _save_plans(plans)
        return {"success": True, "action": "updated", "plan": plan, "event": "reminder.plan_upserted"}

    plan = {
        "id": str(uuid.uuid4())[:8],
        "name": name,
        "schedule": schedule,
        "task": task,
        "channel": str(args.get("channel", "lark")).strip() or "lark",
        "enabled": bool(args.get("enabled", True)),
        "metadata": args.get("metadata", {}),
        "created_at": now,
        "updated_at": now,
        "last_run_at": "",
        "next_run_at": str(args.get("next_run_at", "")).strip(),
    }
    plans.append(plan)
    _save_plans(plans)
    return {"success": True, "action": "created", "plan": plan, "event": "reminder.plan_upserted"}


def list_plans(_: dict[str, Any]) -> dict[str, Any]:
    plans = _load_plans()
    return {"success": True, "plans": plans, "count": len(plans)}


def delete_plan(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    plan_id = str(args.get("id", "")).strip()
    if not name and not plan_id:
        return {"success": False, "error": "name or id is required"}

    plans = _load_plans()
    remain = [plan for plan in plans if plan.get("name") != name and plan.get("id") != plan_id]
    removed = len(plans) - len(remain)
    if removed == 0:
        return {"success": False, "error": "plan not found"}

    _save_plans(remain)
    return {"success": True, "removed": removed, "event": "reminder.plan_deleted"}


def due_plans(args: dict[str, Any]) -> dict[str, Any]:
    now = _parse_iso(str(args.get("now", "")).strip())
    if now is None:
        now = dt.datetime.now(dt.timezone.utc)

    due: list[dict[str, Any]] = []
    for plan in _load_plans():
        if not bool(plan.get("enabled", True)):
            continue
        next_run_at = _parse_iso(str(plan.get("next_run_at", "")).strip())
        if next_run_at is None:
            continue
        if next_run_at <= now:
            due.append(plan)

    return {"success": True, "due": due, "count": len(due)}


def touch_plan(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    plan_id = str(args.get("id", "")).strip()
    next_run_at = str(args.get("next_run_at", "")).strip()
    if not name and not plan_id:
        return {"success": False, "error": "name or id is required"}

    plans = _load_plans()
    now = _now_iso()
    for plan in plans:
        if plan.get("name") != name and plan.get("id") != plan_id:
            continue
        plan["last_run_at"] = now
        if next_run_at:
            plan["next_run_at"] = next_run_at
        plan["updated_at"] = now
        _save_plans(plans)
        return {"success": True, "plan": plan}

    return {"success": False, "error": "plan not found"}


def run(args: dict[str, Any]) -> dict[str, Any]:
    action = str(args.pop("action", "list_once")).strip()
    handlers = {
        "set_once": set_once,
        "list_once": list_once,
        "cancel_once": cancel_once,
        "upsert_plan": upsert_plan,
        "list_plans": list_plans,
        "delete_plan": delete_plan,
        "due_plans": due_plans,
        "touch_plan": touch_plan,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


def main() -> None:
    args = parse_cli_args(sys.argv[1:])
    result = run(args)
    stdout_text, stderr_text, exit_code = render_result(result)
    if stdout_text:
        sys.stdout.write(stdout_text)
        if not stdout_text.endswith("\n"):
            sys.stdout.write("\n")
    if stderr_text:
        sys.stderr.write(stderr_text)
        if not stderr_text.endswith("\n"):
            sys.stderr.write("\n")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
