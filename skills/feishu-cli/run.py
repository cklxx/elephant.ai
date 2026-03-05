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

COMMAND_CHOICES = ("help", "auth", "tool", "api")


def run(args: dict) -> dict:
    if not isinstance(args, dict):
        return {"success": False, "error": "args must be an object"}

    payload = dict(args)
    command = str(payload.pop("command", "")).strip().lower()
    action = str(payload.pop("action", "")).strip()

    if not command:
        normalized_action = action.lower()
        if normalized_action in COMMAND_CHOICES:
            command = normalized_action
        elif str(payload.get("module", "")).strip():
            command = "tool"
        elif str(payload.get("subcommand", "")).strip():
            command = "auth"
        elif str(payload.get("method", "")).strip() and str(payload.get("path", "")).strip():
            command = "api"
        else:
            command = "help"

    if command == "help":
        action_name = str(payload.get("action_name", "")).strip() or str(payload.get("tool_action", "")).strip()
        if not action_name and action.lower() not in COMMAND_CHOICES:
            action_name = action
        return feishu_help(
            topic=str(payload.get("topic", "overview")),
            module=str(payload.get("module", "")),
            action_name=action_name,
        )

    if command == "auth":
        subcommand = str(payload.pop("subcommand", "status"))
        return feishu_auth(subcommand, payload)

    if command == "tool":
        module = str(payload.pop("module", "")).strip()
        tool_action = str(payload.pop("tool_action", "")).strip() or str(payload.pop("action_name", "")).strip()
        if not tool_action and action.lower() not in COMMAND_CHOICES:
            tool_action = action.strip()
        if not module or not tool_action:
            return {"success": False, "error": "module and tool_action are required"}
        return feishu_tool(module, tool_action, payload)

    if command == "api":
        method = str(payload.pop("method", "")).strip()
        path = str(payload.pop("path", "")).strip()
        if not method or not path:
            return {"success": False, "error": "method and path are required"}
        body = payload.pop("body", None)
        query = payload.pop("query", None)
        auth = str(payload.pop("auth", "tenant")).strip() or "tenant"
        user_key = str(payload.pop("user_key", "")).strip()
        retry_on_auth_error = bool(payload.pop("retry_on_auth_error", True))
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
        "error": f"unknown command: {command}, valid: {list(COMMAND_CHOICES)}",
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
