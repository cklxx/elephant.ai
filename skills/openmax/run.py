#!/usr/bin/env python3
"""openmax skill — 多 worker 并行编排。

Dispatch N CC workers in isolated git worktrees, collect their reports.
"""

from __future__ import annotations

import os
import subprocess
import time
from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_TASK_REPORT_TEMPLATE = """\

# openMax Task: {task}

When you complete your task, write a completion report to `.openmax/reports/{task}.md`:

```markdown
## Status
done | error | partial

## Summary
<What was accomplished in 1-2 sentences>

## Changes
- <file>: <what changed>

## Test Results
<pass/fail details>
```

This report is read by the orchestrator — always write it before finishing.
"""

_BRIEF_CONTEXT_TEMPLATE = """\

## Context (auto-injected by openMax — use only if relevant)

Working directory: {worktree_path}
You are already in the correct directory. Do NOT run `cd`.

Branch: openmax/{task} (isolated worktree — commit here, do not switch branches)
"""


def _repo_root() -> Path:
    try:
        out = subprocess.check_output(
            ["git", "rev-parse", "--show-toplevel"], text=True
        ).strip()
        return Path(out)
    except subprocess.CalledProcessError:
        return Path.cwd()


def _worktree_exists(path: Path) -> bool:
    return path.exists() and (path / ".git").exists()


def _create_worktree(root: Path, task: str, base_branch: str, worktree_base: Path) -> tuple[Path, bool]:
    """Create git worktree. Returns (path, created). created=False if already exists."""
    worktree_path = worktree_base / f"openmax_{task}"
    branch = f"openmax/{task}"

    if _worktree_exists(worktree_path):
        return worktree_path, False

    worktree_base.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["git", "worktree", "add", "-b", branch, str(worktree_path), base_branch],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )

    # Copy .env if present.
    env_src = root / ".env"
    if env_src.is_file():
        import shutil
        shutil.copy(env_src, worktree_path / ".env")

    return worktree_path, True


def _inject_claude_md(worktree_path: Path, task: str) -> None:
    """Append task-report template to CLAUDE.md in the worktree."""
    claude_md = worktree_path / "CLAUDE.md"
    if not claude_md.exists():
        return
    existing = claude_md.read_text(encoding="utf-8")
    marker = f"# openMax Task: {task}"
    if marker in existing:
        return  # Already injected.
    claude_md.write_text(
        existing + "\n" + _TASK_REPORT_TEMPLATE.format(task=task),
        encoding="utf-8",
    )


def _inject_brief_context(brief_path: Path, task: str, worktree_path: Path) -> None:
    """Append context block to brief if not already present."""
    if not brief_path.exists():
        return
    existing = brief_path.read_text(encoding="utf-8")
    marker = "## Context (auto-injected by openMax"
    if marker in existing:
        return
    brief_path.write_text(
        existing + _BRIEF_CONTEXT_TEMPLATE.format(task=task, worktree_path=worktree_path),
        encoding="utf-8",
    )


def _launch_worker(worktree_path: Path, brief_content: str, dry_run: bool) -> int | None:
    """Launch claude in background. Returns PID or None on dry-run."""
    if dry_run:
        return None
    log_path = worktree_path / ".openmax_worker.log"
    proc = subprocess.Popen(
        ["claude", "--dangerously-skip-permissions", "--print", brief_content],
        cwd=worktree_path,
        stdout=open(log_path, "w"),
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    return proc.pid


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

    root = _repo_root()
    brief_dir = Path(args.get("brief_dir", root / ".openmax" / "briefs"))
    worktree_base = Path(args.get("worktree_base", root / ".openmax-worktrees"))

    # Ensure output dir for reports exists.
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))
    if not dry_run:
        report_dir.mkdir(parents=True, exist_ok=True)

    results = []
    for task in tasks:
        brief_path = brief_dir / f"{task}.md"
        if not brief_path.exists():
            results.append({"task": task, "status": "skipped", "reason": f"brief not found: {brief_path}"})
            continue

        brief_content = brief_path.read_text(encoding="utf-8")
        if goal:
            brief_content = f"# Goal\n{goal}\n\n" + brief_content

        try:
            worktree_path, created = _create_worktree(root, task, base_branch, worktree_base)
        except subprocess.CalledProcessError as exc:
            results.append({"task": task, "status": "error", "reason": str(exc.stderr or exc)})
            continue

        if not dry_run:
            _inject_claude_md(worktree_path, task)
            _inject_brief_context(brief_path, task, worktree_path)
            # Re-read brief with injected context.
            brief_content = brief_path.read_text(encoding="utf-8")
            if goal:
                brief_content = f"# Goal\n{goal}\n\n" + brief_content

        pid = _launch_worker(worktree_path, brief_content, dry_run)

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


def status(args: dict) -> dict:  # noqa: ARG001
    root = _repo_root()
    worktree_base = Path(args.get("worktree_base", root / ".openmax-worktrees"))
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))

    if not worktree_base.exists():
        return {"success": True, "message": "No openmax worktrees found.", "workers": []}

    workers = []
    for d in sorted(worktree_base.iterdir()):
        if not d.is_dir() or not d.name.startswith("openmax_"):
            continue
        task = d.name[len("openmax_"):]
        report_path = report_dir / f"{task}.md"
        log_path = d / ".openmax_worker.log"

        done = report_path.exists()
        log_tail = ""
        if log_path.exists():
            lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
            log_tail = "\n".join(lines[-5:]) if lines else ""

        workers.append({
            "task": task,
            "worktree": str(d),
            "done": done,
            "report": str(report_path) if done else None,
            "log_tail": log_tail,
        })

    done_count = sum(1 for w in workers if w["done"])
    return {
        "success": True,
        "message": f"{done_count}/{len(workers)} tasks completed.",
        "workers": workers,
    }


def collect(args: dict) -> dict:
    root = _repo_root()
    report_dir = Path(args.get("report_dir", root / ".openmax" / "reports"))
    task_filter = args.get("task", "")

    if not report_dir.exists():
        return {"success": True, "message": "No reports directory found.", "reports": []}

    reports = []
    for f in sorted(report_dir.glob("*.md")):
        if task_filter and f.stem != task_filter:
            continue
        content = f.read_text(encoding="utf-8")
        reports.append({"task": f.stem, "path": str(f), "content": content})

    return {
        "success": True,
        "message": f"Collected {len(reports)} report(s).",
        "reports": [{"task": r["task"], "path": r["path"]} for r in reports],
    }


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
