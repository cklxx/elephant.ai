"""Tests for okr-management skill."""

from __future__ import annotations

import importlib.util
import os
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("okr_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create = _mod.create
list_okrs = _mod.list_okrs
update = _mod.update
run = _mod.run


@pytest.fixture(autouse=True)
def _okr_dir(tmp_path, monkeypatch):
    """Redirect OKR storage to a temp directory."""
    monkeypatch.setattr(_mod, "_OKR_DIR", tmp_path)


class TestCreate:
    def test_missing_title(self):
        result = create({})
        assert result["success"] is False
        assert "title" in result["error"]

    def test_creates_file(self, tmp_path):
        result = create({"title": "Q1 Goal", "key_results": ["KR1", "KR2"]})
        assert result["success"] is True
        assert "q1-goal" in result["path"]
        path = Path(result["path"])
        assert path.exists()
        content = path.read_text()
        assert "Q1 Goal" in content
        assert "- [ ] KR1" in content
        assert "- [ ] KR2" in content

    def test_creates_with_defaults(self):
        result = create({"title": "Test"})
        assert result["success"] is True
        content = Path(result["path"]).read_text()
        assert "KR1: (define)" in content


class TestListOkrs:
    def test_empty_list(self):
        result = list_okrs({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_created_okrs(self):
        create({"title": "Goal A"})
        create({"title": "Goal B"})
        result = list_okrs({})
        assert result["count"] == 2

    def test_filter_by_status(self):
        create({"title": "Goal A"})
        result = list_okrs({"status": "active"})
        assert result["count"] == 1
        result2 = list_okrs({"status": "completed"})
        assert result2["count"] == 0


class TestUpdate:
    def test_missing_file(self):
        result = update({})
        assert result["success"] is False

    def test_file_not_found(self):
        result = update({"file": "nonexistent.md"})
        assert result["success"] is False

    def test_update_status(self):
        create({"title": "Goal"})
        okrs = list_okrs({})
        filename = okrs["okrs"][0]["file"]
        result = update({"file": filename, "status": "completed"})
        assert result["success"] is True
        okrs2 = list_okrs({"status": "completed"})
        assert okrs2["count"] == 1


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True
        assert "okrs" in result

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
