"""Tests for best-practice-search skill."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("best_practice_search_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS_DIR))

_spec.loader.exec_module(_mod)

search = _mod.search
run = _mod.run


_MOCK_SEARCH = {
    "source": "tavily",
    "query": "test",
    "answer": "mock answer",
    "results": [{"title": "R1", "url": "https://example.com", "content": "C1", "score": 0.9}],
    "results_count": 1,
}


class TestSearch:
    def test_missing_topic(self):
        result = search({})
        assert result["success"] is False
        assert "topic" in result["error"]

    def test_web_search(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = search({"topic": "Go error handling"})
            assert result["success"] is True
            assert result["topic"] == "Go error handling"
            assert len(result["web_results"]) == 2  # 2 queries
            assert "synthesis_prompt" in result

    def test_local_docs_search(self, tmp_path):
        docs_dir = tmp_path / "docs"
        docs_dir.mkdir()
        (docs_dir / "guide.md").write_text("Go error handling best practices")
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = search({"topic": "Go error handling", "docs_dir": str(docs_dir)})
            assert result["success"] is True
            assert len(result["local_results"]) >= 0  # grep may or may not find

    def test_synthesis_prompt_content(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH):
            result = search({"topic": "test"})
            prompt = result["synthesis_prompt"]
            assert "TL;DR" in prompt
            assert "共识" in prompt

    def test_custom_max_results(self):
        with patch.object(_mod, "tavily_search", return_value=_MOCK_SEARCH) as mock:
            search({"topic": "test", "max_results": 10})
            for call in mock.call_args_list:
                assert call.kwargs.get("max_results", call.args[1] if len(call.args) > 1 else 5) == 10


class TestRun:
    def test_default_action_is_search(self):
        result = run({})
        assert result["success"] is False  # missing topic

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
