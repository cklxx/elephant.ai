#!/usr/bin/env python3
"""desktop-automation skill — macOS 桌面自动化。

通过 osascript 执行 AppleScript 控制 macOS 应用。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

import subprocess


def run_script(args: dict) -> dict:
    script = args.get("script", "")
    if not script:
        return {"success": False, "error": "script is required"}

    try:
        result = subprocess.run(
            ["osascript", "-e", script],
            capture_output=True, text=True, timeout=30, check=False,
        )
        if result.returncode != 0:
            return {"success": False, "error": result.stderr.strip()}
        return {"success": True, "output": result.stdout.strip()}
    except FileNotFoundError:
        return {"success": False, "error": "osascript not found (requires macOS)"}
    except subprocess.TimeoutExpired:
        return {"success": False, "error": "script timeout (30s)"}


def open_app(args: dict) -> dict:
    app = args.get("app", "")
    if not app:
        return {"success": False, "error": "app is required"}
    return run_script({"script": f'tell application "{app}" to activate'})


def run(args: dict) -> dict:
    action = args.pop("action", "run")
    handlers = {"run": run_script, "open_app": open_app}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
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
