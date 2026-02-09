"""Tests for artifact-management skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("artifact_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create = _mod.create
list_artifacts = _mod.list_artifacts
read = _mod.read
delete = _mod.delete
run = _mod.run


@pytest.fixture(autouse=True)
def _artifacts_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_ARTIFACTS_DIR", tmp_path)


class TestCreate:
    def test_missing_name(self):
        result = create({})
        assert result["success"] is False

    def test_creates_file(self, tmp_path):
        result = create({"name": "report.md", "content": "# Report"})
        assert result["success"] is True
        assert (tmp_path / "report.md").exists()
        assert (tmp_path / "report.md").read_text() == "# Report"

    def test_nested_path(self, tmp_path):
        result = create({"name": "sub/report.md", "content": "ok"})
        assert result["success"] is True
        assert (tmp_path / "sub" / "report.md").exists()


class TestListArtifacts:
    def test_empty(self):
        result = list_artifacts({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_created(self, tmp_path):
        create({"name": "a.md", "content": "A"})
        create({"name": "b.md", "content": "B"})
        result = list_artifacts({})
        assert result["count"] == 2


class TestRead:
    def test_missing_name(self):
        result = read({})
        assert result["success"] is False

    def test_not_found(self):
        result = read({"name": "nonexistent"})
        assert result["success"] is False

    def test_reads_content(self):
        create({"name": "doc.md", "content": "Hello"})
        result = read({"name": "doc.md"})
        assert result["success"] is True
        assert result["content"] == "Hello"


class TestDelete:
    def test_missing_name(self):
        result = delete({})
        assert result["success"] is False

    def test_not_found(self):
        result = delete({"name": "nonexistent"})
        assert result["success"] is False

    def test_deletes_file(self, tmp_path):
        create({"name": "temp.md", "content": "tmp"})
        assert (tmp_path / "temp.md").exists()
        result = delete({"name": "temp.md"})
        assert result["success"] is True
        assert not (tmp_path / "temp.md").exists()


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
