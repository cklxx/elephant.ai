#!/usr/bin/env python3
"""feishu-cli skill — unified local Feishu CLI runtime for agent use."""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json

from skill_runner.feishu_cli import feishu_api, feishu_auth, feishu_help, feishu_tool


def run(args: dict) -> dict:
    action = str(args.pop("action", "help"))

    if action == "help":
        return feishu_help(
            topic=str(args.get("topic", "overview")),
            module=str(args.get("module", "")),
            action_name=str(args.get("action_name", "")),
        )

    if action == "auth":
        subcommand = str(args.pop("subcommand", "status"))
        return feishu_auth(subcommand, args)

    if action == "tool":
        module = str(args.pop("module", "")).strip()
        tool_action = str(args.pop("tool_action", "")).strip()
        if not module or not tool_action:
            return {"success": False, "error": "module and tool_action are required"}
        return feishu_tool(module, tool_action, args)

    if action == "api":
        method = str(args.pop("method", "")).strip()
        path = str(args.pop("path", "")).strip()
        if not method or not path:
            return {"success": False, "error": "method and path are required"}
        body = args.pop("body", None)
        query = args.pop("query", None)
        auth = str(args.pop("auth", "tenant")).strip() or "tenant"
        user_key = str(args.pop("user_key", "")).strip()
        retry_on_auth_error = bool(args.pop("retry_on_auth_error", True))
        return feishu_api(
            method,
            path,
            body=body,
            query=query,
            auth=auth,
            user_key=user_key,
            retry_on_auth_error=retry_on_auth_error,
        )

    return {
        "success": False,
        "error": f"unknown action: {action}, valid: ['help','auth','tool','api']",
    }


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
