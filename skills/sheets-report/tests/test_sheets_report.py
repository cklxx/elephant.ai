"""Tests for sheets-report skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("sheets_report_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_get_spreadsheet_requires_token():
    result = _mod.get_spreadsheet({})
    assert result["success"] is False
    assert "spreadsheet_token" in result["error"]


def test_create_spreadsheet_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"spreadsheet": {"token": "s1"}}},
    ):
        result = _mod.create_spreadsheet({"title": "demo"})

    assert result["success"] is True
    assert result["spreadsheet"]["token"] == "s1"
