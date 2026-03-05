"""Tests for meeting-automation skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("meeting_automation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_meetings_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "start_time and end_time are required"}) as mock:
        result = _mod.list_meetings({})
        mock.assert_called_once_with("meeting", "list_meetings", {})

    assert result["success"] is False


def test_list_rooms_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "rooms": [{"room_id": "r1"}], "has_more": False}):
        result = _mod.list_rooms({})

    assert result["success"] is True
    assert len(result["rooms"]) == 1
