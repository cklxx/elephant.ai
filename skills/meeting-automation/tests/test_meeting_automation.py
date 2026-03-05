"""Tests for meeting-automation skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("meeting_automation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_meetings_requires_time_window():
    result = _mod.list_meetings({})
    assert result["success"] is False
    assert "start_time" in result["error"]


def test_list_rooms_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"rooms": [{"room_id": "r1"}], "has_more": False}},
    ):
        result = _mod.list_rooms({})

    assert result["success"] is True
    assert len(result["rooms"]) == 1
