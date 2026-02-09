"""Tests for self-test skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("self_test_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

execute = _mod.execute
run = _mod.run


def _go_test_output(events: list[dict]) -> str:
    return "\n".join(json.dumps(e) for e in events)


class TestExecute:
    def test_timeout(self):
        import subprocess
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("go", 180)):
            result = execute({})
            assert result["success"] is False
            assert "timeout" in result["error"]

    def test_go_not_found(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = execute({})
            assert result["success"] is False
            assert "go not found" in result["error"]

    def test_all_pass(self):
        events = [
            {"Action": "pass", "Test": "TestA", "Package": "pkg"},
            {"Action": "pass", "Test": "TestB", "Package": "pkg"},
        ]
        mock_result = MagicMock()
        mock_result.stdout = _go_test_output(events)
        mock_result.returncode = 0
        with patch("subprocess.run", return_value=mock_result):
            result = execute({})
            assert result["success"] is True
            assert result["summary"]["passed"] == 2
            assert result["summary"]["failed"] == 0

    def test_with_failures(self):
        events = [
            {"Action": "pass", "Test": "TestA", "Package": "pkg"},
            {"Action": "fail", "Test": "TestB", "Package": "pkg", "Elapsed": 0.5},
            {"Action": "skip", "Test": "TestC", "Package": "pkg"},
        ]
        mock_result = MagicMock()
        mock_result.stdout = _go_test_output(events)
        mock_result.returncode = 1
        with patch("subprocess.run", return_value=mock_result):
            result = execute({})
            assert result["success"] is True
            assert result["summary"]["passed"] == 1
            assert result["summary"]["failed"] == 1
            assert result["summary"]["skipped"] == 1
            assert result["failed"][0]["test"] == "TestB"
            assert "analysis_prompt" in result

    def test_custom_package(self):
        mock_result = MagicMock()
        mock_result.stdout = ""
        mock_result.returncode = 0
        with patch("subprocess.run", return_value=mock_result) as mock_run:
            execute({"package": "./my/pkg/..."})
            cmd = mock_run.call_args[0][0]
            assert "./my/pkg/..." in cmd

    def test_invalid_json_lines_skipped(self):
        mock_result = MagicMock()
        mock_result.stdout = "not json\n" + json.dumps({"Action": "pass", "Test": "T1"})
        mock_result.returncode = 0
        with patch("subprocess.run", return_value=mock_result):
            result = execute({})
            assert result["success"] is True
            assert result["summary"]["passed"] == 1


class TestRun:
    def test_default_action_is_execute(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = run({})
            assert result["success"] is False

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
