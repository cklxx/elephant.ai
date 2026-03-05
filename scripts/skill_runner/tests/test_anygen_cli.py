"""Tests for skill_runner.anygen_cli adapter."""

from __future__ import annotations

from unittest.mock import patch

from skill_runner import anygen_cli


def test_anygen_help_dispatches_execute():
    with patch.object(anygen_cli, "execute", return_value={"success": True}) as mock:
        result = anygen_cli.anygen_help("modules")
        mock.assert_called_once_with({"command": "help", "topic": "modules"})
    assert result["success"] is True


def test_anygen_task_dispatches_execute():
    with patch.object(anygen_cli, "execute", return_value={"success": True}) as mock:
        result = anygen_cli.anygen_task("create", {"operation": "slide", "prompt": "Roadmap"})
        mock.assert_called_once_with(
            {
                "command": "task",
                "module": "task-manager",
                "task_action": "create",
                "operation": "slide",
                "prompt": "Roadmap",
            }
        )
    assert result["success"] is True
