#!/usr/bin/env python3
"""openseed skill — 单任务 openMax 种子。

Seed a single CC worker in an isolated git worktree.
"""

from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

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


def seed(args: dict) -> dict:
    task = str(args.get("task", "")).strip()
    if not task:
        return {"success": False, "error": "--task is required"}

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

    root = _repo_root()
    brief_dir = root / ".openmax" / "briefs"
    worktree_base = root / ".openmax-worktrees"
    report_dir = root / ".openmax" / "reports"

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
                "command": f"claude --dangerously-skip-permissions --print <brief>",
            },
        }

    # Write brief.
    brief_dir.mkdir(parents=True, exist_ok=True)
    report_dir.mkdir(parents=True, exist_ok=True)
    brief_path.write_text(brief_content, encoding="utf-8")

    # Create worktree (skip if already exists).
    created = False
    if worktree_path.exists() and (worktree_path / ".git").exists():
        pass  # Already exists — reuse.
    else:
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

    # Copy .env.
    env_src = root / ".env"
    if env_src.is_file():
        shutil.copy(env_src, worktree_path / ".env")

    # Inject task template into CLAUDE.md.
    claude_md = worktree_path / "CLAUDE.md"
    if claude_md.exists():
        existing = claude_md.read_text(encoding="utf-8")
        marker = f"# openMax Task: {task}"
        if marker not in existing:
            claude_md.write_text(
                existing + "\n" + _TASK_REPORT_TEMPLATE.format(task=task),
                encoding="utf-8",
            )

    # Inject context into brief.
    context_block = _BRIEF_CONTEXT_TEMPLATE.format(task=task, worktree_path=worktree_path)
    brief_with_ctx = brief_content
    if "## Context (auto-injected by openMax" not in brief_with_ctx:
        brief_with_ctx = brief_with_ctx + context_block
        brief_path.write_text(brief_with_ctx, encoding="utf-8")

    # Launch worker.
    log_path = worktree_path / ".openmax_worker.log"
    proc = subprocess.Popen(
        ["claude", "--dangerously-skip-permissions", "--print", brief_with_ctx],
        cwd=worktree_path,
        stdout=open(log_path, "w"),
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )

    return {
        "success": True,
        "message": f"Worker launched for task '{task}' (PID {proc.pid}).",
        "task": task,
        "worktree": str(worktree_path),
        "brief": str(brief_path),
        "pid": proc.pid,
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
