"""Tests for unified AnyGen CLI runtime."""

from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent.parent))

from cli.anygen.anygen_cli import execute


def test_help_overview_returns_progressive_next_steps():
    result = execute({"command": "help"})
    assert result["success"] is True
    assert result["topic"] == "overview"
    assert len(result["next"]) >= 2


def test_help_module_requires_valid_module():
    result = execute({"command": "help", "topic": "module", "module": "unknown"})
    assert result["success"] is False
    assert "valid_modules" in result


def test_task_rejects_guide_only_module_execution():
    result = execute(
        {
            "command": "task",
            "module": "finance-report",
            "task_action": "create",
            "api_key": "sk-test",
            "operation": "slide",
            "prompt": "q2 review",
        }
    )
    assert result["success"] is False
    assert "guide-only" in result["error"]


def test_task_create_dispatches_to_create_handler():
    with patch("cli.anygen.anygen_cli._create_task", return_value={"success": True, "task_id": "task_1"}) as mock:
        result = execute(
            {
                "command": "task",
                "task_action": "create",
                "api_key": "sk-test",
                "operation": "slide",
                "prompt": "product roadmap",
            }
        )
    assert result["success"] is True
    mock.assert_called_once()
    assert mock.call_args.args[0] == "sk-test"
    assert mock.call_args.args[1]["operation"] == "slide"


def test_action_alias_can_infer_task_command():
    with patch("cli.anygen.anygen_cli._create_task", return_value={"success": True, "task_id": "task_2"}) as mock:
        result = execute(
            {
                "action": "create",
                "api_key": "sk-test",
                "operation": "doc",
                "prompt": "technical design doc",
            }
        )
    assert result["success"] is True
    mock.assert_called_once()


def test_status_dispatches_to_status_handler():
    with patch("cli.anygen.anygen_cli._status_task", return_value={"success": True, "status": "processing"}) as mock:
        result = execute({"action": "status", "api_key": "sk-test", "task_id": "task_3"})
    assert result["success"] is True
    mock.assert_called_once_with("sk-test", "task_3", {})
