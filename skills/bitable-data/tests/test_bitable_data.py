"""Tests for bitable-data skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("bitable_data_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_tables_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "app_token is required"}) as mock:
        result = _mod.list_tables({"auto_create_app": False})
        mock.assert_called_once_with("bitable", "list_tables", {"auto_create_app": False})

    assert result["success"] is False


def test_list_tables_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "tables": [{"table_id": "tbl_1"}], "count": 1}):
        result = _mod.list_tables({"app_token": "app_x"})

    assert result["success"] is True
    assert result["count"] == 1
