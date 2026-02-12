"""Tests for moltbook-posting skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("moltbook_posting_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

post = _mod.post
feed = _mod.feed
search = _mod.search
run = _mod.run


def _mock_api_response(data: dict):
    mock_resp = MagicMock()
    mock_resp.read.return_value = json.dumps(data).encode()
    mock_resp.__enter__ = lambda s: s
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


class TestPost:
    def test_missing_content(self):
        result = post({})
        assert result["success"] is False
        assert "title" in result["error"]

    def test_no_api_key(self, monkeypatch):
        monkeypatch.setattr(_mod, "_API_KEY", "")
        result = post({"title": "hello", "content": "world"})
        assert result["success"] is False
        assert "API_KEY" in result["error"]

    def test_success(self, monkeypatch):
        monkeypatch.setattr(_mod, "_API_KEY", "test-key")
        resp = _mock_api_response({"data": {"id": "123", "content": "hello"}})
        with patch("urllib.request.urlopen", return_value=resp):
            result = post({"title": "hello", "content": "hello", "tags": ["test"]})
            assert result["success"] is True
            assert result["post"]["id"] == "123"


class TestFeed:
    def test_success(self, monkeypatch):
        monkeypatch.setattr(_mod, "_API_KEY", "test-key")
        resp = _mock_api_response({"data": [{"id": "1"}, {"id": "2"}]})
        with patch("urllib.request.urlopen", return_value=resp):
            result = feed({})
            assert result["success"] is True
            assert result["count"] == 2


class TestSearch:
    def test_missing_query(self):
        result = search({})
        assert result["success"] is False

    def test_success(self, monkeypatch):
        monkeypatch.setattr(_mod, "_API_KEY", "test-key")
        resp = _mock_api_response({"data": [{"title": "Result 1"}]})
        with patch("urllib.request.urlopen", return_value=resp):
            result = search({"query": "test"})
            assert result["success"] is True
            assert len(result["results"]) == 1


class TestRun:
    def test_default_action_is_feed(self, monkeypatch):
        monkeypatch.setattr(_mod, "_API_KEY", "test-key")
        resp = _mock_api_response({"data": []})
        with patch("urllib.request.urlopen", return_value=resp):
            result = run({})
            assert result["success"] is True
            assert "posts" in result

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
