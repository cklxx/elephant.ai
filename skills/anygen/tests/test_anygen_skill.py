"""Tests for anygen skill wrapper."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("anygen_skill_run", _RUN_PATH)
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
    with patch.object(_mod, "anygen_help", return_value={"success": True, "topic": "overview"}) as mock:
        result = _mod.run({})
        mock.assert_called_once()
    assert result["success"] is True


def test_help_explicit_command():
    with patch.object(_mod, "anygen_help", return_value={"success": True}) as mock:
        result = _mod.run({"command": "help", "topic": "task"})
        mock.assert_called_once()
    assert result["success"] is True


# ---------------------------------------------------------------------------
# task dispatch
# ---------------------------------------------------------------------------

def test_task_dispatch():
    with patch.object(_mod, "anygen_task", return_value={"success": True}) as mock:
        result = _mod.run(
            {
                "action": "task",
                "task_action": "create",
                "operation": "slide",
                "prompt": "Q2 roadmap",
            }
        )
        mock.assert_called_once_with(
            "create",
            {"operation": "slide", "prompt": "Q2 roadmap"},
            module="task-manager",
        )
    assert result["success"] is True


def test_task_action_alias():
    with patch.object(_mod, "anygen_task", return_value={"success": True}) as mock:
        result = _mod.run({"action": "create", "operation": "doc", "prompt": "design"})
        mock.assert_called_once_with(
            "create",
            {"operation": "doc", "prompt": "design"},
            module="task-manager",
        )
    assert result["success"] is True


def test_task_action_from_positionals():
    with patch.object(_mod, "anygen_task", return_value={"success": True}) as mock:
        result = _mod.run(
            {
                "action": "task",
                "positionals": ["create"],
                "operation": "doc",
                "prompt": "design",
            }
        )
        mock.assert_called_once_with(
            "create",
            {"operation": "doc", "prompt": "design"},
            module="task-manager",
        )
    assert result["success"] is True


def test_task_missing_action_returns_error():
    """task command without task_action, positionals, or action alias → error."""
    result = _mod.run({"command": "task"})
    assert result["success"] is False
    assert "task_action" in result["error"]


def test_task_custom_module():
    with patch.object(_mod, "anygen_task", return_value={"success": True}) as mock:
        result = _mod.run(
            {"action": "task", "task_action": "status", "module": "ppt", "task_id": "abc123"}
        )
        mock.assert_called_once_with(
            "status",
            {"task_id": "abc123"},
            module="ppt",
        )
    assert result["success"] is True


# ---------------------------------------------------------------------------
# validation & edge cases
# ---------------------------------------------------------------------------

def test_non_object_args_rejected():
    result = _mod.run([])
    assert result["success"] is False
    assert "object" in result["error"]


def test_unknown_command():
    result = _mod.run({"command": "nope"})
    assert result["success"] is False
    assert "unknown" in result["error"].lower()


# ---------------------------------------------------------------------------
# main() CLI entry-point
# ---------------------------------------------------------------------------

def test_main_exit_code_zero(monkeypatch):
    """main() exits 0 when run() returns success (default = help)."""
    monkeypatch.setattr("sys.argv", ["run.py"])
    with patch.object(_mod, "anygen_help", return_value={"success": True}):
        try:
            _mod.main()
        except SystemExit as exc:
            assert exc.code == 0
