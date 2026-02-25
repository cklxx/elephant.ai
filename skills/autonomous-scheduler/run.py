#!/usr/bin/env python3
"""autonomous-scheduler skill — 自主调度管理。"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import datetime as dt
import json
import os
import uuid
from pathlib import Path
from typing import Any

_STORE_PATH = Path(
    os.environ.get(
        "ALEX_AUTONOMOUS_SCHEDULER_STORE",
        os.path.expanduser("~/.alex/autonomous-scheduler/jobs.json"),
    )
)


def _ensure_store() -> None:
    _STORE_PATH.parent.mkdir(parents=True, exist_ok=True)
    if not _STORE_PATH.exists():
        _STORE_PATH.write_text("[]\n", encoding="utf-8")


def _load_jobs() -> list[dict[str, Any]]:
    _ensure_store()
    return json.loads(_STORE_PATH.read_text(encoding="utf-8"))


def _save_jobs(jobs: list[dict[str, Any]]) -> None:
    _ensure_store()
    _STORE_PATH.write_text(json.dumps(jobs, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()


def _parse_time(value: str) -> dt.datetime | None:
    if not value:
        return None
    normalized = value.replace("Z", "+00:00")
    try:
        parsed = dt.datetime.fromisoformat(normalized)
        if parsed.tzinfo is None:
            return parsed.replace(tzinfo=dt.timezone.utc)
        return parsed
    except ValueError:
        return None


def upsert(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    schedule = str(args.get("schedule", "")).strip()
    task = str(args.get("task", "")).strip()
    if not name or not schedule or not task:
        return {"success": False, "error": "name, schedule, task are required"}

    jobs = _load_jobs()
    now = _now_iso()
    for job in jobs:
        if job.get("name") != name:
            continue
        job["schedule"] = schedule
        job["task"] = task
        job["channel"] = str(args.get("channel", job.get("channel", "lark"))).strip() or "lark"
        job["enabled"] = bool(args.get("enabled", job.get("enabled", True)))
        job["metadata"] = args.get("metadata", job.get("metadata", {}))
        job["updated_at"] = now
        _save_jobs(jobs)
        return {"success": True, "action": "updated", "job": job, "event": "schedule.updated"}

    job = {
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
    jobs.append(job)
    _save_jobs(jobs)
    return {"success": True, "action": "created", "job": job, "event": "schedule.created"}


def list_jobs(_: dict[str, Any]) -> dict[str, Any]:
    jobs = _load_jobs()
    return {"success": True, "jobs": jobs, "count": len(jobs)}


def delete(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    job_id = str(args.get("id", "")).strip()
    if not name and not job_id:
        return {"success": False, "error": "name or id is required"}

    jobs = _load_jobs()
    remain = [j for j in jobs if j.get("name") != name and j.get("id") != job_id]
    removed = len(jobs) - len(remain)
    if removed == 0:
        return {"success": False, "error": "job not found"}
    _save_jobs(remain)
    return {"success": True, "removed": removed, "event": "schedule.deleted"}


def due(args: dict[str, Any]) -> dict[str, Any]:
    jobs = _load_jobs()
    now = _parse_time(str(args.get("now", "")).strip())
    if now is None:
        now = dt.datetime.now(dt.timezone.utc)

    due_jobs = []
    for job in jobs:
        if not bool(job.get("enabled", True)):
            continue
        next_run_at = _parse_time(str(job.get("next_run_at", "")).strip())
        if next_run_at is None:
            continue
        if next_run_at <= now:
            due_jobs.append(job)

    return {"success": True, "due": due_jobs, "count": len(due_jobs)}


def touch_run(args: dict[str, Any]) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    job_id = str(args.get("id", "")).strip()
    next_run_at = str(args.get("next_run_at", "")).strip()
    if not name and not job_id:
        return {"success": False, "error": "name or id is required"}

    jobs = _load_jobs()
    now = _now_iso()
    for job in jobs:
        if job.get("name") != name and job.get("id") != job_id:
            continue
        job["last_run_at"] = now
        if next_run_at:
            job["next_run_at"] = next_run_at
        job["updated_at"] = now
        _save_jobs(jobs)
        return {"success": True, "job": job}

    return {"success": False, "error": "job not found"}


def run(args: dict[str, Any]) -> dict[str, Any]:
    action = args.pop("action", "list")
    handlers = {
        "upsert": upsert,
        "list": list_jobs,
        "delete": delete,
        "due": due,
        "touch_run": touch_run,
    }
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
