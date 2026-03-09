#!/usr/bin/env python3
"""contact-lookup skill — 飞书通讯录查询。

All actions delegate to unified Feishu CLI runtime.
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

from skill_runner.feishu_cli import feishu_tool


def list_scopes(args: dict) -> dict:
    return feishu_tool("contact", "list_scopes", args)


def get_user(args: dict) -> dict:
    return feishu_tool("contact", "get_user", args)


def list_users(args: dict) -> dict:
    return feishu_tool("contact", "list_users", args)


def get_department(args: dict) -> dict:
    return feishu_tool("contact", "get_department", args)


def list_departments(args: dict) -> dict:
    return feishu_tool("contact", "list_departments", args)


def run(args: dict) -> dict:
    action = args.pop("action", "get_user")
    handlers = {
        "get_user": get_user,
        "list_users": list_users,
        "get_department": get_department,
        "list_departments": list_departments,
        "list_scopes": list_scopes,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}, valid: {list(handlers)}"}
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
    sys.exit(0 if result.get("success", False) else 1)


if __name__ == "__main__":
    main()
