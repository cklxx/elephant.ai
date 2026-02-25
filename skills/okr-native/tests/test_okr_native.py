"""Tests for okr-native skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("okr_native_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_user_okrs_requires_user_id():
    result = _mod.list_user_okrs({})
    assert result["success"] is False
    assert "user_id" in result["error"]


def test_list_periods_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"items": [{"period_id": "p1"}], "has_more": False}},
    ):
        result = _mod.list_periods({})

    assert result["success"] is True
    assert len(result["periods"]) == 1
