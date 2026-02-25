"""Tests for meeting-notes skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("meeting_notes_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

collect = _mod.collect
run = _mod.run


class TestCollect:
    def test_missing_notes(self):
        result = collect({})
        assert result["success"] is False
        assert "notes or file" in result["error"]

    def test_with_raw_notes(self):
        result = collect({"notes": "Meeting about Q1 goals."})
        assert result["success"] is True
        assert result["raw_notes"] == "Meeting about Q1 goals."
        assert result["word_count"] > 0
        assert "format_prompt" in result

    def test_with_file(self, tmp_path):
        f = tmp_path / "notes.txt"
        f.write_text("Notes from file")
        result = collect({"file": str(f)})
        assert result["success"] is True
        assert result["raw_notes"] == "Notes from file"

    def test_file_overrides_notes(self, tmp_path):
        f = tmp_path / "notes.txt"
        f.write_text("from file")
        result = collect({"notes": "from args", "file": str(f)})
        assert result["raw_notes"] == "from file"

    def test_meeting_info(self):
        result = collect({
            "notes": "test",
            "title": "Sprint Review",
            "attendees": ["Alice", "Bob"],
            "duration": "30min",
        })
        assert result["meeting_info"]["title"] == "Sprint Review"
        assert result["meeting_info"]["attendees"] == ["Alice", "Bob"]
        assert result["meeting_info"]["duration"] == "30min"
        assert result["meeting_info"]["date"]  # auto-filled

    def test_format_prompt_structure(self):
        result = collect({"notes": "test"})
        prompt = result["format_prompt"]
        assert "关键决策" in prompt
        assert "行动项" in prompt
        assert "讨论要点" in prompt


class TestRun:
    def test_default_action_is_collect(self):
        result = run({})
        assert result["success"] is False  # missing notes

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
