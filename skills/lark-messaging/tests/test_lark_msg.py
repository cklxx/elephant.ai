"""Tests for lark-messaging skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("lark_messaging_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

send = _mod.send
run = _mod.run


def _mock_response(data: dict):
    mock_resp = MagicMock()
    mock_resp.read.return_value = json.dumps(data).encode()
    mock_resp.__enter__ = lambda s: s
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


class TestSend:
    def test_missing_chat_id(self):
        result = send({"content": "hello"})
        assert result["success"] is False

    def test_missing_content(self):
        result = send({"chat_id": "oc_xxx"})
        assert result["success"] is False

    def test_no_credentials(self, monkeypatch):
        monkeypatch.delenv("LARK_APP_ID", raising=False)
        monkeypatch.delenv("LARK_APP_SECRET", raising=False)
        result = send({"chat_id": "oc_xxx", "content": "hello"})
        assert result["success"] is False

    def test_success(self, monkeypatch):
        monkeypatch.setenv("LARK_APP_ID", "test-id")
        monkeypatch.setenv("LARK_APP_SECRET", "test-secret")

        # Mock token request + send request
        token_resp = _mock_response({"tenant_access_token": "test-token"})
        send_resp = _mock_response({"data": {"message_id": "msg_123"}})

        call_count = {"n": 0}
        def mock_urlopen(req, **kwargs):
            call_count["n"] += 1
            if "auth" in req.full_url:
                return token_resp
            return send_resp

        with patch("urllib.request.urlopen", side_effect=mock_urlopen):
            result = send({"chat_id": "oc_xxx", "content": "hello"})
            assert result["success"] is True
            assert result["message_id"] == "msg_123"


class TestRun:
    def test_default_action_is_send(self):
        result = run({})
        assert result["success"] is False  # missing params

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
