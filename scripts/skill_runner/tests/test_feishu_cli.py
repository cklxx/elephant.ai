from __future__ import annotations

"""Tests for skill_runner.feishu_cli adapter."""

import pathlib as _pathlib
import sys as _sys

# Ensure scripts/ is on path so skill_runner and cli packages resolve
_sys.path.insert(0, str(_pathlib.Path(__file__).resolve().parent.parent.parent))

from unittest.mock import patch

from skill_runner import feishu_cli


def test_feishu_tool_dispatches_execute():
    with patch.object(feishu_cli, "execute", return_value={"success": True}) as mock:
        result = feishu_cli.feishu_tool("calendar", "query", {"start": "2026-03-06"})
        mock.assert_called_once_with(
            {
                "command": "tool",
                "module": "calendar",
                "tool_action": "query",
                "args": {"start": "2026-03-06"},
            }
        )
    assert result["success"] is True


def test_feishu_auth_dispatches_execute():
    with patch.object(feishu_cli, "execute", return_value={"success": True}) as mock:
        result = feishu_cli.feishu_auth("status", {})
        mock.assert_called_once_with({"command": "auth", "subcommand": "status", "args": {}})
    assert result["success"] is True
