"""Tests for task-delegation skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("task_delegation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

dispatch = _mod.dispatch
list_tasks = _mod.list_tasks
run = _mod.run


@pytest.fixture(autouse=True)
def _tasks_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_TASKS_DIR", tmp_path)


class TestDispatch:
    def test_missing_task(self):
        result = dispatch({"agent": "codex"})
        assert result["success"] is False

    def test_unknown_agent(self):
        result = dispatch({"agent": "unknown", "task": "test"})
        assert result["success"] is False

    def test_creates_task(self, tmp_path):
        result = dispatch({"agent": "codex", "task": "fix the bug"})
        assert result["success"] is True
        assert result["agent"] == "codex"
        # Task file should exist
        files = list(tmp_path.glob("*.json"))
        assert len(files) == 1

    def test_claude_agent(self):
        result = dispatch({"agent": "claude", "task": "review code"})
        assert result["success"] is True
        assert result["agent"] == "claude"


class TestListTasks:
    def test_empty(self):
        result = list_tasks({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_dispatched(self):
        dispatch({"agent": "codex", "task": "task 1"})
        dispatch({"agent": "claude", "task": "task 2"})
        result = list_tasks({})
        assert result["count"] == 2


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
