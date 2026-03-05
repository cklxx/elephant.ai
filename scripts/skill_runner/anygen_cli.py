"""Shared adapter for the unified AnyGen CLI runtime."""

from __future__ import annotations

from typing import Any

from cli.anygen.anygen_cli import execute


def anygen_help(topic: str = "overview", *, module: str = "", action_name: str = "") -> dict[str, Any]:
    request: dict[str, Any] = {"command": "help", "topic": topic}
    if module:
        request["module"] = module
    if action_name:
        request["action_name"] = action_name
    return execute(request)


def anygen_task(action: str, args: dict[str, Any] | None = None, *, module: str = "task-manager") -> dict[str, Any]:
    payload = dict(args or {})
    payload.update({"command": "task", "module": module, "task_action": action})
    return execute(payload)
