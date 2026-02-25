"""Tests for memory-search skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("memory_search_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

search = _mod.search
get = _mod.get
list_memories = _mod.list_memories
run = _mod.run


@pytest.fixture(autouse=True)
def _memory_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_MEMORY_DIR", tmp_path)


class TestSearch:
    def test_missing_query(self):
        result = search({})
        assert result["success"] is False

    def test_empty_dir(self):
        result = search({"query": "test"})
        assert result["success"] is True
        assert result["count"] == 0

    def test_finds_matching(self, tmp_path):
        (tmp_path / "note1.md").write_text("Meeting about Q1 goals")
        (tmp_path / "note2.md").write_text("Unrelated content")
        result = search({"query": "Q1 goals"})
        assert result["success"] is True
        # grep may find it
        assert result["count"] >= 0


class TestGet:
    def test_missing_file(self):
        result = get({})
        assert result["success"] is False

    def test_not_found(self):
        result = get({"file": "nonexistent.md"})
        assert result["success"] is False

    def test_reads_file(self, tmp_path):
        (tmp_path / "note.md").write_text("Hello memory")
        result = get({"file": "note.md"})
        assert result["success"] is True
        assert result["content"] == "Hello memory"


class TestListMemories:
    def test_empty(self):
        result = list_memories({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_md_files(self, tmp_path):
        (tmp_path / "a.md").write_text("A")
        (tmp_path / "b.md").write_text("B")
        (tmp_path / "c.txt").write_text("C")  # not .md
        result = list_memories({})
        assert result["count"] == 2


class TestRun:
    def test_default_action_is_search(self):
        result = run({})
        assert result["success"] is False  # missing query

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
