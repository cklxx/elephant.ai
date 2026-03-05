"""Tests for reminder-scheduler skill."""

from __future__ import annotations

import importlib.util
import io
import json
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS_DIR))

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("reminder_scheduler_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


@pytest.fixture(autouse=True)
def _plan_store(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_PLAN_STORE_PATH", tmp_path / "plans.json")


class TestMainRouting:
    def test_set_once_action(self):
        mock_result = {"success": True, "timer_id": "abc"}
        with patch.object(_mod, "set_timer", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "set_once", "delay": "5m", "task": "test"})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()

    def test_list_once_action(self):
        mock_result = {"success": True, "timers": []}
        with patch.object(_mod, "list_timers", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "list_once"})]):
                with patch("sys.stdout", new=io.StringIO()):
                    with pytest.raises(SystemExit) as exc:
                        _mod.main()
                    assert exc.value.code == 0
                    mock.assert_called_once()

    def test_cancel_once_action(self):
        mock_result = {"success": True, "message": "cancelled"}
        with patch.object(_mod, "cancel_timer", return_value=mock_result) as mock:
            with patch("sys.argv", ["run.py", json.dumps({"action": "cancel_once", "id": "timer-1"})]):
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


class TestPlanLifecycle:
    def test_upsert_due_touch_delete_flow(self):
        created = _mod.upsert_plan(
            {
                "name": "weekly",
                "schedule": "0 18 * * 5",
                "task": "retro",
                "next_run_at": "2026-03-05T10:00:00Z",
            }
        )
        assert created["success"] is True
        assert created["action"] == "created"

        updated = _mod.upsert_plan(
            {
                "name": "weekly",
                "schedule": "0 19 * * 5",
                "task": "retro-v2",
                "next_run_at": "2026-03-05T10:00:00Z",
            }
        )
        assert updated["success"] is True
        assert updated["action"] == "updated"

        listed = _mod.list_plans({})
        assert listed["success"] is True
        assert listed["count"] == 1
        assert listed["plans"][0]["schedule"] == "0 19 * * 5"

        due = _mod.due_plans({"now": "2026-03-05T10:01:00Z"})
        assert due["success"] is True
        assert due["count"] == 1

        touched = _mod.touch_plan({"name": "weekly", "next_run_at": "2026-03-12T10:00:00Z"})
        assert touched["success"] is True
        assert touched["plan"]["next_run_at"] == "2026-03-12T10:00:00Z"

        deleted = _mod.delete_plan({"name": "weekly"})
        assert deleted["success"] is True
        assert deleted["removed"] == 1

    def test_due_accepts_naive_iso_timestamp(self):
        _mod.upsert_plan(
            {
                "name": "daily-naive",
                "schedule": "0 9 * * *",
                "task": "brief",
                "next_run_at": "2026-03-05T09:00:00",
            }
        )
        due = _mod.due_plans({"now": "2026-03-05T10:00:00"})
        assert due["success"] is True
        assert due["count"] == 1
