"""Tests for desktop-automation skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("desktop_automation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

run_script = _mod.run_script
open_app = _mod.open_app
run = _mod.run


class TestRunScript:
    def test_missing_script(self):
        result = run_script({})
        assert result["success"] is False

    def test_osascript_not_found(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = run_script({"script": "tell app \"Finder\" to activate"})
            assert result["success"] is False
            assert "macOS" in result["error"]

    def test_timeout(self):
        import subprocess
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("osascript", 30)):
            result = run_script({"script": "delay 60"})
            assert result["success"] is False
            assert "timeout" in result["error"]

    def test_success(self):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "Finder"
        with patch("subprocess.run", return_value=mock_result):
            result = run_script({"script": "tell app \"Finder\" to name"})
            assert result["success"] is True
            assert result["output"] == "Finder"

    def test_failure(self):
        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stderr = "execution error"
        with patch("subprocess.run", return_value=mock_result):
            result = run_script({"script": "invalid"})
            assert result["success"] is False


class TestOpenApp:
    def test_missing_app(self):
        result = open_app({})
        assert result["success"] is False

    def test_opens_app(self):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = ""
        with patch("subprocess.run", return_value=mock_result) as mock_run:
            result = open_app({"app": "Safari"})
            assert result["success"] is True
            cmd = mock_run.call_args[0][0]
            assert "Safari" in cmd[2]  # osascript -e "..."


class TestRun:
    def test_default_action_is_run(self):
        result = run({})
        assert result["success"] is False  # missing script

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
