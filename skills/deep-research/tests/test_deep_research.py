"""Tests for deep-research skill."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# Load run.py by absolute path to avoid module name collisions
_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("deep_research_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)

_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS_DIR))

_spec.loader.exec_module(_mod)

_fetch_page = _mod._fetch_page
_generate_queries = _mod._generate_queries
run = _mod.run


class TestGenerateQueries:
    def test_returns_3_queries(self):
        queries = _generate_queries("AI safety")
        assert len(queries) == 3
        assert "AI safety" in queries[0]

    def test_includes_best_practices(self):
        queries = _generate_queries("React hooks")
        assert any("best practices" in q for q in queries)


class TestRun:
    def test_missing_topic_returns_error(self):
        result = run({})
        assert result["success"] is False
        assert "topic" in result["error"]

    def test_basic_research(self):
        mock_search_result = {
            "source": "tavily",
            "query": "test topic",
            "answer": "Test answer",
            "results": [{"title": "R1", "url": "https://example.com", "content": "C1", "score": 0.9}],
            "results_count": 1,
        }
        with patch.object(_mod, "tavily_search", return_value=mock_search_result):
            result = run({"topic": "test topic"})
            assert result["success"] is True
            assert result["topic"] == "test topic"
            assert len(result["searches"]) == 3
            assert result["total_sources"] >= 1

    def test_custom_queries(self):
        mock_search_result = {
            "source": "tavily",
            "query": "q1",
            "answer": "",
            "results": [],
            "results_count": 0,
        }
        with patch.object(_mod, "tavily_search", return_value=mock_search_result):
            result = run({"topic": "test", "queries": ["q1", "q2"]})
            assert len(result["searches"]) == 2

    def test_fetch_urls(self):
        mock_search = {"source": "tavily", "query": "t", "answer": "", "results": [], "results_count": 0}
        mock_page = {"url": "https://example.com", "title": "Test", "content": "Hello"}

        with patch.object(_mod, "tavily_search", return_value=mock_search):
            with patch.object(_mod, "_fetch_page", return_value=mock_page):
                result = run({
                    "topic": "test",
                    "queries": ["q1"],
                    "fetch_urls": ["https://example.com"],
                })
                assert len(result["fetched_pages"]) == 1

    def test_summary_prompt_included(self):
        mock_search = {"source": "tavily", "query": "t", "answer": "", "results": [], "results_count": 0}
        with patch.object(_mod, "tavily_search", return_value=mock_search):
            result = run({"topic": "test", "queries": ["q1"]})
            assert "summary_prompt" in result


class TestFetchPage:
    def test_fetch_error_returns_error_dict(self):
        import urllib.error

        with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("fail")):
            result = _fetch_page("https://nonexistent.example.com")
            assert result["error"] == "fetch failed"

    def test_strips_html_tags(self):
        mock_resp = MagicMock()
        mock_resp.read.return_value = b"<html><title>Test Page</title><body><p>Hello world</p></body></html>"
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch("urllib.request.urlopen", return_value=mock_resp):
            result = _fetch_page("https://example.com")
            assert "Hello world" in result["content"]
            assert "<p>" not in result["content"]
            assert result["title"] == "Test Page"
