"""Tests for bitable-data skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("bitable_data_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_tables_requires_app_token():
    result = _mod.list_tables({"auto_create_app": False})
    assert result["success"] is False
    assert "app_token" in result["error"]


def test_list_tables_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"items": [{"table_id": "tbl_1"}]}},
    ):
        result = _mod.list_tables({"app_token": "app_x"})

    assert result["success"] is True
    assert result["count"] == 1


def test_list_tables_auto_create_when_missing_token():
    with patch.object(
        _mod,
        "_lark_api",
        side_effect=[
            {"data": {"app_token": "app_created"}},
            {"data": {"items": [{"table_id": "tbl_1"}]}},
        ],
    ):
        result = _mod.list_tables({})

    assert result["success"] is True
    assert result["count"] == 1
    assert result["app_token"] == "app_created"
