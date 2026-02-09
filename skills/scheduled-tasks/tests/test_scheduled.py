"""Tests for scheduled-tasks skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("scheduled_tasks_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create = _mod.create
list_jobs = _mod.list_jobs
delete = _mod.delete
run = _mod.run


@pytest.fixture(autouse=True)
def _jobs_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_JOBS_DIR", tmp_path)


class TestCreate:
    def test_missing_name(self):
        result = create({"cron": "0 9 * * *"})
        assert result["success"] is False

    def test_missing_cron(self):
        result = create({"name": "test"})
        assert result["success"] is False

    def test_creates_job(self):
        result = create({"name": "daily", "cron": "0 9 * * *", "command": "echo hi"})
        assert result["success"] is True
        assert result["job"]["name"] == "daily"
        assert result["job"]["cron"] == "0 9 * * *"

    def test_duplicate_name(self):
        create({"name": "daily", "cron": "0 9 * * *"})
        result = create({"name": "daily", "cron": "0 10 * * *"})
        assert result["success"] is False
        assert "already exists" in result["error"]


class TestListJobs:
    def test_empty(self):
        result = list_jobs({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_created(self):
        create({"name": "a", "cron": "* * * * *"})
        create({"name": "b", "cron": "* * * * *"})
        result = list_jobs({})
        assert result["count"] == 2


class TestDelete:
    def test_missing_identifier(self):
        result = delete({})
        assert result["success"] is False

    def test_not_found(self):
        result = delete({"name": "nonexistent"})
        assert result["success"] is False

    def test_deletes_by_name(self):
        create({"name": "daily", "cron": "0 9 * * *"})
        result = delete({"name": "daily"})
        assert result["success"] is True
        assert list_jobs({})["count"] == 0


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
