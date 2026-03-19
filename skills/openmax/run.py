#!/usr/bin/env python3
"""openmax skill — 多 worker 并行编排。

Dispatch N CC workers in isolated git worktrees, collect their reports.
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
    create_worktree,
    inject_brief_context,
    inject_claude_md,
    launch_worker,
    repo_root,
    run_skill_main,
    validate_task_name,
    worker_state,
)

load_repo_dotenv(__file__)


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------


def dispatch(args: dict) -> dict:
    raw_tasks = args.get("tasks", "")
    if not raw_tasks:
        return {"success": False, "error": "--tasks is required (comma-separated task names)"}

    tasks = [t.strip() for t in str(raw_tasks).split(",") if t.strip()]
    goal = args.get("goal", "")
    dry_run = bool(args.get("dry_run", False))
    base_branch = str(args.get("base_branch", "main"))

    root = repo_root()
    brief_dir = Path(args.get("brief_dir", root / ".openmax" / "briefs"))
    worktree_base = Path(args.get("worktree_base", root / ".openmax-worktrees"))
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))
    pid_dir = root / ".openmax" / "pids"

    if not dry_run:
        report_dir.mkdir(parents=True, exist_ok=True)

    results = []
    for task in tasks:
        try:
            validate_task_name(task)
        except ValueError as exc:
            results.append({"task": task, "status": "error", "reason": str(exc)})
            continue

        brief_path = brief_dir / f"{task}.md"
        if not brief_path.exists():
            results.append({"task": task, "status": "skipped", "reason": f"brief not found: {brief_path}"})
            continue

        if goal:
            # Prepend goal header to brief file.
            original = brief_path.read_text(encoding="utf-8")
            if not original.startswith("# Goal"):
                brief_path.write_text(f"# Goal\n{goal}\n\n" + original, encoding="utf-8")

        try:
            worktree_path, created = create_worktree(root, task, base_branch, worktree_base)
        except subprocess.CalledProcessError as exc:
            results.append({"task": task, "status": "error", "reason": str(exc.stderr or exc)})
            continue

        if not dry_run:
            inject_claude_md(worktree_path, task)
            inject_brief_context(brief_path, task, worktree_path)

        try:
            pid = launch_worker(worktree_path, brief_path, pid_dir, task, dry_run)
        except RuntimeError as exc:
            results.append({"task": task, "status": "error", "reason": str(exc)})
            continue

        results.append({
            "task": task,
            "status": "launched" if not dry_run else "dry-run",
            "worktree": str(worktree_path),
            "pid": pid,
            "created": created,
        })

    launched = sum(1 for r in results if r.get("status") == "launched")
    skipped = sum(1 for r in results if r.get("status") in ("skipped", "error"))

    return {
        "success": True,
        "message": f"Dispatched {launched} worker(s), {skipped} skipped/errored.",
        "workers": results,
    }


def status(args: dict) -> dict:
    root = repo_root()
    worktree_base = Path(args.get("worktree_base", root / ".openmax-worktrees"))
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))
    pid_dir = root / ".openmax" / "pids"

    if not worktree_base.exists():
        return {"success": True, "message": "No openmax worktrees found.", "workers": []}

    workers = []
    for d in sorted(worktree_base.iterdir()):
        if not d.is_dir() or not d.name.startswith("openmax_"):
            continue
        task = d.name[len("openmax_"):]
        report_path = report_dir / f"{task}.md"
        log_path = d / ".openmax_worker.log"

        state = worker_state(pid_dir, report_dir, task, d)
        log_tail = ""
        if log_path.exists():
            lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
            log_tail = "\n".join(lines[-5:]) if lines else ""

        workers.append({
            "task": task,
            "worktree": str(d),
            "state": state,
            "done": state == "done",
            "report": str(report_path) if state == "done" else None,
            "log_tail": log_tail,
        })

    done_count = sum(1 for w in workers if w["done"])
    running_count = sum(1 for w in workers if w["state"] == "running")
    return {
        "success": True,
        "message": f"{done_count}/{len(workers)} tasks completed, {running_count} running.",
        "workers": workers,
    }


def collect(args: dict) -> dict:
    root = repo_root()
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))
    task_filter = args.get("task", "")
    synthesize = bool(args.get("synthesize", False))

    if not report_dir.exists():
        return {"success": True, "message": "No reports directory found.", "reports": []}

    reports = []
    for f in sorted(report_dir.glob("*.md")):
        if task_filter and f.stem != task_filter:
            continue
        content = f.read_text(encoding="utf-8")
        reports.append({"task": f.stem, "path": str(f), "content": content})

    result: dict = {
        "success": True,
        "message": f"Collected {len(reports)} report(s).",
        "reports": [{"task": r["task"], "path": r["path"]} for r in reports],
    }

    if synthesize and reports:
        combined = "\n\n---\n\n".join(
            f"## Report: {r['task']}\n\n{r['content']}" for r in reports
        )
        prompt = (
            "You are an engineering lead reviewing multiple parallel worker reports. "
            "Synthesize the following openMax worker reports into a concise executive summary:\n"
            "- What was accomplished overall\n"
            "- Key changes made\n"
            "- Any failures or partial results\n"
            "- Recommended next actions\n\n"
            + combined
        )
        try:
            proc = subprocess.run(
                ["claude", "--dangerously-skip-permissions", "--print", prompt],
                capture_output=True,
                text=True,
                timeout=120,
            )
            result["synthesis"] = proc.stdout.strip() if proc.returncode == 0 else None
            if proc.returncode != 0:
                result["synthesis_error"] = proc.stderr.strip()
        except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
            result["synthesis_error"] = str(exc)

    return result


# ---------------------------------------------------------------------------
# Entry
# ---------------------------------------------------------------------------


def run(args: dict) -> dict:
    action = args.pop("action", "")
    handlers = {
        "dispatch": dispatch,
        "status": status,
        "collect": collect,
    }
    handler = handlers.get(action)
    if not handler:
        return {
            "success": False,
            "error": f"unknown action: {action!r}. Available: {list(handlers)}",
        }
    return handler(args)


def main() -> None:
    run_skill_main(run)


if __name__ == "__main__":
    main()
