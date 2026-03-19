#!/usr/bin/env python3
"""openseed skill — 单任务 openMax 种子。

Seed a single CC worker in an isolated git worktree.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result
from skill_runner.openmax_utils import (
    inject_brief_context,
    inject_claude_md,
    launch_worker,
    repo_root,
    run_skill_main,
    validate_task_name,
)

load_repo_dotenv(__file__)


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------


def seed(args: dict) -> dict:
    task = str(args.get("task", "")).strip()
    if not task:
        return {"success": False, "error": "--task is required"}

    try:
        validate_task_name(task)
    except ValueError as exc:
        return {"success": False, "error": str(exc)}

    brief_content = str(args.get("brief", "")).strip()
    brief_file = args.get("brief_file", "")
    if not brief_content and brief_file:
        bf = Path(brief_file)
        if not bf.exists():
            return {"success": False, "error": f"brief-file not found: {bf}"}
        brief_content = bf.read_text(encoding="utf-8").strip()

    if not brief_content:
        return {"success": False, "error": "either --brief or --brief-file is required"}

    dry_run = bool(args.get("dry_run", False))
    base_branch = str(args.get("base_branch", "main"))

    root = repo_root()
    brief_dir = root / ".openmax" / "briefs"
    worktree_base = root / ".openmax-worktrees"
    report_dir = root / ".openmax" / "reports"
    pid_dir = root / ".openmax" / "pids"

    brief_path = brief_dir / f"{task}.md"
    worktree_path = worktree_base / f"openmax_{task}"

    if dry_run:
        return {
            "success": True,
            "message": "Dry run — no changes made.",
            "plan": {
                "brief": str(brief_path),
                "worktree": str(worktree_path),
                "branch": f"openmax/{task}",
                "command": "claude --dangerously-skip-permissions --print < <brief>",
            },
        }

    # Write brief.
    brief_dir.mkdir(parents=True, exist_ok=True)
    report_dir.mkdir(parents=True, exist_ok=True)
    brief_path.write_text(brief_content, encoding="utf-8")

    # Create worktree (skip if already exists).
    created = False
    if not (worktree_path.exists() and (worktree_path / ".git").exists()):
        worktree_base.mkdir(parents=True, exist_ok=True)
        try:
            subprocess.run(
                ["git", "worktree", "add", "-b", f"openmax/{task}", str(worktree_path), base_branch],
                cwd=root,
                check=True,
                capture_output=True,
                text=True,
            )
            created = True
        except subprocess.CalledProcessError as exc:
            return {"success": False, "error": f"git worktree add failed: {exc.stderr or exc}"}

    inject_claude_md(worktree_path, task)
    inject_brief_context(brief_path, task, worktree_path)

    try:
        pid = launch_worker(worktree_path, brief_path, pid_dir, task, dry_run=False)
    except RuntimeError as exc:
        return {"success": False, "error": str(exc)}

    log_path = worktree_path / ".openmax_worker.log"
    return {
        "success": True,
        "message": f"Worker launched for task '{task}' (PID {pid}).",
        "task": task,
        "worktree": str(worktree_path),
        "brief": str(brief_path),
        "pid": pid,
        "created_worktree": created,
        "log": str(log_path),
    }


def run(args: dict) -> dict:
    action = args.pop("action", "seed")
    handlers = {"seed": seed}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action!r}. Available: {list(handlers)}"}
    return handler(args)


def main() -> None:
    run_skill_main(run)


if __name__ == "__main__":
    main()
