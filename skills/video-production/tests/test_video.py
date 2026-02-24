"""Tests for video-production skill."""

from __future__ import annotations

import importlib.util
import io
import json
from pathlib import Path
from unittest.mock import MagicMock, patch
import urllib.error

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


def _mock_binary_response(data: bytes):
    mock_resp = MagicMock()
    mock_resp.read.return_value = data
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

    def test_no_endpoint_uses_default_model(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "")

        captured = {"body": None}

        def _mock_urlopen(req, timeout=0):
            if isinstance(req, _mod.urllib.request.Request):
                captured["body"] = json.loads(req.data.decode())
            raise _mod.urllib.error.URLError("stop after capture")

        with patch("urllib.request.urlopen", side_effect=_mock_urlopen):
            result = generate({"prompt": "test"})

        assert result["success"] is False
        assert captured["body"]["model"] == "doubao-seedance-1-0-pro-fast-251015"

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
        resp_api = _mock_api_response({"data": [{"url": "https://example.com/v.mp4"}]})
        resp_file = _mock_binary_response(b"fake-mp4-data")

        with patch("urllib.request.urlopen", side_effect=[resp_api, resp_file]) as mock_open:
            result = generate({"prompt": "a cat dancing", "output": output})
            assert result["success"] is True
            assert result["path"] == output
            assert mock_open.call_count == 2

    def test_api_error(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "ep-123")
        with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timeout")):
            result = generate({"prompt": "test"})
            assert result["success"] is False

    def test_404_retries_with_fallback_endpoint(self, monkeypatch, tmp_path):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "bad-endpoint")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_FALLBACKS", "good-endpoint")
        monkeypatch.setattr(_mod, "_discover_seedance_endpoints", lambda _k: [])

        output = str(tmp_path / "retry.mp4")

        def _mock_urlopen(req, timeout=0):
            if isinstance(req, _mod.urllib.request.Request):
                body = json.loads(req.data.decode()) if req.data else {}
                model = body.get("model")
                if model == "bad-endpoint":
                    raise urllib.error.HTTPError(
                        req.full_url,
                        404,
                        "Not Found",
                        hdrs=None,
                        fp=io.BytesIO(b'{"error":"model not found"}'),
                    )
                return _mock_api_response({"data": [{"url": "https://example.com/v.mp4"}]})
            return _mock_binary_response(b"fake-mp4-data")

        with patch("urllib.request.urlopen", side_effect=_mock_urlopen):
            result = generate({"prompt": "test fallback", "output": output})

        assert result["success"] is True
        assert result["endpoint"] == "good-endpoint"

    def test_all_404_returns_clear_error(self, monkeypatch):
        monkeypatch.setenv("ARK_API_KEY", "key-123")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_ID", "bad-endpoint")
        monkeypatch.setenv("SEEDANCE_ENDPOINT_FALLBACKS", "bad-endpoint-2")
        monkeypatch.setattr(_mod, "_discover_seedance_endpoints", lambda _k: [])

        def _mock_404(req, timeout=0):
            if isinstance(req, _mod.urllib.request.Request):
                raise urllib.error.HTTPError(
                    req.full_url,
                    404,
                    "Not Found",
                    hdrs=None,
                    fp=io.BytesIO(b'{"error":"model not found"}'),
                )
            return _mock_binary_response(b"")

        with patch("urllib.request.urlopen", side_effect=_mock_404):
            result = generate({"prompt": "test 404"})

        assert result["success"] is False
        assert "all Seedance endpoints failed with 404" in result["error"]
        assert "attempted_endpoints" in result


class TestRun:
    def test_default_action_is_generate(self):
        result = run({"prompt": ""})
        assert result["success"] is False

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
