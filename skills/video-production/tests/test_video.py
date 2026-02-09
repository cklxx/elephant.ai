"""Tests for video-production skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("video_production_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

generate = _mod.generate
run = _mod.run


def _mock_api_response(data: dict):
    mock_resp = MagicMock()
    mock_resp.read.return_value = json.dumps(data).encode()
    mock_resp.__enter__ = lambda s: s
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


class TestGenerate:
    def test_missing_prompt(self):
        result = generate({})
        assert result["success"] is False
        assert "prompt" in result["error"]

    def test_no_api_key(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "ep-123")
        # Reload to pick up env changes
        result = generate({"prompt": "test"})
        assert result["success"] is False
        assert "ARK_API_KEY" in result["error"]

    def test_no_endpoint(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "")
        result = generate({"prompt": "test"})
        assert result["success"] is False
        assert "SEEDANCE_ENDPOINT_ID" in result["error"]

    def test_api_returns_no_videos(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "ep-123")
        resp = _mock_api_response({"data": []})
        with patch("urllib.request.urlopen", return_value=resp):
            result = generate({"prompt": "test video"})
            assert result["success"] is False
            assert "no video" in result["error"]

    def test_success(self, monkeypatch, tmp_path):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "ep-123")
        output = str(tmp_path / "test.mp4")
        resp = _mock_api_response({"data": [{"url": "https://example.com/v.mp4"}]})

        with patch("urllib.request.urlopen", return_value=resp):
            with patch("urllib.request.urlretrieve") as mock_dl:
                result = generate({"prompt": "a cat dancing", "output": output})
                assert result["success"] is True
                assert result["path"] == output
                mock_dl.assert_called_once()

    def test_api_error(self, monkeypatch):
        import urllib.error
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "ep-123")
        with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timeout")):
            result = generate({"prompt": "test"})
            assert result["success"] is False


class TestRun:
    def test_default_action_is_generate(self):
        result = run({"prompt": ""})
        assert result["success"] is False

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
