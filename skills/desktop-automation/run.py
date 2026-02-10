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

load_repo_dotenv(__file__)

import json
import subprocess
import sys


def run_script(args: dict) -> dict:
    script = args.get("script", "")
    if not script:
        return {"success": False, "error": "script is required"}

    try:
        result = subprocess.run(
            ["osascript", "-e", script],
            capture_output=True, text=True, timeout=30,
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
