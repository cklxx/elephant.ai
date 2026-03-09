"""Tests for drive-file skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("drive_file_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_create_folder_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "name is required"}) as mock:
        result = _mod.create_folder({})
        mock.assert_called_once_with("drive", "create_folder", {})

    assert result["success"] is False


def test_list_files_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "files": [{"token": "f1"}, {"token": "f2"}], "count": 2}):
        result = _mod.list_files({"folder_token": "root"})

    assert result["success"] is True
    assert result["count"] == 2
