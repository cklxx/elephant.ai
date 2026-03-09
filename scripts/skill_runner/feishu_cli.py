"""Shared adapter to call the unified Feishu CLI runtime from skills."""

from __future__ import annotations

from typing import Any

from cli.feishu.feishu_cli import execute


def feishu_help(topic: str = "overview", *, module: str = "", action_name: str = "") -> dict[str, Any]:
    request: dict[str, Any] = {"command": "help", "topic": topic}
    if module:
        request["module"] = module
    if action_name:
        request["action_name"] = action_name
    return execute(request)


def feishu_auth(subcommand: str, args: dict[str, Any] | None = None) -> dict[str, Any]:
    return execute({"command": "auth", "subcommand": subcommand, "args": args or {}})


def feishu_tool(module: str, action: str, args: dict[str, Any] | None = None) -> dict[str, Any]:
    return execute({"command": "tool", "module": module, "tool_action": action, "args": args or {}})


def feishu_api(
    method: str,
    path: str,
    *,
    body: dict[str, Any] | None = None,
    query: dict[str, Any] | str | None = None,
    auth: str = "tenant",
    user_key: str = "",
    retry_on_auth_error: bool = True,
) -> dict[str, Any]:
    return execute(
        {
            "command": "api",
            "method": method,
            "path": path,
            "body": body,
            "query": query,
            "auth": auth,
            "user_key": user_key,
            "retry_on_auth_error": retry_on_auth_error,
        }
    )
