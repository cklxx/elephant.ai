#!/usr/bin/env python3
"""bitable-data skill — 飞书多维表格管理。

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


def list_tables(args: dict) -> dict:
    return feishu_tool("bitable", "list_tables", args)


def list_records(args: dict) -> dict:
    return feishu_tool("bitable", "list_records", args)


def create_record(args: dict) -> dict:
    return feishu_tool("bitable", "create_record", args)


def update_record(args: dict) -> dict:
    return feishu_tool("bitable", "update_record", args)


def delete_record(args: dict) -> dict:
    return feishu_tool("bitable", "delete_record", args)


def list_fields(args: dict) -> dict:
    return feishu_tool("bitable", "list_fields", args)


def run(args: dict) -> dict:
    action = args.pop("action", "list_tables")

    handlers = {
        "list_tables": list_tables,
        "list_records": list_records,
        "create_record": create_record,
        "update_record": update_record,
        "delete_record": delete_record,
        "list_fields": list_fields,
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
