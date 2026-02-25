"""Tests for timer-management skill (integration via CLI)."""

from __future__ import annotations

import importlib.util
import io
import json
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

# The timer-management skill delegates to scripts/cli/timer/timer_cli.py
# which is already tested. Here we test the skill's action routing.

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS_DIR))

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("timer_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


class TestMain:
    def test_set_action(self):
        mock_result = {"success": True, "timer_id": "abc"}
        with patch.object(_mod, "set_timer", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "set", "delay": "5m", "message": "test"})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()

    def test_list_action(self):
        mock_result = {"success": True, "timers": []}
        with patch.object(_mod, "list_timers", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "list"})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()

    def test_cancel_action(self):
        mock_result = {"success": True, "message": "cancelled"}
        with patch.object(_mod, "cancel_timer", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "cancel", "timer_id": "abc"})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()

    def test_unknown_action(self):
        with patch("sys.argv", ["run.py", json.dumps({"action": "invalid"})]):
            with patch("sys.stdout", new=io.StringIO()):
                with pytest.raises(SystemExit) as exc:
                    _mod.main()
                assert exc.value.code == 1

    def test_default_action_is_list(self):
        mock_result = {"success": True, "timers": []}
        with patch.object(_mod, "list_timers", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()
