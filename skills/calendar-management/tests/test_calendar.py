"""Tests for calendar-management skill."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("calendar_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create_event = _mod.create_event
query_events = _mod.query_events
delete_event = _mod.delete_event
run = _mod.run


class TestCreateEvent:
    def test_missing_title(self):
        result = create_event({"start": "2026-02-10 14:00"})
        assert result["success"] is False
        assert "title" in result["error"]

    def test_missing_start(self):
        result = create_event({"title": "周会"})
        assert result["success"] is False
        assert "start" in result["error"]

    def test_no_token(self):
        with patch.dict("os.environ", {}, clear=True):
            result = create_event({"title": "周会", "start": "2026-02-10 14:00"})
            assert result["success"] is False
            assert "LARK_TENANT_TOKEN" in result.get("error", "")

    def test_successful_creation(self):
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({
            "data": {"event_id": "evt_123", "summary": "周会"}
        }).encode()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch.dict("os.environ", {"LARK_TENANT_TOKEN": "test-token"}):
            with patch("urllib.request.urlopen", return_value=mock_resp):
                result = create_event({"title": "周会", "start": "2026-02-10 14:00"})
                assert result["success"] is True
                assert "周会" in result["message"]


class TestQueryEvents:
    def test_missing_start(self):
        result = query_events({})
        assert result["success"] is False

    def test_successful_query(self):
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({
            "data": {"items": [{"event_id": "evt_1", "summary": "Meeting"}]}
        }).encode()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch.dict("os.environ", {"LARK_TENANT_TOKEN": "test-token"}):
            with patch("urllib.request.urlopen", return_value=mock_resp):
                result = query_events({"start": "2026-02-10"})
                assert result["success"] is True
                assert result["count"] == 1


class TestDeleteEvent:
    def test_missing_event_id(self):
        result = delete_event({})
        assert result["success"] is False

    def test_successful_delete(self):
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({}).encode()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch.dict("os.environ", {"LARK_TENANT_TOKEN": "test-token"}):
            with patch("urllib.request.urlopen", return_value=mock_resp):
                result = delete_event({"event_id": "evt_123"})
                assert result["success"] is True


class TestRunDispatch:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False

    def test_default_action_is_query(self):
        with patch.object(_mod, "query_events", return_value={"success": True}) as mock:
            run({})
            mock.assert_called_once()
