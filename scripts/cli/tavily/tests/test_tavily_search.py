"""Tests for tavily_search CLI."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent.parent))

from cli.tavily.tavily_search import (
    _DDGParser,
    _duckduckgo_fallback,
    tavily_search,
)


class TestTavilySearch:
    """Tests for the main tavily_search function."""

    def test_no_api_key_falls_back_to_ddg(self):
        """When no API key, should use DuckDuckGo fallback."""
        with patch.dict("os.environ", {}, clear=True):
            with patch(
                "cli.tavily.tavily_search._duckduckgo_fallback",
                return_value={"source": "duckduckgo", "query": "test", "results": [], "answer": "", "results_count": 0},
            ) as mock_ddg:
                result = tavily_search("test query")
                mock_ddg.assert_called_once_with("test query", 5)
                assert result["source"] == "duckduckgo"

    def test_with_api_key_calls_tavily(self):
        """When API key exists, should call Tavily API."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "query": "test",
            "answer": "Test answer",
            "results": [
                {"title": "Result 1", "url": "https://example.com", "content": "Content 1", "score": 0.9},
            ],
        }).encode()
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)

        with patch("urllib.request.urlopen", return_value=mock_response):
            result = tavily_search("test", api_key="test-key")
            assert result["source"] == "tavily"
            assert result["answer"] == "Test answer"
            assert len(result["results"]) == 1
            assert result["results"][0]["title"] == "Result 1"

    def test_api_failure_falls_back(self):
        """When Tavily API fails, should fallback to DuckDuckGo."""
        import urllib.error

        with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timeout")):
            with patch(
                "cli.tavily.tavily_search._duckduckgo_fallback",
                return_value={"source": "duckduckgo", "query": "test", "results": [], "answer": "", "results_count": 0},
            ) as mock_ddg:
                result = tavily_search("test", api_key="test-key")
                mock_ddg.assert_called_once()

    def test_max_results_parameter(self):
        """Should pass max_results to the API."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "query": "test", "answer": "", "results": [],
        }).encode()
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)

        with patch("urllib.request.urlopen", return_value=mock_response) as mock_urlopen:
            tavily_search("test", api_key="key", max_results=10)
            call_args = mock_urlopen.call_args
            req = call_args[0][0]
            body = json.loads(req.data)
            assert body["max_results"] == 10


class TestDDGParser:
    """Tests for the DuckDuckGo HTML parser."""

    def test_parses_results(self):
        html = """
        <div class="result__body">
            <a class="result__a" href="https://example.com">Example Title</a>
            <a class="result__snippet">Example snippet text</a>
        </div>
        <div class="result__body">
            <a class="result__a" href="https://other.com">Other Title</a>
            <a class="result__snippet">Other snippet</a>
        </div>
        """
        parser = _DDGParser()
        parser.feed(html)
        assert len(parser.results) == 2
        assert parser.results[0]["title"] == "Example Title"
        assert parser.results[0]["url"] == "https://example.com"
        assert parser.results[1]["title"] == "Other Title"

    def test_empty_html(self):
        parser = _DDGParser()
        parser.feed("<html><body></body></html>")
        assert len(parser.results) == 0
