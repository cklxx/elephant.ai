#!/usr/bin/env python3
"""scheduled-tasks skill — 定时任务管理。

Cron 调度任务 CRUD，存储为 JSON 文件。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import os
import sys
import time
import uuid
from pathlib import Path

_JOBS_DIR = Path(os.environ.get("ALEX_JOBS_DIR", os.path.expanduser("~/.alex/jobs")))


def _ensure_dir():
    _JOBS_DIR.mkdir(parents=True, exist_ok=True)


def _load_jobs() -> list[dict]:
    _ensure_dir()
    jobs_file = _JOBS_DIR / "jobs.json"
    if not jobs_file.exists():
        return []
    return json.loads(jobs_file.read_text(encoding="utf-8"))


def _save_jobs(jobs: list[dict]):
    _ensure_dir()
    (_JOBS_DIR / "jobs.json").write_text(
        json.dumps(jobs, ensure_ascii=False, indent=2), encoding="utf-8"
    )


def create(args: dict) -> dict:
    name = args.get("name", "")
    cron = args.get("cron", "")
    command = args.get("command", "")
    if not name or not cron:
        return {"success": False, "error": "name and cron are required"}

    jobs = _load_jobs()
    if any(j["name"] == name for j in jobs):
        return {"success": False, "error": f"job '{name}' already exists"}

    job = {
        "id": str(uuid.uuid4())[:8],
        "name": name,
        "cron": cron,
        "command": command,
        "description": args.get("description", ""),
        "enabled": True,
        "created": time.strftime("%Y-%m-%d %H:%M"),
    }
    jobs.append(job)
    _save_jobs(jobs)
    return {"success": True, "job": job, "message": f"定时任务「{name}」已创建"}


def list_jobs(args: dict) -> dict:
    jobs = _load_jobs()
    return {"success": True, "jobs": jobs, "count": len(jobs)}


def delete(args: dict) -> dict:
    name = args.get("name", "")
    job_id = args.get("id", "")
    if not name and not job_id:
        return {"success": False, "error": "name or id is required"}

    jobs = _load_jobs()
    remaining = [j for j in jobs if j["name"] != name and j["id"] != job_id]
    if len(remaining) == len(jobs):
        return {"success": False, "error": f"job not found"}
    _save_jobs(remaining)
    return {"success": True, "message": f"已删除 {len(jobs) - len(remaining)} 个任务"}


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"create": create, "list": list_jobs, "delete": delete}
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
