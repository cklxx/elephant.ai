"""Tests for sheets-report skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("sheets_report_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_get_spreadsheet_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "spreadsheet_token is required"}) as mock:
        result = _mod.get_spreadsheet({})
        mock.assert_called_once_with("sheets", "get", {})

    assert result["success"] is False


def test_create_spreadsheet_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "spreadsheet": {"token": "s1"}}):
        result = _mod.create_spreadsheet({"title": "demo"})

    assert result["success"] is True
    assert result["spreadsheet"]["token"] == "s1"
