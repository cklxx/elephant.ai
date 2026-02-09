#!/usr/bin/env python3
"""background-tasks skill — 后台任务管理。

创建/查询/收集后台长时间运行的任务。
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path

_BG_DIR = Path(os.environ.get("ALEX_BG_DIR", os.path.expanduser("~/.alex/background")))


def _ensure_dir():
    _BG_DIR.mkdir(parents=True, exist_ok=True)


def dispatch(args: dict) -> dict:
    command = args.get("command", "")
    if not command:
        return {"success": False, "error": "command is required"}

    _ensure_dir()
    task_id = str(uuid.uuid4())[:8]
    output_file = _BG_DIR / f"{task_id}.out"
    meta_file = _BG_DIR / f"{task_id}.json"

    # Launch in background
    proc = subprocess.Popen(
        command, shell=True,
        stdout=open(output_file, "w"),
        stderr=subprocess.STDOUT,
        cwd=args.get("cwd", "."),
    )

    meta = {
        "id": task_id,
        "command": command,
        "description": args.get("description", ""),
        "pid": proc.pid,
        "status": "running",
        "started": time.strftime("%Y-%m-%d %H:%M"),
        "output_file": str(output_file),
    }
    meta_file.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding="utf-8")

    return {"success": True, "task_id": task_id, "pid": proc.pid, "message": f"后台任务已启动 (PID {proc.pid})"}


def list_tasks(args: dict) -> dict:
    _ensure_dir()
    tasks = []
    for f in sorted(_BG_DIR.glob("*.json")):
        meta = json.loads(f.read_text(encoding="utf-8"))
        # Check if still running
        pid = meta.get("pid")
        if pid and meta.get("status") == "running":
            try:
                os.kill(pid, 0)
            except OSError:
                meta["status"] = "completed"
                f.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding="utf-8")
        tasks.append(meta)
    return {"success": True, "tasks": tasks, "count": len(tasks)}


def collect(args: dict) -> dict:
    task_id = args.get("task_id", "")
    if not task_id:
        return {"success": False, "error": "task_id is required"}

    meta_file = _BG_DIR / f"{task_id}.json"
    output_file = _BG_DIR / f"{task_id}.out"

    if not meta_file.exists():
        return {"success": False, "error": f"task '{task_id}' not found"}

    meta = json.loads(meta_file.read_text(encoding="utf-8"))
    output = ""
    if output_file.exists():
        output = output_file.read_text(encoding="utf-8", errors="replace")[-10000:]

    return {"success": True, "task": meta, "output": output}


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"dispatch": dispatch, "list": list_tasks, "collect": collect}
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
