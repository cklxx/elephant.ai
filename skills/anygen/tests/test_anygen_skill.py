"""Tests for anygen skill wrapper."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("anygen_skill_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_help_default():
    with patch.object(_mod, "anygen_help", return_value={"success": True, "topic": "overview"}) as mock:
        result = _mod.run({})
        mock.assert_called_once()
    assert result["success"] is True


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


def test_non_object_args_rejected():
    result = _mod.run([])
    assert result["success"] is False
    assert "object" in result["error"]
