"""Tests for background-tasks skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("background_tasks_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

dispatch = _mod.dispatch
list_tasks = _mod.list_tasks
collect = _mod.collect
run = _mod.run


@pytest.fixture(autouse=True)
def _bg_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_BG_DIR", tmp_path)


class TestDispatch:
    def test_missing_command(self):
        result = dispatch({})
        assert result["success"] is False

    def test_launches_process(self, tmp_path):
        mock_proc = MagicMock()
        mock_proc.pid = 12345
        with patch("subprocess.Popen", return_value=mock_proc):
            result = dispatch({"command": "echo hello"})
            assert result["success"] is True
            assert result["pid"] == 12345
            # Meta file created
            json_files = list(tmp_path.glob("*.json"))
            assert len(json_files) == 1


class TestListTasks:
    def test_empty(self):
        result = list_tasks({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_tasks(self, tmp_path):
        meta = {"id": "abc", "command": "test", "pid": 99999, "status": "running"}
        (tmp_path / "abc.json").write_text(json.dumps(meta))
        # PID 99999 won't exist, so status should update to completed
        result = list_tasks({})
        assert result["count"] == 1
        assert result["tasks"][0]["status"] == "completed"


class TestCollect:
    def test_missing_task_id(self):
        result = collect({})
        assert result["success"] is False

    def test_not_found(self):
        result = collect({"task_id": "nonexistent"})
        assert result["success"] is False

    def test_reads_output(self, tmp_path):
        meta = {"id": "abc", "command": "test", "status": "completed"}
        (tmp_path / "abc.json").write_text(json.dumps(meta))
        (tmp_path / "abc.out").write_text("task output here")
        result = collect({"task_id": "abc"})
        assert result["success"] is True
        assert "task output here" in result["output"]


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
