#!/usr/bin/env python3
"""task-delegation skill — 跨 Agent 任务委派。

构建委派任务包，供 LLM 通过 bridge 执行。
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

_TASKS_DIR = Path(os.environ.get("ALEX_TASKS_DIR", os.path.expanduser("~/.alex/tasks")))

_AGENTS = {
    "codex": {"cli": "codex", "sandbox": "workspace-write"},
    "claude": {"cli": "claude", "sandbox": "interactive"},
    "gemini": {"cli": "gemini", "sandbox": "default"},
}


def dispatch(args: dict) -> dict:
    agent = args.get("agent", "codex")
    task = args.get("task", "")
    if not task:
        return {"success": False, "error": "task is required"}
    if agent not in _AGENTS:
        return {"success": False, "error": f"unknown agent: {agent}, available: {list(_AGENTS.keys())}"}

    _TASKS_DIR.mkdir(parents=True, exist_ok=True)
    task_id = str(uuid.uuid4())[:8]
    task_record = {
        "id": task_id,
        "agent": agent,
        "task": task,
        "cwd": args.get("cwd", os.getcwd()),
        "context": args.get("context", ""),
        "status": "pending",
        "created": time.strftime("%Y-%m-%d %H:%M"),
    }
    (_TASKS_DIR / f"{task_id}.json").write_text(
        json.dumps(task_record, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    return {
        "success": True,
        "task_id": task_id,
        "agent": agent,
        "message": f"任务已创建，待通过 bridge 委派给 {agent}",
        "execution_hint": (
            f"使用 bash 执行:\n"
            f"alex exec --agent {agent} --task-id {task_id}"
        ),
    }


def list_tasks(args: dict) -> dict:
    if not _TASKS_DIR.exists():
        return {"success": True, "tasks": [], "count": 0}
    tasks = []
    for f in sorted(_TASKS_DIR.glob("*.json")):
        tasks.append(json.loads(f.read_text(encoding="utf-8")))
    return {"success": True, "tasks": tasks, "count": len(tasks)}


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"dispatch": dispatch, "list": list_tasks}
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
