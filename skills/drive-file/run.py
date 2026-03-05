#!/usr/bin/env python3
"""drive-file skill — 飞书云盘文件管理。

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


def list_files(args: dict) -> dict:
    return feishu_tool("drive", "list_files", args)


def create_folder(args: dict) -> dict:
    return feishu_tool("drive", "create_folder", args)


def copy_file(args: dict) -> dict:
    return feishu_tool("drive", "copy_file", args)


def delete_file(args: dict) -> dict:
    return feishu_tool("drive", "delete_file", args)


def run(args: dict) -> dict:
    action = args.pop("action", "list_files")

    handlers = {
        "list_files": list_files,
        "create_folder": create_folder,
        "copy_file": copy_file,
        "delete_file": delete_file,
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
