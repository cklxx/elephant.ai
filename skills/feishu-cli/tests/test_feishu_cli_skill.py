"""Tests for feishu-cli skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("feishu_cli_skill_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_help_default():
    with patch.object(_mod, "feishu_help", return_value={"success": True, "help_level": "overview"}) as mock:
        result = _mod.run({})
        mock.assert_called_once()
    assert result["success"] is True


def test_tool_requires_module_and_action():
    result = _mod.run({"action": "tool", "module": "calendar"})
    assert result["success"] is False


def test_tool_dispatch():
    with patch.object(_mod, "feishu_tool", return_value={"success": True}) as mock:
        result = _mod.run({"action": "tool", "module": "calendar", "tool_action": "query", "start": "2026-03-06"})
        mock.assert_called_once_with("calendar", "query", {"start": "2026-03-06"})
    assert result["success"] is True


def test_command_alias_for_tool():
    with patch.object(_mod, "feishu_tool", return_value={"success": True}) as mock:
        result = _mod.run({"command": "tool", "module": "calendar", "action": "query", "start": "2026-03-06"})
        mock.assert_called_once_with("calendar", "query", {"start": "2026-03-06"})
    assert result["success"] is True


def test_non_object_args_rejected():
    result = _mod.run([])
    assert result["success"] is False
    assert "object" in result["error"]
