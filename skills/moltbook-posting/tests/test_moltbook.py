"""Tests for moltbook-posting skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch
import urllib.error

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


def test_search_fallback_to_alternate_domain(monkeypatch):
    monkeypatch.setattr(_mod, "_API_KEY", "test-key")
    monkeypatch.setattr(_mod, "_BASE", "https://www.moltbook.com/api/v1")
    monkeypatch.setattr(_mod, "_LAST_GOOD_BASE", "")

    resp = _mock_api_response({"data": [{"title": "Result 1"}]})

    def _mock_urlopen(req, timeout=0):
        _ = timeout
        url = req.full_url if hasattr(req, "full_url") else str(req)
        if "www.moltbook.com" in url:
            raise urllib.error.URLError("timed out")
        return resp

    with patch("urllib.request.urlopen", side_effect=_mock_urlopen):
        result = search({"query": "test"})

    assert result["success"] is True
    assert len(result["results"]) == 1


def test_search_reports_all_domain_failures(monkeypatch):
    monkeypatch.setattr(_mod, "_API_KEY", "test-key")
    monkeypatch.setattr(_mod, "_BASE", "https://www.moltbook.com/api/v1")
    monkeypatch.setattr(_mod, "_LAST_GOOD_BASE", "")

    with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timed out")):
        result = search({"query": "test"})

    assert result["success"] is False
    assert "all Moltbook endpoints failed" in result["error"]
    assert isinstance(result.get("details"), list)
