"""Tests for music-discovery skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("music_discovery_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

search = _mod.search
run = _mod.run


def _mock_itunes_response(results: list):
    mock_resp = MagicMock()
    mock_resp.read.return_value = json.dumps({"results": results}).encode()
    mock_resp.__enter__ = lambda s: s
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


class TestSearch:
    def test_missing_query(self):
        result = search({})
        assert result["success"] is False

    def test_success(self):
        items = [{
            "trackName": "晴天",
            "artistName": "周杰伦",
            "collectionName": "叶惠美",
            "previewUrl": "https://example.com/preview.m4a",
            "artworkUrl100": "https://example.com/art.jpg",
            "primaryGenreName": "Pop",
            "trackTimeMillis": 269000,
        }]
        resp = _mock_itunes_response(items)
        with patch("urllib.request.urlopen", return_value=resp):
            result = search({"query": "周杰伦 晴天"})
            assert result["success"] is True
            assert result["count"] == 1
            assert result["results"][0]["track"] == "晴天"
            assert result["results"][0]["artist"] == "周杰伦"

    def test_api_error(self):
        import urllib.error
        with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timeout")):
            result = search({"query": "test"})
            assert result["success"] is False

    def test_empty_results(self):
        resp = _mock_itunes_response([])
        with patch("urllib.request.urlopen", return_value=resp):
            result = search({"query": "nonexistent song xyz"})
            assert result["success"] is True
            assert result["count"] == 0


class TestRun:
    def test_default_action_is_search(self):
        result = run({})
        assert result["success"] is False  # missing query

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
