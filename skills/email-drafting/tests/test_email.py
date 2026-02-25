"""Tests for email-drafting skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("email_drafting_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

collect = _mod.collect
run = _mod.run


class TestCollect:
    def test_missing_purpose(self):
        result = collect({})
        assert result["success"] is False
        assert "purpose" in result["error"]

    def test_basic_collect(self):
        result = collect({"purpose": "Request budget approval"})
        assert result["success"] is True
        assert result["elements"]["purpose"] == "Request budget approval"
        assert result["elements"]["tone"] == "professional"
        assert result["elements"]["language"] == "zh"
        assert "draft_prompt" in result

    def test_all_elements(self):
        result = collect({
            "purpose": "Follow up",
            "recipient": "alice@example.com",
            "tone": "casual",
            "language": "en",
            "context": "After last meeting",
            "key_points": ["point1", "point2"],
            "cta": "Reply by Friday",
            "is_reply": True,
            "thread": "Re: Budget Q1",
        })
        e = result["elements"]
        assert e["recipient"] == "alice@example.com"
        assert e["tone"] == "casual"
        assert e["language"] == "en"
        assert e["key_points"] == ["point1", "point2"]
        assert e["is_reply"] is True
        assert e["thread"] == "Re: Budget Q1"

    def test_draft_prompt_structure(self):
        result = collect({"purpose": "test"})
        prompt = result["draft_prompt"]
        assert "主题行" in prompt
        assert "CTA" in prompt
        assert "tone" in prompt


class TestRun:
    def test_default_action_is_collect(self):
        result = run({})
        assert result["success"] is False  # missing purpose

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
