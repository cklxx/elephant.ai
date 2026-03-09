#!/usr/bin/env python3
"""wiki-knowledge skill — 飞书知识库管理。

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


def list_spaces(args: dict) -> dict:
    return feishu_tool("wiki", "list_spaces", args)


def list_nodes(args: dict) -> dict:
    return feishu_tool("wiki", "list_nodes", args)


def create_node(args: dict) -> dict:
    return feishu_tool("wiki", "create_node", args)


def get_node(args: dict) -> dict:
    return feishu_tool("wiki", "get_node", args)


def run(args: dict) -> dict:
    action = args.pop("action", "list_spaces")

    handlers = {
        "list_spaces": list_spaces,
        "list_nodes": list_nodes,
        "create_node": create_node,
        "get_node": get_node,
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
