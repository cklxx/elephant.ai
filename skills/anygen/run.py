#!/usr/bin/env python3
"""anygen skill — unified AnyGen wrapper with progressive CLI help."""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)


from skill_runner.anygen_cli import anygen_help, anygen_task

COMMAND_CHOICES = ("help", "task")
TASK_ACTIONS = ("create", "status", "poll", "download", "run")


def run(args: dict) -> dict:
    if not isinstance(args, dict):
        return {"success": False, "error": "args must be an object"}

    payload = dict(args)
    command = str(payload.pop("command", "")).strip().lower()
    action = str(payload.pop("action", "")).strip().lower()

    if not command:
        if action in COMMAND_CHOICES:
            command = action
        elif action in TASK_ACTIONS or payload.get("task_action") or payload.get("operation") or payload.get("task_id"):
            command = "task"
        else:
            command = "help"

    if command == "help":
        action_name = str(payload.get("action_name", "") or payload.get("task_action", "") or "").strip()
        if not action_name and action in TASK_ACTIONS:
            action_name = action
        return anygen_help(
            topic=str(payload.get("topic", "overview")),
            module=str(payload.get("module", "")),
            action_name=action_name,
        )

    if command == "task":
        module = str(payload.pop("module", "task-manager")).strip() or "task-manager"
        task_action = str(payload.pop("task_action", "") or payload.pop("action_name", "") or "").strip().lower()
        if not task_action and action in TASK_ACTIONS:
            task_action = action
        if not task_action:
            return {"success": False, "error": "task_action is required"}
        return anygen_task(task_action, payload, module=module)

    return {
        "success": False,
        "error": f"unknown command: {command}, valid: {list(COMMAND_CHOICES)}",
    }


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
