"""Tests for research-briefing skill."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("research_briefing_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS_DIR))

_spec.loader.exec_module(_mod)

collect = _mod.collect
run = _mod.run


_MOCK_SEARCH = {
    "source": "tavily",
    "query": "test",
    "answer": "mock answer",
    "results": [{"title": "R1", "url": "https://example.com", "content": "C1", "score": 0.9}],
    "results_count": 1,
}


class TestCollect:
    def test_missing_topic(self):
        result = collect({})
        assert result["success"] is False
        assert "topic" in result["error"]

    def test_basic_collect(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({"topic": "AI safety"})
            assert result["success"] is True
            assert result["topic"] == "AI safety"
            assert len(result["search_results"]) == 2  # default 2 questions
            assert "briefing_prompt" in result

    def test_custom_questions(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({
                "topic": "AI safety",
                "questions": ["Q1?", "Q2?", "Q3?"],
            })
            assert len(result["search_results"]) == 3

    def test_max_5_questions(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({
                "topic": "test",
                "questions": ["Q1", "Q2", "Q3", "Q4", "Q5", "Q6", "Q7"],
            })
            assert len(result["search_results"]) == 5

    def test_audience(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({"topic": "test", "audience": "executive"})
            assert result["audience"] == "executive"

    def test_briefing_prompt_structure(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({"topic": "test"})
            prompt = result["briefing_prompt"]
            assert "摘要" in prompt
            assert "关键问题" in prompt
            assert "参考来源" in prompt

    def test_search_results_contain_question(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = collect({"topic": "test", "questions": ["What is test?"]})
            assert result["search_results"][0]["question"] == "What is test?"


class TestRun:
    def test_default_action_is_collect(self):
        result = run({})
        assert result["success"] is False  # missing topic

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
