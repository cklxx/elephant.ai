"""Tests for drive-file skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("drive_file_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_create_folder_requires_name():
    result = _mod.create_folder({})
    assert result["success"] is False
    assert "name" in result["error"]


def test_list_files_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"files": [{"token": "f1"}, {"token": "f2"}]}},
    ):
        result = _mod.list_files({"folder_token": "root"})

    assert result["success"] is True
    assert result["count"] == 2
