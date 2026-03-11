"""Tests for reminder-scheduler skill."""

from __future__ import annotations

import importlib.util
import io
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
        with (
            patch.object(_mod, "set_timer", return_value=mock_result) as mock,
            patch("sys.argv", ["run.py", "set_once", "--delay", "5m", "--task", "test"]),
            patch("sys.stdout", new=io.StringIO()),
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 0
            mock.assert_called_once()

    def test_list_once_action(self):
        mock_result = {"success": True, "timers": []}
        with (
            patch.object(_mod, "list_timers", return_value=mock_result) as mock,
            patch("sys.argv", ["run.py", "list_once"]),
            patch("sys.stdout", new=io.StringIO()) as stdout,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 0
            mock.assert_called_once()
            assert "timers:" in stdout.getvalue()

    def test_cancel_once_action(self):
        mock_result = {"success": True, "message": "cancelled"}
        with (
            patch.object(_mod, "cancel_timer", return_value=mock_result) as mock,
            patch("sys.argv", ["run.py", "cancel_once", "--id", "timer-1"]),
            patch("sys.stdout", new=io.StringIO()),
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 0
            mock.assert_called_once()

    def test_unknown_action(self):
        with (
            patch("sys.argv", ["run.py", "invalid"]),
            patch("sys.stdout", new=io.StringIO()),
            patch("sys.stderr", new=io.StringIO()) as stderr,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 1
            assert "unknown action: invalid" in stderr.getvalue()

    def test_empty_stdin_defaults_to_list_once(self):
        mock_result = {"success": True, "timers": []}
        with (
            patch.object(_mod, "list_timers", return_value=mock_result) as mock,
            patch("sys.argv", ["run.py"]),
            patch("sys.stdin", new=io.StringIO("")),
            patch("sys.stdout", new=io.StringIO()) as stdout,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 0
            mock.assert_called_once()
            assert "timers:" in stdout.getvalue()

    def test_malformed_stdin_returns_structured_error(self):
        with (
            patch("sys.argv", ["run.py"]),
            patch("sys.stdin", new=io.StringIO("{")),
            patch("sys.stdout", new=io.StringIO()),
            patch("sys.stderr", new=io.StringIO()) as stderr,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 1
            assert "invalid stdin JSON payload" in stderr.getvalue()

    def test_non_object_stdin_returns_structured_error(self):
        with (
            patch("sys.argv", ["run.py"]),
            patch("sys.stdin", new=io.StringIO('["oops"]')),
            patch("sys.stdout", new=io.StringIO()),
            patch("sys.stderr", new=io.StringIO()) as stderr,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 1
            assert "stdin JSON payload must be an object" in stderr.getvalue()

    def test_json_object_stdin_dispatches_action(self):
        mock_result = {"success": True, "count": 0, "plans": []}
        with (
            patch.object(_mod, "list_plans", return_value=mock_result) as mock,
            patch("sys.argv", ["run.py"]),
            patch("sys.stdin", new=io.StringIO('{"action":"list_plans"}')),
            patch("sys.stdout", new=io.StringIO()) as stdout,
        ):
            with pytest.raises(SystemExit) as exc:
                _mod.main()
            assert exc.value.code == 0
            mock.assert_called_once_with({})
            assert "count: 0" in stdout.getvalue()


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

    def test_delete_requires_same_record_when_name_and_id_are_both_provided(self):
        _mod.upsert_plan({"name": "plan-a", "schedule": "*", "task": "task-a"})
        created_b = _mod.upsert_plan({"name": "plan-b", "schedule": "*", "task": "task-b"})

        deleted = _mod.delete_plan({"name": "plan-a", "id": created_b["plan"]["id"]})

        assert deleted["success"] is False
        assert deleted["error"] == "plan not found"
        assert _mod.list_plans({})["count"] == 2

    def test_touch_requires_same_record_when_name_and_id_are_both_provided(self):
        _mod.upsert_plan({"name": "plan-a", "schedule": "*", "task": "task-a"})
        created_b = _mod.upsert_plan({"name": "plan-b", "schedule": "*", "task": "task-b"})

        touched = _mod.touch_plan(
            {
                "name": "plan-a",
                "id": created_b["plan"]["id"],
                "next_run_at": "2026-03-12T10:00:00Z",
            }
        )

        assert touched["success"] is False
        assert touched["error"] == "plan not found"
        plans = _mod.list_plans({})["plans"]
        assert plans[0]["next_run_at"] == ""
        assert plans[1]["next_run_at"] == ""
