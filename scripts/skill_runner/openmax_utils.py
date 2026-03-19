"""Shared utilities for openMax / openSeed / openBench skills.

Extracted from individual run.py files to eliminate duplication.
"""

from __future__ import annotations

import os
import re
import shutil
import signal
import subprocess
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_TASK_NAME_RE = re.compile(r"^[a-zA-Z0-9_-]+$")

TASK_REPORT_TEMPLATE = """\

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

BRIEF_CONTEXT_TEMPLATE = """\

## Context (auto-injected by openMax — use only if relevant)

Working directory: {worktree_path}
You are already in the correct directory. Do NOT run `cd`.

Branch: openmax/{task} (isolated worktree — commit here, do not switch branches)
"""

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def repo_root() -> Path:
    try:
        out = subprocess.check_output(
            ["git", "rev-parse", "--show-toplevel"], text=True
        ).strip()
        return Path(out)
    except subprocess.CalledProcessError:
        return Path.cwd()


def validate_task_name(task: str) -> None:
    """Raise ValueError if task name contains path-traversal or unsafe chars."""
    if not task:
        raise ValueError("task name must not be empty")
    if not _TASK_NAME_RE.match(task):
        raise ValueError(
            f"task name {task!r} is invalid — only [a-zA-Z0-9_-] allowed"
        )


def worktree_exists(path: Path) -> bool:
    return path.exists() and (path / ".git").exists()


def create_worktree(
    root: Path,
    task: str,
    base_branch: str,
    worktree_base: Path,
) -> tuple[Path, bool]:
    """Create git worktree. Returns (path, created). created=False if already exists."""
    validate_task_name(task)
    worktree_path = worktree_base / f"openmax_{task}"
    branch = f"openmax/{task}"

    if worktree_exists(worktree_path):
        return worktree_path, False

    worktree_base.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["git", "worktree", "add", "-b", branch, str(worktree_path), base_branch],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )

    env_src = root / ".env"
    if env_src.is_file():
        shutil.copy(env_src, worktree_path / ".env")

    return worktree_path, True


def inject_claude_md(worktree_path: Path, task: str) -> None:
    """Append task-report template to CLAUDE.md in the worktree."""
    claude_md = worktree_path / "CLAUDE.md"
    if not claude_md.exists():
        return
    existing = claude_md.read_text(encoding="utf-8")
    marker = f"# openMax Task: {task}"
    if marker in existing:
        return
    claude_md.write_text(
        existing + "\n" + TASK_REPORT_TEMPLATE.format(task=task),
        encoding="utf-8",
    )


def inject_brief_context(brief_path: Path, task: str, worktree_path: Path) -> None:
    """Append context block to brief file if not already present."""
    if not brief_path.exists():
        return
    existing = brief_path.read_text(encoding="utf-8")
    marker = "## Context (auto-injected by openMax"
    if marker in existing:
        return
    brief_path.write_text(
        existing
        + BRIEF_CONTEXT_TEMPLATE.format(task=task, worktree_path=worktree_path),
        encoding="utf-8",
    )


# ---------------------------------------------------------------------------
# PID tracking
# ---------------------------------------------------------------------------


def pid_file(pid_dir: Path, task: str) -> Path:
    return pid_dir / f"{task}.pid"


def write_pid(pid_dir: Path, task: str, pid: int) -> None:
    pid_dir.mkdir(parents=True, exist_ok=True)
    pid_file(pid_dir, task).write_text(str(pid), encoding="utf-8")


def read_pid(pid_dir: Path, task: str) -> int | None:
    p = pid_file(pid_dir, task)
    if not p.exists():
        return None
    try:
        return int(p.read_text(encoding="utf-8").strip())
    except (ValueError, OSError):
        return None


def is_pid_running(pid: int) -> bool:
    """Return True if the process with given PID is still alive."""
    try:
        os.kill(pid, 0)
        return True
    except (ProcessLookupError, PermissionError):
        return False


def worker_state(
    pid_dir: Path,
    report_dir: Path,
    task: str,
    worktree_path: Path,
) -> str:
    """Return worker state: running | done | failed | unknown."""
    report_path = report_dir / f"{task}.md"
    if report_path.exists():
        return "done"
    pid = read_pid(pid_dir, task)
    if pid is None:
        # No pid file and no report — worktree exists but no process tracked yet.
        return "unknown"
    return "running" if is_pid_running(pid) else "failed"


# ---------------------------------------------------------------------------
# Worker launch (brief via stdin — avoids brief content in ps aux)
# ---------------------------------------------------------------------------


def launch_worker(
    worktree_path: Path,
    brief_path: Path,
    pid_dir: Path,
    task: str,
    dry_run: bool,
) -> int | None:
    """Launch claude in background. Returns PID or None on dry-run.

    Brief content is fed via stdin (not CLI argv) to prevent the prompt from
    appearing in `ps aux` output.
    """
    if dry_run:
        return None

    validate_task_name(task)
    log_path = worktree_path / ".openmax_worker.log"

    try:
        with open(log_path, "w", encoding="utf-8") as log_file, \
             open(brief_path, "r", encoding="utf-8") as brief_stdin:
            proc = subprocess.Popen(
                ["claude", "--dangerously-skip-permissions", "--print"],
                cwd=worktree_path,
                stdin=brief_stdin,
                stdout=log_file,
                stderr=subprocess.STDOUT,
                start_new_session=True,
            )
    except FileNotFoundError as exc:
        raise RuntimeError(f"claude not found in PATH: {exc}") from exc
    except OSError as exc:
        raise RuntimeError(f"Failed to launch worker: {exc}") from exc

    write_pid(pid_dir, task, proc.pid)
    return proc.pid


# ---------------------------------------------------------------------------
# Shared main() runner
# ---------------------------------------------------------------------------


def run_skill_main(run_fn) -> None:
    """Standard entry point for openMax family skills."""
    # Import here to avoid circular dependency if openmax_utils is imported early.
    from skill_runner.cli_contract import parse_cli_args, render_result  # noqa: PLC0415

    args = parse_cli_args(sys.argv[1:])
    result = run_fn(args)
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
