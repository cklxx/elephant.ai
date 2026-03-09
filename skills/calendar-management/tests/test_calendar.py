"""Tests for calendar-management skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("calendar_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create_event = _mod.create_event
query_events = _mod.query_events
delete_event = _mod.delete_event
run = _mod.run


class TestCreateEvent:
    def test_delegates_to_feishu_cli(self):
        with patch.object(_mod, "feishu_tool", return_value={"success": True, "event": {"id": "evt1"}}) as mock:
            result = create_event({"title": "周会", "start": "2026-02-10 14:00"})
            mock.assert_called_once_with("calendar", "create", {"title": "周会", "start": "2026-02-10 14:00"})
            assert result["success"] is True


class TestQueryEvents:
    def test_delegates_to_feishu_cli(self):
        with patch.object(_mod, "feishu_tool", return_value={"success": True, "count": 1}) as mock:
            result = query_events({"start": "2026-02-10"})
            mock.assert_called_once_with("calendar", "query", {"start": "2026-02-10"})
            assert result["count"] == 1


class TestDeleteEvent:
    def test_delegates_to_feishu_cli(self):
        with patch.object(_mod, "feishu_tool", return_value={"success": True}) as mock:
            result = delete_event({"event_id": "evt_123"})
            mock.assert_called_once_with("calendar", "delete", {"event_id": "evt_123"})
            assert result["success"] is True


class TestRunDispatch:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False

    def test_default_action_is_query(self):
        with patch.object(_mod, "query_events", return_value={"success": True}) as mock:
            run({})
            mock.assert_called_once()
