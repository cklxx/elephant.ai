"""Tests for feishu-cli skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("feishu_cli_skill_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


# ---------------------------------------------------------------------------
# module import & smoke
# ---------------------------------------------------------------------------

def test_module_has_run_and_main():
    """run.py exposes both run() and main() entry points."""
    assert callable(getattr(_mod, "run", None))
    assert callable(getattr(_mod, "main", None))


# ---------------------------------------------------------------------------
# help
# ---------------------------------------------------------------------------

def test_help_default():
    with patch.object(_mod, "feishu_help", return_value={"success": True, "help_level": "overview"}) as mock:
        result = _mod.run({})
        mock.assert_called_once()
    assert result["success"] is True


def test_help_with_topic():
    with patch.object(_mod, "feishu_help", return_value={"success": True}) as mock:
        result = _mod.run({"action": "help", "topic": "modules", "module": "calendar"})
        mock.assert_called_once_with(topic="modules", module="calendar", action_name="")
    assert result["success"] is True


# ---------------------------------------------------------------------------
# auth
# ---------------------------------------------------------------------------

def test_auth_dispatch():
    with patch.object(_mod, "feishu_auth", return_value={"success": True, "status": "ok"}) as mock:
        result = _mod.run({"action": "auth", "subcommand": "status"})
        mock.assert_called_once_with("status", {})
    assert result["success"] is True


def test_auth_default_subcommand():
    with patch.object(_mod, "feishu_auth", return_value={"success": True}) as mock:
        result = _mod.run({"action": "auth"})
        mock.assert_called_once_with("status", {})
    assert result["success"] is True


# ---------------------------------------------------------------------------
# tool
# ---------------------------------------------------------------------------

def test_tool_requires_module_and_action():
    result = _mod.run({"action": "tool", "module": "calendar"})
    assert result["success"] is False


def test_tool_requires_module():
    result = _mod.run({"action": "tool", "tool_action": "query"})
    assert result["success"] is False
    assert "module" in result["error"]


def test_tool_dispatch():
    with patch.object(_mod, "feishu_tool", return_value={"success": True}) as mock:
        result = _mod.run({"action": "tool", "module": "calendar", "tool_action": "query", "start": "2026-03-06"})
        mock.assert_called_once_with("calendar", "query", {"start": "2026-03-06"})
    assert result["success"] is True


# ---------------------------------------------------------------------------
# api
# ---------------------------------------------------------------------------

def test_api_requires_method_and_path():
    result = _mod.run({"action": "api", "method": "GET"})
    assert result["success"] is False
    assert "path" in result["error"]


def test_api_dispatch():
    with patch.object(_mod, "feishu_api", return_value={"success": True, "data": {}}) as mock:
        result = _mod.run({
            "action": "api",
            "method": "GET",
            "path": "/open-apis/contact/v3/users",
        })
        mock.assert_called_once_with(
            "GET",
            "/open-apis/contact/v3/users",
            body=None,
            query=None,
            auth="tenant",
            user_key="",
            retry_on_auth_error=True,
        )
    assert result["success"] is True


# ---------------------------------------------------------------------------
# unknown action
# ---------------------------------------------------------------------------

def test_unknown_action():
    result = _mod.run({"action": "nope"})
    assert result["success"] is False
    assert "unknown" in result["error"].lower()


# ---------------------------------------------------------------------------
# main() CLI entry-point
# ---------------------------------------------------------------------------

def test_main_exit_code_zero(monkeypatch, capsys):
    """main() exits 0 when run() returns success."""
    monkeypatch.setattr("sys.argv", ["run.py", '{}'])
    with patch.object(_mod, "feishu_help", return_value={"success": True}):
        try:
            _mod.main()
        except SystemExit as exc:
            assert exc.code == 0
