"""Tests for autonomous-scheduler skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("autonomous_scheduler_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

upsert = _mod.upsert
list_jobs = _mod.list_jobs
delete = _mod.delete
due = _mod.due
touch_run = _mod.touch_run
run = _mod.run


@pytest.fixture(autouse=True)
def _store(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_STORE_PATH", tmp_path / "jobs.json")


class TestUpsertAndList:
    def test_create_and_update(self):
        created = upsert({"name": "weekly", "schedule": "0 18 * * 5", "task": "retro"})
        assert created["success"] is True
        assert created["action"] == "created"

        updated = upsert({"name": "weekly", "schedule": "0 19 * * 5", "task": "retro-v2"})
        assert updated["success"] is True
        assert updated["action"] == "updated"

        listed = list_jobs({})
        assert listed["count"] == 1
        assert listed["jobs"][0]["schedule"] == "0 19 * * 5"


class TestDueAndDelete:
    def test_due_and_delete(self):
        created = upsert(
            {
                "name": "daily",
                "schedule": "0 9 * * *",
                "task": "brief",
                "next_run_at": "2026-02-13T09:00:00Z",
            }
        )
        assert created["success"] is True

        due_result = due({"now": "2026-02-13T10:00:00Z"})
        assert due_result["success"] is True
        assert due_result["count"] == 1

        touched = touch_run({"name": "daily", "next_run_at": "2026-02-14T09:00:00Z"})
        assert touched["success"] is True
        assert touched["job"]["next_run_at"] == "2026-02-14T09:00:00Z"

        deleted = delete({"name": "daily"})
        assert deleted["success"] is True
        assert deleted["removed"] == 1

    def test_due_accepts_naive_iso_timestamp(self):
        upsert(
            {
                "name": "daily-naive",
                "schedule": "0 9 * * *",
                "task": "brief",
                "next_run_at": "2026-02-13T09:00:00",
            }
        )
        due_result = due({"now": "2026-02-13T10:00:00"})
        assert due_result["success"] is True
        assert due_result["count"] == 1


class TestRun:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
